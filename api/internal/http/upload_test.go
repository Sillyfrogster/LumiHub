package http

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// uploadRequest builds the form a browser sends: the metadata first, then the
// file.
func uploadRequest(t *testing.T, metadata map[string]any, file []byte) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	writeMetadataPart(t, form, metadata)
	writeFilePart(t, form, file)
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/assets", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func writeMetadataPart(t *testing.T, form *multipart.Writer, metadata map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}
	if err := form.WriteField(metadataPart, string(encoded)); err != nil {
		t.Fatalf("write metadata part: %v", err)
	}
}

func writeFilePart(t *testing.T, form *multipart.Writer, file []byte) {
	t.Helper()
	part, err := form.CreateFormFile(filePart, "upload.bin")
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(file); err != nil {
		t.Fatalf("write file part: %v", err)
	}
}

func exampleMetadata(name string) map[string]any {
	return map[string]any{
		"kind":      "character",
		"filename":  name + ".bin",
		"name":      name,
		"discovery": "listed",
	}
}

func send(t *testing.T, r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestUploadOverTheCeilingIsRefused(t *testing.T) {
	r, session := newVerifiedTestRouterWith(t, 64, DefaultDeadlines())

	rec := send(t, r, authorized(
		uploadRequest(t, exampleMetadata("Huge"), bytes.Repeat([]byte("a"), 1024)), session,
	))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413. body: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadAtTheCeilingIsAccepted(t *testing.T) {
	req := uploadRequest(t, exampleMetadata("Exactly"), bytes.Repeat([]byte("a"), 1024))
	r, session := newVerifiedTestRouterWith(t, req.ContentLength, DefaultDeadlines())

	rec := send(t, r, authorized(req, session))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: a request of exactly the ceiling fits. body: %s",
			rec.Code, rec.Body.String())
	}
}

// countingBody reports how much of the request was read, so a refusal that
// waits for the whole upload can be told from one that does not.
type countingBody struct {
	reader io.Reader
	read   int
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.read += n
	return n, err
}

func TestUploadIsRefusedWithoutReadingTheBody(t *testing.T) {
	r, session := newVerifiedTestRouterWith(t, 64, DefaultDeadlines())

	req := uploadRequest(t, exampleMetadata("Declared"), bytes.Repeat([]byte("a"), 1024))
	req.AddCookie(session)
	body := &countingBody{reader: req.Body}
	req.Body = io.NopCloser(body)

	rec := send(t, r, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413. body: %s", rec.Code, rec.Body.String())
	}
	if body.read != 0 {
		t.Errorf("read %d bytes of a request that was always going to be refused", body.read)
	}
}

func TestUploadIsCutOffWhenItsLengthIsUnknown(t *testing.T) {
	r, session := newVerifiedTestRouterWith(t, 512, DefaultDeadlines())

	// A sender that does not say how long the request is gets no free pass.
	req := uploadRequest(t, exampleMetadata("Unstated"), bytes.Repeat([]byte("a"), 4096))
	req.ContentLength = -1
	req.AddCookie(session)

	rec := send(t, r, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413. body: %s", rec.Code, rec.Body.String())
	}
}

func formRequest(t *testing.T, write func(*multipart.Writer)) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	write(form)
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/assets", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func TestFormDataThatCannotBeReadIsRefused(t *testing.T) {
	cases := []struct {
		name    string
		request func(t *testing.T) *http.Request
		says    string
	}{
		{
			name: "not form data at all",
			request: func(t *testing.T) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/v1/assets",
					strings.NewReader(`{"kind":"character"}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			says: "form data",
		},
		{
			name: "the metadata part is missing",
			request: func(t *testing.T) *http.Request {
				return formRequest(t, func(form *multipart.Writer) {
					writeFilePart(t, form, []byte("bytes"))
				})
			},
			says: metadataPart,
		},
		{
			name: "the file part is missing",
			request: func(t *testing.T) *http.Request {
				return formRequest(t, func(form *multipart.Writer) {
					writeMetadataPart(t, form, exampleMetadata("Fileless"))
				})
			},
			says: filePart,
		},
		{
			name: "the metadata part is not JSON",
			request: func(t *testing.T) *http.Request {
				return formRequest(t, func(form *multipart.Writer) {
					if err := form.WriteField(metadataPart, "kind=character"); err != nil {
						t.Fatalf("write metadata part: %v", err)
					}
					writeFilePart(t, form, []byte("bytes"))
				})
			},
			says: "JSON",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, session := newVerifiedTestRouter(t)

			rec := send(t, r, authorized(c.request(t), session))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.says) {
				t.Errorf("refusal %s does not say what was wrong, expected it to mention %q",
					rec.Body.String(), c.says)
			}
		})
	}
}

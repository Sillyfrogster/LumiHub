package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// serve runs a handler on a real port, which is the only way to watch what the
// server does with a connection rather than with a request.
func serve(t *testing.T, handler http.Handler, timeouts Timeouts) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := NewServer(listener.Addr().String(), handler, timeouts)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	return "http://" + listener.Addr().String()
}

func TestAConnectionThatNeverFinishesItsHeadersIsCutOff(t *testing.T) {
	base := serve(t, http.NotFoundHandler(), Timeouts{
		ReadHeader: 200 * time.Millisecond,
		Idle:       time.Minute,
	})

	conn, err := net.Dial("tcp", base[len("http://"):])
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// No blank line after the header, so the request is never finished.
	if _, err := fmt.Fprint(conn, "GET /v1/assets HTTP/1.1\r\nHost: lumihub.test\r\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	if _, err := io.ReadAll(conn); err != nil {
		t.Fatalf("the connection was still open after five seconds: %v", err)
	}
}

func TestAnUploadArrivingSlowlyRunsToCompletion(t *testing.T) {
	r, session := newVerifiedTestRouterWith(t, 1<<20, Deadlines{
		JSON:     300 * time.Millisecond,
		Upload:   30 * time.Second,
		Download: 30 * time.Second,
	})
	base := serve(t, r, Timeouts{ReadHeader: 300 * time.Millisecond, Idle: time.Minute})

	built := uploadRequest(t, exampleMetadata("Trickle"), []byte("bytes that take their time"))
	form, err := io.ReadAll(built.Body)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}

	// Sent over about a second, which is far longer than a listing is allowed
	// and longer than the whole request may take to start.
	body := &slowReader{rest: form, chunk: len(form)/5 + 1, pause: 200 * time.Millisecond}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/assets", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", built.Header.Get("Content-Type"))
	req.AddCookie(session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		answer, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 202. body: %s", resp.StatusCode, answer)
	}
}

func TestADownloadReadSlowlyRunsToCompletion(t *testing.T) {
	r, session, assets := newVerifiedTestRouterWithService(t, 8<<20, Deadlines{
		JSON:     300 * time.Millisecond,
		Upload:   30 * time.Second,
		Download: 30 * time.Second,
	})
	base := serve(t, r, Timeouts{ReadHeader: 300 * time.Millisecond, Idle: time.Minute})

	file := bytes.Repeat([]byte("lumi"), 1<<20) // 4 MB
	id := uploadOver(t, base, file, session, assets)

	resp, err := http.Get(base + "/v1/assets/" + id + "/original")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	got, err := readInPieces(resp.Body, 128<<10, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("the download stopped after %d bytes of %d: %v", len(got), len(file), err)
	}
	if !bytes.Equal(got, file) {
		t.Fatalf("downloaded %d bytes, want %d", len(got), len(file))
	}
}

func uploadOver(
	t *testing.T,
	base string,
	file []byte,
	session *http.Cookie,
	assets interface {
		ProcessNextIngest(context.Context) (bool, error)
	},
) string {
	t.Helper()

	metadata := exampleMetadata("Chunky")
	metadata["filename"] = "chunky.lumitheme"
	built := uploadRequest(t, metadata, file)
	req, err := http.NewRequest(http.MethodPost, base+"/v1/assets", built.Body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", built.Header.Get("Content-Type"))
	req.AddCookie(session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		answer, _ := io.ReadAll(resp.Body)
		t.Fatalf("create failed: %d %s", resp.StatusCode, answer)
	}

	var accepted struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if processed, err := assets.ProcessNextIngest(context.Background()); err != nil || !processed {
		t.Fatalf("process ingest = %v, %v; want true, nil", processed, err)
	}
	poll, err := http.NewRequest(http.MethodGet, base+accepted.URL, nil)
	if err != nil {
		t.Fatalf("build poll request: %v", err)
	}
	poll.AddCookie(session)
	completed, err := http.DefaultClient.Do(poll)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	defer completed.Body.Close()
	if completed.StatusCode != http.StatusOK {
		answer, _ := io.ReadAll(completed.Body)
		t.Fatalf("poll failed: %d %s", completed.StatusCode, answer)
	}
	var operation struct {
		Asset *struct {
			ID string `json:"id"`
		} `json:"asset"`
	}
	if err := json.NewDecoder(completed.Body).Decode(&operation); err != nil {
		t.Fatalf("decode completed ingest: %v", err)
	}
	if operation.Asset == nil {
		t.Fatal("completed ingest has no asset")
	}
	return operation.Asset.ID
}

// readInPieces reads with a pause between each piece, the way a thin
// connection takes a file.
func readInPieces(from io.Reader, piece int, pause time.Duration) ([]byte, error) {
	var got []byte
	buffer := make([]byte, piece)
	for {
		n, err := from.Read(buffer)
		got = append(got, buffer[:n]...)
		if err == io.EOF {
			return got, nil
		}
		if err != nil {
			return got, err
		}
		time.Sleep(pause)
	}
}

// slowReader hands over a request a piece at a time, with a pause between.
type slowReader struct {
	rest  []byte
	chunk int
	pause time.Duration
}

func (s *slowReader) Read(p []byte) (int, error) {
	if len(s.rest) == 0 {
		return 0, io.EOF
	}
	time.Sleep(s.pause)
	n := copy(p, s.rest[:min(s.chunk, len(s.rest))])
	s.rest = s.rest[n:]
	return n, nil
}

package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

// Handlers turns HTTP requests into catalog calls.
type Handlers struct {
	assets         *asset.Service
	maxUploadBytes int64
}

func NewHandlers(assets *asset.Service, maxUploadBytes int64) *Handlers {
	return &Handlers{assets: assets, maxUploadBytes: maxUploadBytes}
}

func (h *Handlers) ListAssets(c *gin.Context, params ListAssetsParams) {
	f := asset.ListFilter{}

	if params.Kind != nil {
		f.Kind = *params.Kind
	}
	if params.Platform != nil {
		f.Platform, f.PlatformSet = params.Platform, true
	}
	if params.Tag != nil {
		f.Tags = *params.Tag
	}
	if params.Facet != nil {
		f.Facets = parseFacets(*params.Facet)
	}
	if params.Limit != nil {
		f.Limit = *params.Limit
	}

	before, ok := cursorFrom(params)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "before and beforeId belong together, send both or neither",
		})
		return
	}
	f.Before = before

	found, err := h.assets.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list assets"})
		return
	}

	items := make([]Asset, 0, len(found))
	for _, a := range found {
		items = append(items, toAPI(a))
	}
	c.JSON(http.StatusOK, AssetList{Items: items})
}

func (h *Handlers) CreateAsset(c *gin.Context) {
	// Refused before the body is read, so a hopeless upload is never received.
	if c.Request.ContentLength > h.maxUploadBytes {
		h.refuse(c, errOverCeiling)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)

	parts, err := c.Request.MultipartReader()
	if err != nil {
		h.refuse(c, refusal{
			reason: "send the asset as form data, with a metadata part and a file part",
			cause:  err,
		})
		return
	}

	metadata, err := readMetadata(parts)
	if err != nil {
		h.refuse(c, err)
		return
	}
	file, err := nextPart(parts, filePart)
	if err != nil {
		h.refuse(c, err)
		return
	}

	created, err := h.assets.Create(c.Request.Context(), createInput(metadata, file))
	if err != nil {
		h.refuse(c, err)
		return
	}

	c.JSON(http.StatusCreated, toAPI(created))
}

const (
	metadataPart = "metadata"
	filePart     = "file"
)

// readMetadata reads the catalog fields, which come first because the file is
// stored as it arrives and the fields have to be known by then.
func readMetadata(parts *multipart.Reader) (CreateAssetRequest, error) {
	part, err := nextPart(parts, metadataPart)
	if err != nil {
		return CreateAssetRequest{}, err
	}

	var metadata CreateAssetRequest
	if err := json.NewDecoder(part).Decode(&metadata); err != nil {
		return CreateAssetRequest{}, refusal{
			reason: "the " + metadataPart + " part is not valid JSON",
			cause:  err,
		}
	}
	return metadata, nil
}

func nextPart(parts *multipart.Reader, name string) (*multipart.Part, error) {
	part, err := parts.NextPart()
	if errors.Is(err, io.EOF) {
		return nil, refusal{reason: "the " + name + " part is missing", cause: err}
	}
	if err != nil {
		return nil, refusal{reason: "the form data could not be read", cause: err}
	}
	if part.FormName() != name {
		return nil, refusal{
			reason: fmt.Sprintf("expected the %s part here, found %q", name, part.FormName()),
			cause:  nil,
		}
	}
	return part, nil
}

func createInput(metadata CreateAssetRequest, file io.Reader) asset.CreateInput {
	in := asset.CreateInput{
		OwnerID:   uuid.Nil,
		Kind:      metadata.Kind,
		Filename:  metadata.Filename,
		File:      file,
		Name:      metadata.Name,
		Discovery: "listed",
	}
	if metadata.Description != nil {
		in.Description = *metadata.Description
	}
	if metadata.Tags != nil {
		in.Tags = *metadata.Tags
	}
	if metadata.IsNsfw != nil {
		in.IsNSFW = *metadata.IsNsfw
	}
	if metadata.Discovery != nil {
		in.Discovery = string(*metadata.Discovery)
	}
	return in
}

// refusal is a reason a request cannot be accepted, worded for whoever sent
// it. The cause stays attached so the ceiling can still be recognised through
// the layers that wrapped it.
type refusal struct {
	reason string
	cause  error
}

func (r refusal) Error() string { return r.reason }
func (r refusal) Unwrap() error { return r.cause }

// errOverCeiling is a request that declares itself too large to be worth
// reading.
var errOverCeiling = errors.New("upload over the ceiling")

func (h *Handlers) refuse(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) || errors.Is(err, errOverCeiling) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("the upload is over the limit of %d bytes", h.maxUploadBytes),
		})
		return
	}

	var refused refusal
	if errors.As(err, &refused) {
		c.JSON(http.StatusBadRequest, gin.H{"error": refused.Error()})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "could not create the asset"})
}

func (h *Handlers) DownloadOriginal(c *gin.Context, id types.UUID) {
	rc, err := h.assets.OpenOriginal(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, asset.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the file"})
		return
	}
	defer rc.Close()

	// Originals always download. Serving a stored media type would let an
	// uploaded HTML file run as a page on this site.
	c.Header("Content-Disposition", `attachment; filename="original"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, -1, "application/octet-stream", rc, nil)
}

func cursorFrom(params ListAssetsParams) (*asset.Cursor, bool) {
	switch {
	case params.Before == nil && params.BeforeId == nil:
		return nil, true
	case params.Before == nil || params.BeforeId == nil:
		return nil, false
	}
	return &asset.Cursor{MadeAt: *params.Before, ID: uuid.UUID(*params.BeforeId)}, true
}

/** Split "key=value" query values into facets. Anything without an = is ignored. */
func parseFacets(raw []string) []format.Facet {
	out := make([]format.Facet, 0, len(raw))
	for _, pair := range raw {
		for i := 0; i < len(pair); i++ {
			if pair[i] == '=' {
				out = append(out, format.Facet{Key: pair[:i], Value: pair[i+1:]})
				break
			}
		}
	}
	return out
}

func toAPI(a asset.Asset) Asset {
	return Asset{
		Id:                  types.UUID(a.ID),
		Kind:                a.Kind,
		PassthroughPlatform: a.PassthroughPlatform,
		Format:              a.Format,
		Name:                a.Name,
		Description:         a.Description,
		Tags:                a.Tags,
		IsNsfw:              a.IsNSFW,
		Discovery:           AssetDiscovery(a.Discovery),
		CreatedAt:           a.CreatedAt,
	}
}

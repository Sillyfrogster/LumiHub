package format

import (
	"context"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/google/uuid"
)

// OriginIllarin is the origin of an asset built here rather than imported. A
// null origin_format means exactly this and never "unknown", so the builder is
// an origin in its own right and a writer may declare itself tested against it.
const OriginIllarin = "illarin"

// ExportAsset is the asset at the role layer. It holds everything a writer may
// read and nothing about how any of it is stored, which is what makes import
// and export symmetric. A writer never sees another format's bytes.
type ExportAsset struct {
	Kind   string
	Header Header
	// Elements are the asset's role-tagged content in page order. A hidden
	// block's elements are in here, because hiding is a promise about a page
	// and an export is a promise about a file (ADR-0024).
	Elements []block.Element
	// Cover is the picture that stands for the asset, nil where it has none.
	Cover *ExportMedia
	// Images are the pictures elements point at, by the media id an image item
	// carries.
	Images map[uuid.UUID]ExportMedia
	// Preserved is what the asset kept from the file it arrived in. It is
	// filled only where the target belongs to that file's family, so a writer
	// never has to ask whether the data in front of it is its own.
	Preserved []Remainder
}

// ExportMedia is one picture as a writer sees it.
type ExportMedia struct {
	MediaType string
	Data      []byte
}

// Element returns the first element carrying a role, and whether the asset has
// content for it. An element the creator left empty answers false, because the
// question every writer asks is what there is to write.
func (a ExportAsset) Element(role block.Role) (block.Element, bool) {
	for _, element := range a.Elements {
		if element.Role == role && element.Content != nil && !element.Content.Empty() {
			return element, true
		}
	}
	return block.Element{}, false
}

// Content returns the content of the first element carrying a role.
func (a ExportAsset) Content(role block.Role) (block.Content, bool) {
	element, ok := a.Element(role)
	if !ok {
		return nil, false
	}
	return element.Content, true
}

// Text returns the prose written under a role, empty where there is none.
func (a ExportAsset) Text(role block.Role) string {
	content, ok := a.Content(role)
	if !ok {
		return ""
	}
	prose, isProse := content.(block.Prose)
	if !isProse {
		return ""
	}
	return prose.Text
}

// Artifact is one finished file a reader receives.
type Artifact struct {
	Body      []byte
	MediaType string
	// Extension carries the leading dot, as a filename wants it.
	Extension string
}

// Writer is the optional capability that empties an asset's roles into a file.
type Writer interface {
	Module
	Write(context.Context, ExportAsset) (Artifact, error)
}

// TravelsWithOrigin reports whether preserved data from one format belongs in
// another format's file. Two modules that keep their leftovers in the same
// place are the same family, which is the whole of the origin-match rule: a
// namespace goes back where it came from, and nowhere else.
func TravelsWithOrigin(origin, target Declaration) bool {
	return origin.Preservation.Body == target.Preservation.Body &&
		slices.Equal(origin.Preservation.Container, target.Preservation.Container)
}

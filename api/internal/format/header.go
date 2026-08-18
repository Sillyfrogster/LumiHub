package format

import "slices"

// HeaderField is one of the fields an asset carries above its blocks. Header
// fields are the asset's own identity rather than its content, so they have no
// roles, and a writer declares which of them it puts in the file it produces.
//
// The same field is part of one kind's file and no part of another's. A card
// has nowhere to put a blurb, and a theme bundle's description is exactly that
// text, so export relevance is a question about a kind rather than a field.
type HeaderField string

const (
	HeaderName           HeaderField = "name"
	HeaderBlurb          HeaderField = "blurb"
	HeaderAssetVersion   HeaderField = "asset_version"
	HeaderCreditedAuthor HeaderField = "credited_author"
	HeaderNickname       HeaderField = "nickname"
)

// HeaderFields returns the whole vocabulary, in the order it is declared above.
func HeaderFields() []HeaderField {
	return []HeaderField{
		HeaderName, HeaderBlurb, HeaderAssetVersion, HeaderCreditedAuthor,
		HeaderNickname,
	}
}

// Known reports whether the field belongs to the shared header vocabulary.
func (f HeaderField) Known() bool { return slices.Contains(HeaderFields(), f) }

// ExportedHeaderFields names the header fields that reach a file for one kind,
// sorted and each named once. A field no writer for the kind carries is
// presentation there, however much it matters on another kind.
func (r *Registry) ExportedHeaderFields(kind string) []HeaderField {
	fields := make([]HeaderField, 0)
	for _, module := range r.modules {
		declaration := module.Declaration()
		if !declaration.Direction.Write || declaration.Kind != kind {
			continue
		}
		for _, field := range declaration.Header {
			if !slices.Contains(fields, field) {
				fields = append(fields, field)
			}
		}
	}
	slices.Sort(fields)
	return fields
}

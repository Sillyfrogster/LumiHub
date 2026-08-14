package character

import (
	"context"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

// CCv3Module reads a Character Card V3, as a JSON document or inside a raster
// image. A V3 card in an archive is CharX and belongs to that module.
type CCv3Module struct{}

func (CCv3Module) ID() string { return V3 }

func (CCv3Module) BrowseDefinition() format.BrowseDefinition { return browseDefinition() }

func (CCv3Module) Claim(file probe.Inspection) (format.Claim, bool) { return CCv3(file) }

func (m CCv3Module) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	if err := readableVersion(file, claim, 3); err != nil {
		return format.Parsed{}, err
	}
	read, err := readCard(file, claim)
	if err != nil {
		return format.Parsed{}, err
	}
	return format.Parsed{
		Kind:      Kind,
		Format:    m.ID(),
		Name:      read.name(),
		Blurb:     read.blurb(),
		Tags:      read.tags(),
		Facets:    read.facets(),
		Media:     documentImage(file),
		CreatedAt: read.createdAt(),
	}, nil
}

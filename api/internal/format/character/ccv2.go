package character

import (
	"context"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

// CCv2Module reads a Character Card V2, as a JSON document or inside a raster
// image. It also reads the shape cards had before any spec existed, which is
// the only place that shape is ever consulted.
type CCv2Module struct{}

func (CCv2Module) ID() string { return V2 }

func (CCv2Module) BrowseDefinition() format.BrowseDefinition { return browseDefinition() }

func (CCv2Module) Claim(file probe.Inspection) (format.Claim, bool) { return CCv2(file) }

func (m CCv2Module) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	if err := readableVersion(file, claim, 2); err != nil {
		return format.Parsed{}, err
	}
	read, err := readCard(file, claim)
	if err != nil {
		return format.Parsed{}, err
	}
	return format.Parsed{
		Kind:   Kind,
		Format: m.ID(),
		Name:   read.name(),
		Blurb:  read.blurb(),
		Tags:   read.tags(),
		Facets: read.facets(),
		Media:  documentImage(file),
	}, nil
}

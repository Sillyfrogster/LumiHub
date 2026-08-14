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

func (m CCv2Module) BrowseDefinition() format.BrowseDefinition {
	return browseDefinition(m.ExportTargets())
}

func (CCv2Module) ExportTargets() []format.BrowseOption { return exportTargets() }

func (CCv2Module) ValidatePatch(patch format.Patch) error { return validatePatch(patch) }

func (CCv2Module) Export(_ context.Context, request format.ExportRequest) (format.ExportedArtifact, error) {
	return exportCard(request, V2, "chara")
}

func (CCv2Module) Claim(file probe.Inspection) (format.Claim, bool) { return CCv2(file) }

func (m CCv2Module) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	read, err := readCard(file, claim, 2)
	if err != nil {
		return format.Parsed{}, err
	}
	return read.parsed(m.ID(), documentImage(file)), nil
}

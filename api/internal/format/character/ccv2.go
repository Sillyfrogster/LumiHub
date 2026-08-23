package character

import (
	"context"

	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
)

// CCv2Module reads a Character Card V2, as a JSON document or inside a raster
// image. It also reads the shape cards had before any spec existed, which is
// the only place that shape is ever consulted.
type CCv2Module struct{}

func (CCv2Module) ID() string { return V2 }

func (CCv2Module) Declaration() format.Declaration { return declaration(V2) }

func (m CCv2Module) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

func (m CCv2Module) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	read, err := readCard(file, claim, 2, m.ID())
	if err != nil {
		return format.Parsed{}, err
	}
	return read.parsed(m.ID(), documentImage(file))
}

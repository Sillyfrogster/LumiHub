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

func (CCv3Module) Declaration() format.Declaration { return declaration(V3) }

func (m CCv3Module) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

func (m CCv3Module) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	read, err := readCard(file, claim, 3, m.ID())
	if err != nil {
		return format.Parsed{}, err
	}
	return read.parsed(m.ID(), documentImage(file))
}

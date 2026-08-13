package format

import (
	"context"
	"io"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

/** A key and value a module extracts so Browse can filter on it */
type Facet struct {
	Key   string
	Value string
}

/** What a module reads out of an uploaded file */
type Parsed struct {
	Kind                string
	PassthroughPlatform *string
	Format              string
	Name                string
	Description         string
	Tags                []string
	Facets              []Facet
	// CreatedAt is the date the file carries. Nil means the file does not say.
	CreatedAt *time.Time
}

/** The minimum every format module implements */
type Module interface {
	ID() string
	Claim(probe.Inspection) (Claim, bool)
	Parse(ctx context.Context, file probe.Inspection, claim Claim) (Parsed, error)
}

type claimStrength uint8

const (
	compatibility claimStrength = iota + 1
	authoritative
)

type Claim struct {
	payloadID uint32
	strength  claimStrength
	formatID  string
}

func AuthoritativeClaim(payload probe.Payload, discriminator string) (Claim, bool) {
	formatID, ok := payload.String(discriminator)
	if !ok || formatID == "" {
		return Claim{}, false
	}
	return Claim{payloadID: payload.ID, strength: authoritative, formatID: formatID}, true
}

func CompatibilityClaim(payload probe.Payload) Claim {
	return Claim{payloadID: payload.ID, strength: compatibility}
}

func (c Claim) Payload(file probe.Inspection) (probe.Payload, bool) {
	for _, payload := range file.Payloads {
		if payload.ID == c.payloadID {
			return payload, true
		}
	}
	return probe.Payload{}, false
}

/** Implemented only by modules that can change a file without losing data */
type Editor interface {
	Edit(ctx context.Context, src io.Reader, patch []byte) (io.Reader, error)
}

/** Implemented only by modules that can write a file out in another format */
type Exporter interface {
	Export(ctx context.Context, src io.Reader, target string) (io.Reader, error)
}

/** A labelled block of plain text, for quality scoring and moderation */
type TextSection struct {
	Label string
	Text  string
}

/** Implemented only by modules that can pull readable text out of a file */
type TextExtractor interface {
	ExtractText(ctx context.Context, src io.Reader) ([]TextSection, error)
}

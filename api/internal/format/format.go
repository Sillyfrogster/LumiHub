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
	Claim(probe.Result) (Claim, bool)
	Parse(ctx context.Context, file probe.Result, claim Claim) (Parsed, error)
}

type ClaimStrength uint8

const (
	Compatibility ClaimStrength = iota + 1
	Authoritative
)

type Claim struct {
	PayloadID uint32
	Strength  ClaimStrength
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

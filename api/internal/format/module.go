package format

import (
	"context"
	"io"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

// Module provides a format's identity and declaration.
type Module interface {
	ID() string
	Declaration() Declaration
}

// Reader claims and parses source bytes.
type Reader interface {
	Module
	Claim(probe.Inspection) (Claim, bool)
	Parse(ctx context.Context, file probe.Inspection, claim Claim) (Parsed, error)
}

// DatabaseReader reads a database row through the module interface.
type DatabaseReader interface {
	Module
	ReadDatabaseRow(ctx context.Context, row any) (Parsed, error)
}

// SpecOwner names standards contained by a format.
type SpecOwner interface {
	OwnedSpecs() []string
}

func ownsSpec(module Module, spec string) bool {
	if spec == module.ID() {
		return true
	}
	owner, ok := module.(SpecOwner)
	return ok && slices.Contains(owner.OwnedSpecs(), spec)
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
	byteSize  int64
}

const wholeFilePayloadID = ^uint32(0)

// WholeFileCompatibilityClaim claims a container as one payload.
func WholeFileCompatibilityClaim(file probe.Inspection) Claim {
	return Claim{
		payloadID: wholeFilePayloadID, strength: compatibility, byteSize: file.ByteSize(),
	}
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
	if c.payloadID == wholeFilePayloadID {
		return probe.Payload{ID: wholeFilePayloadID, ByteSize: c.byteSize}, true
	}
	for _, payload := range file.Payloads {
		if payload.ID == c.payloadID {
			return payload, true
		}
	}
	return probe.Payload{}, false
}

// TextBlock is one labelled block of extracted text.
type TextBlock struct {
	Label string
	Text  string
}

// TextExtractor reads text from formats that provide it.
type TextExtractor interface {
	ExtractText(ctx context.Context, src io.Reader) ([]TextBlock, error)
}

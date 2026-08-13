package format

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

type FailureReason string

const (
	FailureMalformedInput     FailureReason = "malformed_input"
	FailureUnsupportedFormat  FailureReason = "unsupported_format"
	FailureUnsupportedVersion FailureReason = "unsupported_version"
	FailureSafetyViolation    FailureReason = "safety_violation"
	FailureInternal           FailureReason = "internal_failure"
)

type failure struct {
	reason FailureReason
	cause  error
}

func (f failure) Error() string { return fmt.Sprintf("%s: %v", f.reason, f.cause) }
func (f failure) Unwrap() error { return f.cause }

// UnsupportedVersion marks a format revision the module cannot safely read.
func UnsupportedVersion(err error) error {
	return failure{reason: FailureUnsupportedVersion, cause: err}
}

// SafetyViolation marks a bounded-resource or structural refusal found by a module.
func SafetyViolation(err error) error {
	return failure{reason: FailureSafetyViolation, cause: err}
}

// InternalFailure marks infrastructure or a module bug that may succeed on retry.
func InternalFailure(err error) error {
	return failure{reason: FailureInternal, cause: err}
}

// FailureOf returns the creator-facing category carried by err.
func FailureOf(err error) (FailureReason, bool) {
	var classified failure
	if !errors.As(err, &classified) {
		return "", false
	}
	return classified.reason, true
}

/** A key and value a module extracts so Browse can filter on it */
type Facet struct {
	Key   string
	Value string
}

// Media is one image extracted from a source file.
type Media struct {
	Role  media.Role
	Bytes []byte
}

/** What a module reads out of an uploaded file */
type Parsed struct {
	Kind                string
	PassthroughPlatform *string
	Format              string
	Name                string
	Blurb               string
	Tags                []string
	IsNSFW              *bool
	Facets              []Facet
	Media               []Media
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

package format

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

type FailureReason string

const (
	FailureMalformedInput     FailureReason = "malformed_input"
	FailureUnsupportedFormat  FailureReason = "unsupported_format"
	FailureUnsupportedVersion FailureReason = "unsupported_version"
	FailureSafetyViolation    FailureReason = "safety_violation"
	FailureWrongKind          FailureReason = "wrong_kind"
	FailureLimitExceeded      FailureReason = "limit_exceeded"
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

// MalformedInput marks a recognized payload the reader cannot interpret.
func MalformedInput(err error) error {
	return failure{reason: FailureMalformedInput, cause: err}
}

// SafetyViolation marks a bounded-resource or structural refusal found by a module.
func SafetyViolation(err error) error {
	return failure{reason: FailureSafetyViolation, cause: err}
}

// LimitExceeded marks valid input whose declared content is too large to import.
func LimitExceeded(err error) error {
	return failure{reason: FailureLimitExceeded, cause: err}
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

type BrowseOption struct {
	Value string
	Label string
}

type BrowseFacet struct {
	Key       string
	Label     string
	Platforms []string
	Options   []BrowseOption
}

type BrowseDefinition struct {
	Kind          string
	ExportTargets []BrowseOption
	Facets        []BrowseFacet
}

type BrowseDeclarer interface {
	BrowseDefinition() BrowseDefinition
}

// ExportTargetDeclarer names every non-raw artifact a module can produce.
// Browse may expose a smaller, platform-oriented subset of these targets.
type ExportTargetDeclarer interface {
	ExportTargets() []BrowseOption
}

// Media is one image a module found in a source file and gave a role. It names
// an image the probe located rather than carrying bytes, so a module never
// holds a file in memory and never writes one.
type Media struct {
	Role        media.Role
	ImageID     uint32
	ElementRole block.Role
	Name        string
}

/** What a module reads out of an uploaded file */
type Parsed struct {
	Kind   string
	Format string
	Blurb  string
	Tags   []string
	IsNSFW *bool
	Facets []Facet
	Media  []Media
	// CreatedAt is the date the file carries. Nil means the file does not say.
	CreatedAt *time.Time
	Header    Header
	Elements  []block.Element
	Remainder []Remainder
}

// Header is creator-authored identity that stays above the builder.
type Header struct {
	Name           string
	AssetVersion   string
	CreditedAuthor string
	Nickname       string
}

// Remainder is source data a reader could not turn into an element or header.
type Remainder struct {
	Namespace string
	Payload   []byte
}

type Direction struct {
	Read  bool
	Write bool
}

type RecognitionKind string

const (
	RecognitionDiscriminator RecognitionKind = "discriminator"
	RecognitionSignature     RecognitionKind = "signature"
)

type ValueType string

const (
	ValueString  ValueType = "string"
	ValueNumber  ValueType = "number"
	ValueBoolean ValueType = "boolean"
	ValueObject  ValueType = "object"
	ValueArray   ValueType = "array"
)

// Recognition is declared evidence that a payload belongs to one format.
type Recognition struct {
	Kind       RecognitionKind
	Containers []probe.Container
	Path       []string
	Values     []string
	Required   map[string]ValueType
	LegacyOnly bool
	// SupersededBy names values at the same Path that outrank this one.
	SupersededBy []string
}

// ClaimByDeclaration applies only the recognition data beside a module. The
// module keeps normalization code, but recognition has no hidden code path.
func ClaimByDeclaration(file probe.Inspection, declaration Declaration) (Claim, bool) {
	for _, recognition := range declaration.Recognition {
		if supersededInFile(file, recognition) {
			continue
		}
		for _, payload := range file.Payloads {
			if len(recognition.Containers) > 0 &&
				!slices.Contains(recognition.Containers, payload.Locator.Container) {
				continue
			}
			switch recognition.Kind {
			case RecognitionDiscriminator:
				value, ok := payloadValue(payload.Root, recognition.Path)
				if !ok || !slices.Contains(recognition.Values, value) {
					continue
				}
				return Claim{
					payloadID: payload.ID, strength: authoritative, formatID: declaration.ID,
				}, true
			case RecognitionSignature:
				if recognition.LegacyOnly {
					if spec, _ := payload.String("spec"); spec != "" {
						continue
					}
				}
				if signatureMatches(payload.Root, recognition.Required) {
					return CompatibilityClaim(payload), true
				}
			}
		}
	}
	return Claim{}, false
}

// supersededInFile reports whether the file carries a value that outranks this recognition.
func supersededInFile(file probe.Inspection, recognition Recognition) bool {
	if len(recognition.SupersededBy) == 0 {
		return false
	}
	for _, payload := range file.Payloads {
		if len(recognition.Containers) > 0 &&
			!slices.Contains(recognition.Containers, payload.Locator.Container) {
			continue
		}
		value, ok := payloadValue(payload.Root, recognition.Path)
		if ok && slices.Contains(recognition.SupersededBy, value) {
			return true
		}
	}
	return false
}

func payloadValue(root map[string]json.RawMessage, path []string) (string, bool) {
	current := root
	for i, part := range path {
		raw, ok := current[part]
		if !ok {
			return "", false
		}
		if i == len(path)-1 {
			var value string
			if json.Unmarshal(raw, &value) == nil {
				return value, true
			}
			if kind := jsonValueType(raw); kind == ValueNumber || kind == ValueBoolean {
				return string(bytes.TrimSpace(raw)), true
			}
			return "", false
		}
		if json.Unmarshal(raw, &current) != nil {
			return "", false
		}
	}
	return "", false
}

func signatureMatches(root map[string]json.RawMessage, required map[string]ValueType) bool {
	for key, wanted := range required {
		raw, ok := root[key]
		if !ok || jsonValueType(raw) != wanted {
			return false
		}
	}
	return true
}

func jsonValueType(raw json.RawMessage) ValueType {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	switch value.(type) {
	case string:
		return ValueString
	case float64:
		return ValueNumber
	case bool:
		return ValueBoolean
	case []any:
		return ValueArray
	case map[string]any:
		return ValueObject
	default:
		return ""
	}
}

type SupportGrade string

const (
	SupportFull    SupportGrade = "full"
	SupportPartial SupportGrade = "partial"
	SupportNone    SupportGrade = "none"
)

// ContentCondition makes partial support executable against the element that
// carries a role. Description is the wording shown with a loss.
type ContentCondition struct {
	Description string
	Matches     func(block.Content) bool
}

type RoleSupport struct {
	Grade     SupportGrade
	Condition *ContentCondition
}

type DirectionalRoleSupport struct {
	Read  RoleSupport
	Write RoleSupport
}

type SlotDeclaration struct {
	Name        string
	Type        ValueType
	Constraints []string
}

type ContentLimits struct {
	PayloadBytes    int
	CollectionItems int
	ItemBytes       int
}

type Boilerplate struct {
	Namespace string
	Path      []string
}

type PreservationDeclaration struct {
	Namespaces []string
}

// Declaration is the complete static contract beside one format module.
type Declaration struct {
	ID            string
	Kind          string
	Direction     Direction
	Recognition   []Recognition
	Roles         map[block.Role]DirectionalRoleSupport
	Slots         []SlotDeclaration
	Limits        ContentLimits
	ConsumedKeys  []string
	Boilerplate   []Boilerplate
	Preservation  PreservationDeclaration
	TestedOrigins []string
}

// ValidateDeclaration checks the parts registry consumers rely on without
// asking the module to parse a file.
func ValidateDeclaration(d Declaration) error {
	if d.ID == "" || d.Kind == "" {
		return errors.New("identity and kind are required")
	}
	if !d.Direction.Read && !d.Direction.Write {
		return errors.New("at least one direction is required")
	}
	if d.Direction.Read && len(d.Recognition) == 0 {
		return errors.New("a reader needs declared recognition")
	}
	for _, recognition := range d.Recognition {
		if len(recognition.Containers) == 0 {
			return errors.New("recognition needs at least one container")
		}
		switch recognition.Kind {
		case RecognitionDiscriminator:
			if len(recognition.Path) == 0 || len(recognition.Values) == 0 {
				return errors.New("a discriminator needs a location and accepted values")
			}
			for _, superseding := range recognition.SupersededBy {
				if slices.Contains(recognition.Values, superseding) {
					return fmt.Errorf("value %q both matches and supersedes the discriminator", superseding)
				}
			}
		case RecognitionSignature:
			if len(recognition.Required) == 0 {
				return errors.New("a structural signature needs required keys")
			}
			if len(recognition.SupersededBy) > 0 {
				return errors.New("only a discriminator can name what supersedes it")
			}
			for key, valueType := range recognition.Required {
				if key == "" || !valueType.known() {
					return fmt.Errorf("structural key %q has type %q", key, valueType)
				}
			}
		default:
			return fmt.Errorf("unknown recognition kind %q", recognition.Kind)
		}
	}
	for role, directional := range d.Roles {
		if !role.Known() {
			return fmt.Errorf("unknown semantic role %q", role)
		}
		for direction, support := range map[string]RoleSupport{"read": directional.Read, "write": directional.Write} {
			if support.Grade != SupportFull && support.Grade != SupportPartial && support.Grade != SupportNone {
				return fmt.Errorf("%s %s support has grade %q", role, direction, support.Grade)
			}
			if support.Grade == SupportPartial &&
				(support.Condition == nil || support.Condition.Matches == nil || support.Condition.Description == "") {
				return fmt.Errorf("%s %s partial support needs a content condition", role, direction)
			}
		}
	}
	seenSlots := make(map[string]bool)
	for _, slot := range d.Slots {
		if slot.Name == "" || !slot.Type.known() {
			return fmt.Errorf("slot %q has type %q", slot.Name, slot.Type)
		}
		if seenSlots[slot.Name] {
			return fmt.Errorf("slot %q is declared twice", slot.Name)
		}
		seenSlots[slot.Name] = true
	}
	if d.Limits.PayloadBytes <= 0 || d.Limits.CollectionItems <= 0 || d.Limits.ItemBytes <= 0 {
		return errors.New("payload, collection and item limits are required")
	}
	if len(d.ConsumedKeys) == 0 {
		return errors.New("consumed keys are required")
	}
	if len(d.Preservation.Namespaces) == 0 {
		return errors.New("preservation namespaces are required")
	}
	if len(d.TestedOrigins) == 0 {
		return errors.New("tested origins are required")
	}
	return nil
}

func (t ValueType) known() bool {
	return t == ValueString || t == ValueNumber || t == ValueBoolean ||
		t == ValueObject || t == ValueArray
}

// Module is the identity and static contract every format module implements.
type Module interface {
	ID() string
	Declaration() Declaration
}

// Reader is the optional capability that claims and parses source bytes.
type Reader interface {
	Module
	Claim(probe.Inspection) (Claim, bool)
	Parse(ctx context.Context, file probe.Inspection, claim Claim) (Parsed, error)
}

/**
 * Implemented only by a module whose payloads name a standard rather than the
 * module itself. CharX is the one: a `.charx` archive holds a card declaring
 * `chara_card_v3`, because CharX is a container for a CCv3 card and has no
 * discriminator of its own.
 */
type SpecOwner interface {
	OwnedSpecs() []string
}

// Field names one semantic part of a creator file that a module may patch.
type Field string

const (
	FieldDescription             Field = "description"
	FieldPersonality             Field = "personality"
	FieldScenario                Field = "scenario"
	FieldFirstMessage            Field = "first_mes"
	FieldSystemPrompt            Field = "system_prompt"
	FieldPostHistoryInstructions Field = "post_history_instructions"
	FieldCreatorNotes            Field = "creator_notes"
	FieldCharacterVersion        Field = "character_version"
)

// Patch is a creator's named changes to their file.
type Patch map[Field]string

var ErrInvalidPatch = errors.New("invalid file patch")

const RawTarget = "raw"

// ValidatePatchFields checks a patch against the fields one module owns.
func ValidatePatchFields(patch Patch, fields ...Field) error {
	for field, value := range patch {
		if !slices.Contains(fields, field) {
			return fmt.Errorf("field %q is not patchable: %w", field, ErrInvalidPatch)
		}
		if !utf8.ValidString(value) {
			return fmt.Errorf("field %q is not UTF-8: %w", field, ErrInvalidPatch)
		}
	}
	return nil
}

// Patcher is implemented by modules that map semantic fields into their files.
type Patcher interface {
	ValidatePatch(Patch) error
}

// ExportMedia is creator-managed media available while writing an artifact.
type ExportMedia struct {
	Role      media.Role
	MediaType string
	Data      []byte
}

// ExportRequest is everything a format module may merge into one artifact.
type ExportRequest struct {
	Source io.Reader
	Target string
	Patch  Patch
	Media  []ExportMedia
}

// ExportedArtifact is one primary file and any media it could not embed.
type ExportedArtifact struct {
	Artifact        io.Reader
	MediaType       string
	Extension       string
	UnembeddedMedia []ExportMedia
}

// ownsSpec reports whether an authoritative claim naming spec belongs to m.
func ownsSpec(m Module, spec string) bool {
	if spec == m.ID() {
		return true
	}
	owner, ok := m.(SpecOwner)
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

// WholeFileCompatibilityClaim supports a module whose structure is the
// container itself rather than one decoded JSON payload.
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

/** Implemented only by modules that can write a file out in another format */
type Exporter interface {
	Export(context.Context, ExportRequest) (ExportedArtifact, error)
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

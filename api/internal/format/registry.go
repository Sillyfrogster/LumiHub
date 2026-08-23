package format

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

var (
	ErrInvariant         = errors.New("format registry invariant")
	ErrConflictingClaims = fmt.Errorf("%w: conflicting authoritative claims", ErrInvariant)
	ErrAmbiguousClaims   = fmt.Errorf("%w: ambiguous format claims", ErrInvariant)
	ErrInvalidClaim      = fmt.Errorf("%w: invalid format claim", ErrInvariant)
	ErrUnsupportedFormat = errors.New("unsupported format")
)

type Resolution struct {
	Module Reader
	Claim  Claim
}

/** Holds every known module and picks the right one for a file */
type Registry struct {
	modules map[string]Module
}

func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]Module)}
}

func (r *Registry) Empty() bool { return len(r.modules) == 0 }

func (r *Registry) Register(m Module) error {
	if _, taken := r.modules[m.ID()]; taken {
		return fmt.Errorf("format module %q is already registered", m.ID())
	}
	r.modules[m.ID()] = m
	return nil
}

// ValidateDeclarations checks the static contract every registry consumer
// reads without invoking module code.
func (r *Registry) ValidateDeclarations() error {
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		module := r.modules[id]
		declaration := module.Declaration()
		if declaration.ID != id {
			return fmt.Errorf("module %q declares identity %q: %w", id, declaration.ID, ErrInvariant)
		}
		if err := ValidateDeclaration(declaration); err != nil {
			return fmt.Errorf("module %q declaration: %w", id, err)
		}
		if declaration.Direction.Read && declaration.Input == InputFile {
			if _, ok := module.(Reader); !ok {
				return fmt.Errorf("module %q declares read support without a reader: %w", id, ErrInvariant)
			}
		}
		if declaration.Direction.Read && declaration.Input == InputDatabaseRow {
			if _, ok := module.(DatabaseReader); !ok {
				return fmt.Errorf("module %q declares row read support without a database reader: %w", id, ErrInvariant)
			}
		}
		if declaration.Direction.Write {
			if _, ok := module.(Writer); !ok {
				return fmt.Errorf("module %q declares write support without a writer: %w", id, ErrInvariant)
			}
		}
	}
	for i, firstID := range ids {
		first := r.modules[firstID].Declaration()
		for _, secondID := range ids[i+1:] {
			second := r.modules[secondID].Declaration()
			if signaturesOverlap(first.Recognition, second.Recognition) {
				return fmt.Errorf(
					"modules %q and %q have overlapping structural signatures: %w",
					firstID, secondID, ErrInvariant,
				)
			}
		}
	}
	return nil
}

// signaturesOverlap reports whether one structural signature shadows another.
// Distinct required keys are separable; equal-strength matches fail closed.
func signaturesOverlap(first, second []Recognition) bool {
	for _, a := range first {
		if a.Kind != RecognitionSignature {
			continue
		}
		for _, b := range second {
			if b.Kind != RecognitionSignature || incompatibleContainers(a.Containers, b.Containers) {
				continue
			}
			if shadows(a.Required, b.Required) || shadows(b.Required, a.Required) {
				return true
			}
		}
	}
	return false
}

// shadows reports whether every key the looser signature requires is required
// by the stricter one at the same type.
func shadows(looser, stricter map[string]ValueType) bool {
	for key, wanted := range looser {
		if held, present := stricter[key]; !present || held != wanted {
			return false
		}
	}
	return true
}

func incompatibleContainers(first, second []probe.Container) bool {
	if len(first) == 0 || len(second) == 0 {
		return false
	}
	for _, container := range first {
		if slices.Contains(second, container) {
			return false
		}
	}
	return true
}

func (r *Registry) ByID(id string) (Module, bool) {
	m, ok := r.modules[id]
	return m, ok
}

// Declaration returns one registered module's contract, for code that needs to
// read what a format declares without asking it to parse anything.
func (r *Registry) Declaration(id string) (Declaration, bool) {
	module, ok := r.modules[id]
	if !ok {
		return Declaration{}, false
	}
	return module.Declaration(), true
}

// ReadableLabels names every format the registry can read, in the order a
// person would read them out. A refusal is built from this rather than from a
// sentence somebody has to remember to update.
func (r *Registry) ReadableLabels() []string {
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	labels := make([]string, 0, len(ids))
	for _, id := range ids {
		declaration := r.modules[id].Declaration()
		if declaration.Direction.Read && declaration.Input == InputFile {
			labels = append(labels, declaration.Label)
		}
	}
	return labels
}

func (r *Registry) Resolve(file probe.Inspection) (Resolution, bool, error) {
	var candidates []Resolution
	for _, module := range r.modules {
		declaration := module.Declaration()
		if !declaration.Direction.Read {
			continue
		}
		reader, ok := module.(Reader)
		if !ok {
			continue
		}
		claim, ok := reader.Claim(file)
		if !ok {
			continue
		}
		if err := validateClaim(file, reader, claim); err != nil {
			return Resolution{}, false, fmt.Errorf("module %q: %w", module.ID(), err)
		}
		candidates = append(candidates, Resolution{Module: reader, Claim: claim})
	}
	if len(candidates) == 0 {
		if err := r.unsupportedDiscriminator(file); err != nil {
			return Resolution{}, false, err
		}
		return Resolution{}, false, nil
	}

	for i, first := range candidates {
		if first.Claim.strength != authoritative {
			continue
		}
		for _, second := range candidates[i+1:] {
			if second.Claim.strength == authoritative && second.Claim.payloadID == first.Claim.payloadID {
				return Resolution{}, false, fmt.Errorf(
					"modules %q and %q claimed payload %d: %w",
					first.Module.ID(), second.Module.ID(), first.Claim.payloadID, ErrConflictingClaims,
				)
			}
		}
	}

	strongest := candidates[0].Claim.strength
	for _, candidate := range candidates[1:] {
		strongest = max(strongest, candidate.Claim.strength)
	}
	var winners []Resolution
	for _, candidate := range candidates {
		if candidate.Claim.strength == strongest {
			winners = append(winners, candidate)
		}
	}
	if len(winners) > 1 {
		return Resolution{}, false, fmt.Errorf(
			"modules %q and %q made equally strong claims: %w",
			winners[0].Module.ID(), winners[1].Module.ID(), ErrAmbiguousClaims,
		)
	}
	return winners[0], true, nil
}

func (r *Registry) unsupportedDiscriminator(file probe.Inspection) error {
	type observation struct {
		kind    string
		path    string
		value   string
		formats []string
	}
	observations := make(map[string]*observation)
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		declaration := r.modules[id].Declaration()
		for _, recognition := range declaration.Recognition {
			if recognition.Kind != RecognitionDiscriminator {
				continue
			}
			for _, payload := range file.Payloads {
				if len(recognition.Containers) > 0 &&
					!slices.Contains(recognition.Containers, payload.Locator.Container) {
					continue
				}
				value, present := payloadValue(payload.Root, recognition.Path)
				if !present || slices.Contains(recognition.Values, value) {
					continue
				}
				path := slices.Concat(recognition.Path)
				joined := ""
				for i, part := range path {
					if i > 0 {
						joined += "."
					}
					joined += part
				}
				key := declaration.Kind + "\x00" + joined + "\x00" + value
				found := observations[key]
				if found == nil {
					found = &observation{kind: declaration.Kind, path: joined, value: value}
					observations[key] = found
				}
				found.formats = append(found.formats, declaration.ID)
			}
		}
	}
	if len(observations) == 0 {
		return nil
	}
	keys := make([]string, 0, len(observations))
	for key := range observations {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	found := observations[keys[0]]
	if len(found.formats) == 1 {
		return fmt.Errorf(
			"format %q recognises discriminator %q but cannot read value %q: %w",
			found.formats[0], found.path, found.value, ErrUnsupportedFormat,
		)
	}
	return fmt.Errorf(
		"formats for kind %q recognise discriminator %q but cannot read value %q: %w",
		found.kind, found.path, found.value, ErrUnsupportedFormat,
	)
}

func validateClaim(file probe.Inspection, module Reader, claim Claim) error {
	if claim.strength != compatibility && claim.strength != authoritative {
		return fmt.Errorf("strength %d: %w", claim.strength, ErrInvalidClaim)
	}
	if _, ok := claim.Payload(file); !ok {
		return fmt.Errorf("payload %d does not exist: %w", claim.payloadID, ErrInvalidClaim)
	}
	if claim.strength == authoritative && !ownsSpec(module, claim.formatID) {
		return fmt.Errorf("payload names format %q, module is %q: %w", claim.formatID, module.ID(), ErrInvalidClaim)
	}
	if claim.strength == compatibility && claim.formatID != "" {
		return fmt.Errorf("compatibility claim names format %q: %w", claim.formatID, ErrInvalidClaim)
	}
	return nil
}

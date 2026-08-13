package format

import (
	"errors"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

var (
	ErrInvariant         = errors.New("format registry invariant")
	ErrConflictingClaims = fmt.Errorf("%w: conflicting authoritative claims", ErrInvariant)
	ErrAmbiguousClaims   = fmt.Errorf("%w: ambiguous format claims", ErrInvariant)
	ErrInvalidClaim      = fmt.Errorf("%w: invalid format claim", ErrInvariant)
)

type Resolution struct {
	Module Module
	Claim  Claim
}

/** Holds every known module and picks the right one for a file */
type Registry struct {
	modules map[string]Module
}

func NewRegistry() *Registry {
	return &Registry{modules: make(map[string]Module)}
}

func (r *Registry) Register(m Module) error {
	if _, taken := r.modules[m.ID()]; taken {
		return fmt.Errorf("format module %q is already registered", m.ID())
	}
	r.modules[m.ID()] = m
	return nil
}

func (r *Registry) ByID(id string) (Module, bool) {
	m, ok := r.modules[id]
	return m, ok
}

func (r *Registry) CanEdit(id string) bool {
	m, ok := r.modules[id]
	if !ok {
		return false
	}
	_, editable := m.(Editor)
	return editable
}

func (r *Registry) Resolve(file probe.Inspection) (Resolution, bool, error) {
	var candidates []Resolution
	for _, module := range r.modules {
		claim, ok := module.Claim(file)
		if !ok {
			continue
		}
		if err := validateClaim(file, module, claim); err != nil {
			return Resolution{}, false, fmt.Errorf("module %q: %w", module.ID(), err)
		}
		candidates = append(candidates, Resolution{Module: module, Claim: claim})
	}
	if len(candidates) == 0 {
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

func validateClaim(file probe.Inspection, module Module, claim Claim) error {
	if claim.strength != compatibility && claim.strength != authoritative {
		return fmt.Errorf("strength %d: %w", claim.strength, ErrInvalidClaim)
	}
	if _, ok := claim.Payload(file); !ok {
		return fmt.Errorf("payload %d does not exist: %w", claim.payloadID, ErrInvalidClaim)
	}
	if claim.strength == authoritative && claim.formatID != module.ID() {
		return fmt.Errorf("payload names format %q, module is %q: %w", claim.formatID, module.ID(), ErrInvalidClaim)
	}
	if claim.strength == compatibility && claim.formatID != "" {
		return fmt.Errorf("compatibility claim names format %q: %w", claim.formatID, ErrInvalidClaim)
	}
	return nil
}

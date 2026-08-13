package format

import (
	"errors"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

var (
	ErrConflictingClaims = errors.New("conflicting authoritative claims")
	ErrAmbiguousClaims   = errors.New("ambiguous format claims")
	ErrInvalidClaim      = errors.New("invalid format claim")
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

func (r *Registry) Resolve(file probe.Result) (Resolution, bool, error) {
	var candidates []Resolution
	for _, module := range r.modules {
		claim, ok := module.Claim(file)
		if !ok {
			continue
		}
		if err := validateClaim(file, claim); err != nil {
			return Resolution{}, false, fmt.Errorf("module %q: %w", module.ID(), err)
		}
		candidates = append(candidates, Resolution{Module: module, Claim: claim})
	}
	if len(candidates) == 0 {
		return Resolution{}, false, nil
	}

	for i, first := range candidates {
		if first.Claim.Strength != Authoritative {
			continue
		}
		for _, second := range candidates[i+1:] {
			if second.Claim.Strength == Authoritative && second.Claim.PayloadID == first.Claim.PayloadID {
				return Resolution{}, false, fmt.Errorf(
					"modules %q and %q claimed payload %d: %w",
					first.Module.ID(), second.Module.ID(), first.Claim.PayloadID, ErrConflictingClaims,
				)
			}
		}
	}

	strongest := candidates[0].Claim.Strength
	for _, candidate := range candidates[1:] {
		strongest = max(strongest, candidate.Claim.Strength)
	}
	var winners []Resolution
	for _, candidate := range candidates {
		if candidate.Claim.Strength == strongest {
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

func validateClaim(file probe.Result, claim Claim) error {
	if claim.Strength != Compatibility && claim.Strength != Authoritative {
		return fmt.Errorf("strength %d: %w", claim.Strength, ErrInvalidClaim)
	}
	for _, payload := range file.Payloads {
		if payload.ID == claim.PayloadID {
			return nil
		}
	}
	return fmt.Errorf("payload %d does not exist: %w", claim.PayloadID, ErrInvalidClaim)
}

package format

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/probe"
)

type claimingModule struct {
	id            string
	spec          string
	authoritative bool
}

func (m claimingModule) ID() string { return m.id }
func (m claimingModule) Declaration() Declaration {
	declaration := testReaderDeclaration(m.id, "character")
	declaration.Recognition = []Recognition{{
		Kind: RecognitionDiscriminator, Path: []string{"spec"}, Values: []string{m.spec},
		Containers: []probe.Container{probe.PNG},
	}}
	return declaration
}
func (m claimingModule) Parse(context.Context, probe.Inspection, Claim) (Parsed, error) {
	return Parsed{Format: m.id}, nil
}
func (m claimingModule) Claim(file probe.Inspection) (Claim, bool) {
	for _, payload := range file.Payloads {
		if spec, ok := payload.String("spec"); ok && spec == m.spec {
			if m.authoritative {
				return AuthoritativeClaim(payload, "spec")
			}
			return CompatibilityClaim(payload), true
		}
	}
	return Claim{}, false
}

func TestResolveReturnsNoModuleWhenNothingClaimsTheFile(t *testing.T) {
	registry := NewRegistry()

	_, ok, err := registry.Resolve(probe.Inspection{Container: probe.Unknown})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ok {
		t.Fatal("an unclaimed file resolved to a module")
	}
}

func TestResolveRejectsAPayloadThatNamesAnUnsupportedFormat(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(claimingModule{
		id: "chara_card_v3", spec: "chara_card_v3", authoritative: true,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, ok, err := registry.Resolve(probedPayload("future_card", "card"))
	if ok {
		t.Fatal("unsupported format resolved to a module")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Resolve error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestResolvePrefersAnAuthoritativeClaimRegardlessOfRegistrationOrder(t *testing.T) {
	file := probedPayload("chara_card_v3", "ccv3")
	compatible := claimingModule{id: "compatible_reader", spec: "chara_card_v3"}
	authority := claimingModule{id: "chara_card_v3", spec: "chara_card_v3", authoritative: true}

	for _, modules := range [][]Module{
		{compatible, authority},
		{authority, compatible},
	} {
		registry := NewRegistry()
		for _, module := range modules {
			if err := registry.Register(module); err != nil {
				t.Fatalf("Register: %v", err)
			}
		}

		got, ok, err := registry.Resolve(file)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if !ok || got.Module.ID() != "chara_card_v3" {
			t.Fatalf("resolved module = %v, %v; want chara_card_v3, true", got.Module, ok)
		}
	}
}

func TestResolveRejectsTwoAuthoritativeClaimsOnOnePayload(t *testing.T) {
	registry := NewRegistry()
	for _, id := range []string{"first", "second"} {
		if err := registry.Register(forcedAuthoritativeModule{id: id}); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	_, _, err := registry.Resolve(probedPayload("chara_card_v2", "chara"))
	if !errors.Is(err, ErrConflictingClaims) {
		t.Fatalf("Resolve error = %v, want ErrConflictingClaims", err)
	}
}

func TestResolveUsesThePayloadDiscriminatorRatherThanItsLocator(t *testing.T) {
	registry := NewRegistry()
	for _, module := range []Module{
		claimingModule{id: "chara_card_v3", spec: "chara_card_v3", authoritative: true},
		claimingModule{id: "chara_card_v2", spec: "chara_card_v2", authoritative: true},
	} {
		if err := registry.Register(module); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	got, ok, err := registry.Resolve(probedPayload("chara_card_v2", "ccv3"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok || got.Module.ID() != "chara_card_v2" {
		t.Fatalf("resolved module = %v, %v; want chara_card_v2, true", got.Module, ok)
	}
}

type forcedAuthoritativeModule struct{ id string }

func (m forcedAuthoritativeModule) ID() string { return m.id }
func (m forcedAuthoritativeModule) Declaration() Declaration {
	return testReaderDeclaration(m.id, "character")
}
func (m forcedAuthoritativeModule) Claim(file probe.Inspection) (Claim, bool) {
	return Claim{
		payloadID: file.Payloads[0].ID,
		strength:  authoritative,
		formatID:  m.id,
	}, true
}
func (m forcedAuthoritativeModule) Parse(context.Context, probe.Inspection, Claim) (Parsed, error) {
	return Parsed{Format: m.id}, nil
}

func TestResolveRejectsAuthorityForADifferentDiscriminator(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(claimingModule{
		id: "chara_card_v3", spec: "chara_card_v2", authoritative: true,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, _, err := registry.Resolve(probedPayload("chara_card_v2", "chara"))
	if !errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("Resolve error = %v, want ErrInvalidClaim", err)
	}
}

func probedPayload(spec, locator string) probe.Inspection {
	return probe.Inspection{
		Container: probe.PNG,
		Payloads: []probe.Payload{{
			ID:      0,
			Locator: probe.Locator{Container: probe.PNG, Name: locator},
			Root: map[string]json.RawMessage{
				"spec": json.RawMessage(`"` + spec + `"`),
			},
		}},
	}
}

// containerModule stands for CharX: its payload names a standard rather than
// the module, because the standard has no discriminator of its own.
type containerModule struct{ claimingModule }

func (containerModule) OwnedSpecs() []string { return []string{"chara_card_v3"} }

func TestResolveAcceptsAuthorityForASpecTheModuleOwns(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(containerModule{claimingModule{
		id: "charx", spec: "chara_card_v3", authoritative: true,
	}}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, ok, err := registry.Resolve(probedPayload("chara_card_v3", "card.json"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok || got.Module.ID() != "charx" {
		t.Fatalf("resolved module = %v, %v; want charx, true", got.Module, ok)
	}
}

func TestResolveStillRejectsAuthorityForASpecNobodyOwns(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(containerModule{claimingModule{
		id: "charx", spec: "chara_card_v2", authoritative: true,
	}}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, _, err := registry.Resolve(probedPayload("chara_card_v2", "card.json"))
	if !errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("Resolve error = %v, want ErrInvalidClaim", err)
	}
}

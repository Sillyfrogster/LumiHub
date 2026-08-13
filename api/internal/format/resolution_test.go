package format

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

type claimingModule struct {
	id       string
	spec     string
	strength ClaimStrength
}

func (m claimingModule) ID() string { return m.id }
func (m claimingModule) Parse(context.Context, probe.Result, Claim) (Parsed, error) {
	return Parsed{Format: m.id}, nil
}
func (m claimingModule) Claim(file probe.Result) (Claim, bool) {
	for _, payload := range file.Payloads {
		if spec, ok := payload.String("spec"); ok && spec == m.spec {
			return Claim{PayloadID: payload.ID, Strength: m.strength}, true
		}
	}
	return Claim{}, false
}

func TestResolveReturnsNoModuleWhenNothingClaimsTheFile(t *testing.T) {
	registry := NewRegistry()

	_, ok, err := registry.Resolve(probe.Result{Container: probe.Unknown})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ok {
		t.Fatal("an unclaimed file resolved to a module; passthrough must be the absence of a claim")
	}
}

func TestResolvePrefersAnAuthoritativeClaimRegardlessOfRegistrationOrder(t *testing.T) {
	file := probedPayload("chara_card_v3", "ccv3")
	compatibility := claimingModule{id: "compatibility", spec: "chara_card_v3", strength: Compatibility}
	authoritative := claimingModule{id: "authoritative", spec: "chara_card_v3", strength: Authoritative}

	for _, modules := range [][]Module{
		{compatibility, authoritative},
		{authoritative, compatibility},
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
		if !ok || got.Module.ID() != "authoritative" {
			t.Fatalf("resolved module = %v, %v; want authoritative, true", got.Module, ok)
		}
	}
}

func TestResolveRejectsTwoAuthoritativeClaimsOnOnePayload(t *testing.T) {
	registry := NewRegistry()
	for _, id := range []string{"first", "second"} {
		if err := registry.Register(claimingModule{
			id: id, spec: "chara_card_v2", strength: Authoritative,
		}); err != nil {
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
		claimingModule{id: "ccv3", spec: "chara_card_v3", strength: Authoritative},
		claimingModule{id: "ccv2", spec: "chara_card_v2", strength: Authoritative},
	} {
		if err := registry.Register(module); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	got, ok, err := registry.Resolve(probedPayload("chara_card_v2", "ccv3"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !ok || got.Module.ID() != "ccv2" {
		t.Fatalf("resolved module = %v, %v; want ccv2, true", got.Module, ok)
	}
}

func probedPayload(spec, locator string) probe.Result {
	return probe.Result{
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

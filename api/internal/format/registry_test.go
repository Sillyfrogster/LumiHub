package format

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/probe"
)

type stubModule struct{ id string }

func testReaderDeclaration(id, kind string) Declaration {
	return Declaration{
		ID: id, Kind: kind, Direction: Direction{Read: true},
		Recognition: []Recognition{{
			Kind: RecognitionSignature, Containers: []probe.Container{probe.JSON},
			Required: map[string]ValueType{"payload": ValueBoolean},
		}},
		Limits:        ContentLimits{PayloadBytes: 1024, CollectionItems: 100, ItemBytes: 100},
		ConsumedKeys:  []string{"payload"},
		Preservation:  PreservationDeclaration{Body: "test"},
		TestedOrigins: []string{id},
	}
}

func (s stubModule) ID() string               { return s.id }
func (s stubModule) Declaration() Declaration { return testReaderDeclaration(s.id, "character") }
func (stubModule) Claim(probe.Inspection) (Claim, bool) {
	return Claim{}, false
}

type declaredDiscriminatorModule struct{}

func (declaredDiscriminatorModule) ID() string { return "theme_lumiverse" }
func (declaredDiscriminatorModule) Declaration() Declaration {
	declaration := testReaderDeclaration("theme_lumiverse", "theme")
	declaration.Recognition = []Recognition{{
		Kind: RecognitionDiscriminator, Containers: []probe.Container{probe.JSON},
		Path: []string{"format"}, Values: []string{"3"},
	}}
	return declaration
}
func (m declaredDiscriminatorModule) Claim(file probe.Inspection) (Claim, bool) {
	return ClaimByDeclaration(file, m.Declaration())
}
func (declaredDiscriminatorModule) Parse(context.Context, probe.Inspection, Claim) (Parsed, error) {
	return Parsed{}, nil
}

func TestDiscriminatorOutsideTheAcceptedSetNamesTheFormatAndValue(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(declaredDiscriminatorModule{}); err != nil {
		t.Fatalf("register module: %v", err)
	}
	file := probe.Inspection{Payloads: []probe.Payload{{
		ID: 0, Locator: probe.Locator{Container: probe.JSON},
		Root: map[string]json.RawMessage{"format": json.RawMessage(`4`)},
	}}}
	_, claimed, err := registry.Resolve(file)
	if claimed || !errors.Is(err, ErrUnsupportedFormat) ||
		!strings.Contains(err.Error(), "theme_lumiverse") || !strings.Contains(err.Error(), `"4"`) {
		t.Fatalf("Resolve = claimed %v, error %v; want named unsupported discriminator", claimed, err)
	}
}
func (s stubModule) Parse(context.Context, probe.Inspection, Claim) (Parsed, error) {
	return Parsed{Format: s.id}, nil
}

type declarationModule struct {
	stubModule
	declaration Declaration
}

func (m declarationModule) Declaration() Declaration { return m.declaration }

// registerSignatures puts one module behind each structural signature.
func registerSignatures(t *testing.T, signatures map[string]map[string]ValueType) *Registry {
	t.Helper()
	registry := NewRegistry()
	for id, required := range signatures {
		declaration := testReaderDeclaration(id, "character")
		declaration.Recognition[0].Required = required
		if err := registry.Register(declarationModule{
			stubModule: stubModule{id: id}, declaration: declaration,
		}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}
	return registry
}

func TestValidationRejectsASignatureThatShadowsAnother(t *testing.T) {
	registry := registerSignatures(t, map[string]map[string]ValueType{
		"looser":   {"alpha": ValueString},
		"stricter": {"alpha": ValueString, "beta": ValueBoolean},
	})
	if err := registry.ValidateDeclarations(); !errors.Is(err, ErrInvariant) {
		t.Fatalf("validation error = %v, want the shadowed signature rejected", err)
	}
}

// The formats that name themselves nowhere are told apart by the keys each one
// requires and the other does not, which is the whole of how a lorebook and a
// legacy character card separate.
func TestValidationAcceptsSignaturesThatEachRequireWhatTheOtherDoesNot(t *testing.T) {
	registry := registerSignatures(t, map[string]map[string]ValueType{
		"first":  {"alpha": ValueString},
		"second": {"beta": ValueBoolean},
	})
	if err := registry.ValidateDeclarations(); err != nil {
		t.Fatalf("validation error = %v, want signatures with disjoint keys accepted", err)
	}
}

func TestValidationRejectsTwoModulesDeclaringOneSignature(t *testing.T) {
	registry := registerSignatures(t, map[string]map[string]ValueType{
		"first":  {"alpha": ValueString},
		"second": {"alpha": ValueString},
	})
	if err := registry.ValidateDeclarations(); !errors.Is(err, ErrInvariant) {
		t.Fatalf("validation error = %v, want the repeated signature rejected", err)
	}
}

func TestRegisterRejectsDuplicateIDs(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(stubModule{id: "same"}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := r.Register(stubModule{id: "same"}); err == nil {
		t.Fatal("expected an error registering the same id twice")
	}
}

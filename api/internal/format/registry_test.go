package format

import (
	"context"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

type stubModule struct{ id string }

func (s stubModule) ID() string { return s.id }
func (stubModule) Claim(probe.Inspection) (Claim, bool) {
	return Claim{}, false
}
func (s stubModule) Parse(context.Context, probe.Inspection, Claim) (Parsed, error) {
	return Parsed{Format: s.id}, nil
}

type editableModule struct{ stubModule }

func (editableModule) ValidatePatch(Patch) error { return nil }

func TestRegisterRejectsDuplicateIDs(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(stubModule{id: "same"}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := r.Register(stubModule{id: "same"}); err == nil {
		t.Fatal("expected an error registering the same id twice")
	}
}

func TestCanEditReflectsTheOptionalInterface(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(stubModule{id: "plain"})
	_ = r.Register(editableModule{stubModule{id: "rich"}})

	if r.CanEdit("plain") {
		t.Error("CanEdit(plain) = true, want false")
	}
	if !r.CanEdit("rich") {
		t.Error("CanEdit(rich) = false, want true")
	}
}

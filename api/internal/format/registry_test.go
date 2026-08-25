package format

import (
	"context"
	"io"
	"testing"
)

type stubModule struct {
	id      string
	detects bool
}

func (s stubModule) ID() string                 { return s.id }
func (s stubModule) Detect(string, []byte) bool { return s.detects }
func (s stubModule) Parse(context.Context, io.Reader) (Parsed, error) {
	return Parsed{Format: s.id}, nil
}

type editableModule struct{ stubModule }

func (editableModule) Edit(context.Context, io.Reader, []byte) (io.Reader, error) {
	return nil, nil
}

func TestDetectFallsBackWhenNothingMatches(t *testing.T) {
	fallback := stubModule{id: "fallback"}
	r := NewRegistry(fallback)
	if err := r.Register(stubModule{id: "never", detects: false}); err != nil {
		t.Fatalf("register: %v", err)
	}

	got := r.Detect("mystery.bin", nil)
	if got.ID() != "fallback" {
		t.Fatalf("Detect returned %q, want fallback", got.ID())
	}
}

func TestDetectPrefersAMatchingModule(t *testing.T) {
	r := NewRegistry(stubModule{id: "fallback"})
	if err := r.Register(stubModule{id: "matcher", detects: true}); err != nil {
		t.Fatalf("register: %v", err)
	}

	got := r.Detect("card.png", nil)
	if got.ID() != "matcher" {
		t.Fatalf("Detect returned %q, want matcher", got.ID())
	}
}

func TestRegisterRejectsDuplicateIDs(t *testing.T) {
	r := NewRegistry(stubModule{id: "fallback"})
	if err := r.Register(stubModule{id: "same"}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := r.Register(stubModule{id: "same"}); err == nil {
		t.Fatal("expected an error registering the same id twice")
	}
}

func TestCanEditReflectsTheOptionalInterface(t *testing.T) {
	r := NewRegistry(stubModule{id: "fallback"})
	_ = r.Register(stubModule{id: "plain"})
	_ = r.Register(editableModule{stubModule{id: "rich"}})

	if r.CanEdit("plain") {
		t.Error("CanEdit(plain) = true, want false")
	}
	if !r.CanEdit("rich") {
		t.Error("CanEdit(rich) = false, want true")
	}
}

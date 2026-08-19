package asset

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The app a preset is built for seeds slot names and is stored nowhere. It is
// not identity, so a preset built by hand has no origin format and no record
// of which app was picked.
func TestTheAppAPresetIsBuiltForIsStoredNowhere(t *testing.T) {
	svc, pool := newTestService(t)
	owner := uuid.New()

	draft, err := svc.StartFromNothing(context.Background(), owner, "preset", "sillytavern")
	if err != nil {
		t.Fatalf("start a preset: %v", err)
	}

	var origin *string
	err = pool.QueryRow(context.Background(),
		`select origin_format from assets where id = $1`, draft).Scan(&origin)
	if err != nil {
		t.Fatalf("read the stored preset: %v", err)
	}
	if origin != nil {
		t.Errorf("origin_format = %q, want null for an asset built here", *origin)
	}

	// Nothing else on the row records the answer either.
	var stored string
	err = pool.QueryRow(context.Background(),
		`select to_jsonb(assets)::text from assets where id = $1`, draft).Scan(&stored)
	if err != nil {
		t.Fatalf("read the whole preset row: %v", err)
	}
	if strings.Contains(stored, "sillytavern") {
		t.Errorf("the stored preset carries the app that was answered: %s", stored)
	}
}

// Only the kinds whose settings have names an app gives them are asked, and an
// answer for any other kind is refused rather than quietly ignored.
func TestOnlyAPresetIsAskedWhichAppItIsFor(t *testing.T) {
	svc, _ := newTestService(t)
	owner := uuid.New()

	if _, err := svc.StartFromNothing(
		context.Background(), owner, "preset", "",
	); !errors.Is(err, ErrAppNotAnswered) {
		t.Errorf("a preset with no app = %v, want ErrAppNotAnswered", err)
	}
	if _, err := svc.StartFromNothing(
		context.Background(), owner, "preset", "koboldcpp",
	); !errors.Is(err, ErrAppNotAnswered) {
		t.Errorf("a preset for an app Illarin has no names for = %v, want ErrAppNotAnswered", err)
	}
	if _, err := svc.StartFromNothing(
		context.Background(), owner, "lorebook", "sillytavern",
	); !errors.Is(err, ErrAppNotAnswered) {
		t.Errorf("a lorebook answering an app question = %v, want ErrAppNotAnswered", err)
	}
	if !KindAsksForAnApp("preset") {
		t.Error("a preset is not asked which app it is for")
	}
	if KindAsksForAnApp("character") || KindAsksForAnApp("lorebook") {
		t.Error("a kind whose settings depend on no app is asked anyway")
	}
}

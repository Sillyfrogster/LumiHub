package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestOnlyTheOwnerPublishesADraft(t *testing.T) {
	svc, _ := newTestService(t)
	owner, somebodyElse := uuid.New(), uuid.New()
	draft, err := svc.StartFromNothing(context.Background(), owner, "character", "")
	if err != nil {
		t.Fatalf("start a draft: %v", err)
	}

	if _, err := svc.Publish(context.Background(), somebodyElse, draft); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another account publishing the draft = %v, want ErrNotFound", err)
	}

	page, err := svc.Detail(context.Background(), draft, &owner, ContentShown)
	if err != nil {
		t.Fatalf("read the draft: %v", err)
	}
	if page.Lifecycle != LifecycleDraft {
		t.Fatalf("lifecycle = %q, want a draft nobody else could publish", page.Lifecycle)
	}
}

func TestAWithheldAssetIsNotPublished(t *testing.T) {
	svc, pool := newTestService(t)
	owner := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'withheld.owner')`, owner); err != nil {
		t.Fatalf("insert the owner: %v", err)
	}
	draft, err := svc.StartFromNothing(context.Background(), owner, "character", "")
	if err != nil {
		t.Fatalf("start a draft: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		update assets
		   set withheld_at = now(), withheld_by = owner_id, withheld_reason = 'testing'
		 where id = $1
	`, draft); err != nil {
		t.Fatalf("withhold the draft: %v", err)
	}

	if _, err := svc.Publish(context.Background(), owner, draft); !errors.Is(err, ErrAssetFrozen) {
		t.Fatalf("publishing a withheld draft = %v, want ErrAssetFrozen", err)
	}
}

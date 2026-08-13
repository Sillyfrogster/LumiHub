package asset

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func createMadeAt(t *testing.T, svc *Service, name string, made time.Time) Asset {
	t.Helper()
	a, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: name + ".bin",
		File: bytes.NewReader([]byte(name)), Name: name, Discovery: "listed", CreatedAt: &made,
	})
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return a
}

func pageAll(t *testing.T, svc *Service, size int) []Asset {
	t.Helper()
	var seen []Asset
	var from *Cursor
	for range 20 {
		page, err := svc.List(context.Background(), ListFilter{Limit: size, Before: from})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(page) == 0 {
			return seen
		}
		seen = append(seen, page...)
		last := page[len(page)-1]
		from = &Cursor{MadeAt: last.CreatedAt, ID: last.ID}
	}
	t.Fatal("paging never reached the end")
	return nil
}

func names(assets []Asset) []string {
	out := make([]string, len(assets))
	for i, a := range assets {
		out[i] = a.Name
	}
	return out
}

func TestPagingSeesEveryAssetExactlyOnce(t *testing.T) {
	svc, _ := newTestService(t)
	base := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	for i, name := range []string{"oldest", "older", "middle", "newer", "newest"} {
		createMadeAt(t, svc, name, base.Add(time.Duration(i)*time.Hour))
	}

	got := names(pageAll(t, svc, 2))

	want := []string{"newest", "newer", "middle", "older", "oldest"}
	if len(got) != len(want) {
		t.Fatalf("paging saw %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paging saw %v, want %v", got, want)
		}
	}
}

func TestPagingIsUndisturbedByAnAssetPublishedMidway(t *testing.T) {
	svc, _ := newTestService(t)
	base := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	for i, name := range []string{"oldest", "older", "middle", "newer", "newest"} {
		createMadeAt(t, svc, name, base.Add(time.Duration(i)*time.Hour))
	}

	first, err := svc.List(context.Background(), ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page held %d assets, want 2", len(first))
	}

	createMadeAt(t, svc, "arrived-later", base.Add(9*time.Hour))

	last := first[len(first)-1]
	rest, err := svc.List(context.Background(), ListFilter{
		Limit:  10,
		Before: &Cursor{MadeAt: last.CreatedAt, ID: last.ID},
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}

	got := names(rest)
	want := []string{"middle", "older", "oldest"}
	if len(got) != len(want) {
		t.Fatalf("after the first page paging saw %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after the first page paging saw %v, want %v", got, want)
		}
	}
}

func TestPagingIsStableWhenMadeDatesAreIdentical(t *testing.T) {
	svc, _ := newTestService(t)
	shared := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		createMadeAt(t, svc, name, shared)
	}

	got := pageAll(t, svc, 2)

	if len(got) != 5 {
		t.Fatalf("paging saw %v, want all five", names(got))
	}
	seen := make(map[uuid.UUID]bool, len(got))
	for _, a := range got {
		if seen[a.ID] {
			t.Fatalf("paging saw %s twice, the id tie break is not holding the boundary", a.Name)
		}
		seen[a.ID] = true
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ID.String() <= got[i].ID.String() {
			t.Fatalf("assets sharing a date came back as %v, want them ordered by id", names(got))
		}
	}
}

func TestPagingPastTheEndReturnsNothing(t *testing.T) {
	svc, _ := newTestService(t)
	oldest := createMadeAt(t, svc, "oldest", time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC))

	got, err := svc.List(context.Background(), ListFilter{
		Limit:  10,
		Before: &Cursor{MadeAt: oldest.CreatedAt, ID: oldest.ID},
	})
	if err != nil {
		t.Fatalf("a page past the end must not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("a page past the end returned %v, want nothing", names(got))
	}
}

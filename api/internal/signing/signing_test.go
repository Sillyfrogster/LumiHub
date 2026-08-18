package signing

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func parse(t *testing.T, signed string) (string, string, string) {
	t.Helper()
	path, query, found := strings.Cut(signed, "?")
	if !found {
		t.Fatalf("signed URL %q carries no query", signed)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("parse the signed query: %v", err)
	}
	return path, values.Get(ExpiresParam), values.Get(SignatureParam)
}

func TestASignedURLIsGoodUntilItRunsOut(t *testing.T) {
	key := NewKey()
	now := time.Unix(1_700_000_000, 0)

	path, expires, signature := parse(t, key.Sign("/media/abc/detail/1", now))

	if !key.Valid(path, expires, signature, now) {
		t.Error("a fresh signature is refused")
	}
	if !key.Valid(path, expires, signature, now.Add(Life-time.Second)) {
		t.Error("a signature is refused before it runs out")
	}
	if key.Valid(path, expires, signature, now.Add(Life+time.Second)) {
		t.Error("a signature outlives its deadline")
	}
}

func TestASignatureIsGoodForOnePathAndOneKey(t *testing.T) {
	key := NewKey()
	other := NewKey()
	now := time.Unix(1_700_000_000, 0)

	path, expires, signature := parse(t, key.Sign("/media/abc/detail/1", now))

	if key.Valid("/media/abc/og/1", expires, signature, now) {
		t.Error("a signature carries over to another image")
	}
	if other.Valid(path, expires, signature, now) {
		t.Error("another key accepts this signature")
	}
	if key.Valid(path, "9999999999", signature, now) {
		t.Error("a rewritten deadline extends a signature")
	}
	if key.Valid(path, expires, "", now) {
		t.Error("an empty signature passes")
	}
}

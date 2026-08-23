package migration

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// LegacyPath is one v1 public address and the asset that answers for it.
type LegacyPath struct {
	Path    string
	AssetID uuid.UUID
}

// legacyCandidate is one asset's claim on a v1 address, with the creation time that settles a collision.
type legacyCandidate struct {
	AssetID   uuid.UUID
	Author    string
	Handle    string
	Name      string
	CreatedAt time.Time
}

// address is the credited author and the name slugified, falling back to the owner's handle where v1 rendered that instead.
func (candidate legacyCandidate) address() string {
	head := legacySlug(candidate.Author)
	if head == "" {
		head = legacySlug(candidate.Handle)
	}
	tail := legacySlug(candidate.Name)
	if head == "" || tail == "" {
		return ""
	}
	return head + "/" + tail
}

// legacySlug lowercases and joins on anything that is not an ASCII letter or digit, as v1 did.
func legacySlug(text string) string {
	var built strings.Builder
	separated := false
	for _, letter := range strings.ToLower(text) {
		if letter < unicode.MaxASCII && (unicode.IsLetter(letter) || unicode.IsDigit(letter)) {
			if separated {
				built.WriteByte('-')
			}
			separated = false
			built.WriteRune(letter)
			continue
		}
		separated = built.Len() > 0
	}
	return built.String()
}

// resolveLegacyPaths gives each address to the asset that held it first and returns the later claims for the ledger.
func resolveLegacyPaths(candidates []legacyCandidate) ([]LegacyPath, []legacyCandidate) {
	ordered := append([]legacyCandidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
		}
		return ordered[i].AssetID.String() < ordered[j].AssetID.String()
	})
	held := make(map[string]struct{}, len(ordered))
	paths := make([]LegacyPath, 0, len(ordered))
	displaced := make([]legacyCandidate, 0)
	for _, candidate := range ordered {
		address := candidate.address()
		if address == "" {
			continue
		}
		if _, taken := held[address]; taken {
			displaced = append(displaced, candidate)
			continue
		}
		held[address] = struct{}{}
		paths = append(paths, LegacyPath{Path: address, AssetID: candidate.AssetID})
	}
	return paths, displaced
}

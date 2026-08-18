package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MaxNameRunes is as long as a name may be. It is a boundary on stored text
// rather than a judgement about what a good name is.
const MaxNameRunes = 200

var (
	// ErrNameTooLong is a name past MaxNameRunes.
	ErrNameTooLong = errors.New("the name is too long")
	// ErrRatingUnanswerable is an attempt to unanswer the adult content
	// question on a published asset, which readers have already been told.
	ErrRatingUnanswerable = errors.New("a published asset needs an adult content answer")
)

// Identity is the header an asset carries above its blocks. A blank name and
// an unanswered adult content question are both ordinary states for a draft.
type Identity struct {
	OwnerID uuid.UUID
	AssetID uuid.UUID
	Name    string
	// IsNSFW is nil where the creator has not answered. Only a draft may be
	// unanswered.
	IsNSFW *bool
}

// SetIdentity saves the header fields the publish floor reads. An edit is live
// when it saves and there is no republish step.
func (s *Service) SetIdentity(ctx context.Context, in Identity) error {
	name := strings.TrimSpace(in.Name)
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return fmt.Errorf("%w: %d characters is past %d", ErrNameTooLong,
			utf8.RuneCountInString(name), MaxNameRunes)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var lifecycle string
	var withheld bool
	err = tx.QueryRow(ctx, `
		select lifecycle, withheld_at is not null
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
		 for update
	`, in.AssetID, in.OwnerID).Scan(&lifecycle, &withheld)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read asset header: %w", err)
	}
	if withheld {
		return ErrAssetFrozen
	}
	if in.IsNSFW == nil && Lifecycle(lifecycle) != LifecycleDraft {
		return ErrRatingUnanswerable
	}
	fingerprint, err := s.contentFingerprint(ctx, tx, in.AssetID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		update assets set name = $2, is_nsfw = $3, updated_at = now()
		 where id = $1
	`, in.AssetID, name, in.IsNSFW); err != nil {
		return fmt.Errorf("save asset header: %w", err)
	}
	// A name is part of a file and the adult content answer is part of a page,
	// so the counter follows the name alone.
	if err := s.moveContentGeneration(ctx, tx, in.AssetID, fingerprint); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

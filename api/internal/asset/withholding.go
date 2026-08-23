package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/google/uuid"
)

var ErrInvalidWithholdReason = errors.New("invalid withhold reason")

func (s *Service) Withhold(ctx context.Context, id, actorID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ErrInvalidWithholdReason
	}
	changed, err := db.New(s.pool).WithholdAsset(ctx, db.WithholdAssetParams{
		ID:             uuidToPgtype(id),
		WithheldBy:     uuidToPgtype(actorID),
		WithheldReason: textToNullable(&reason),
	})
	if err != nil {
		return fmt.Errorf("withhold asset: %w", err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ClearWithhold(ctx context.Context, id uuid.UUID) error {
	changed, err := db.New(s.pool).ClearAssetWithhold(ctx, uuidToPgtype(id))
	if err != nil {
		return fmt.Errorf("clear asset withhold: %w", err)
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

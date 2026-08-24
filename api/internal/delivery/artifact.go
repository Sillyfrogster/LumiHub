package delivery

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrArtifactNotFound is a delivery artifact nobody may fetch, whatever the reason.
var ErrArtifactNotFound = errors.New("no such delivery artifact")

// Artifact resolves a signed export address, bounded by the signature and by the delivery's lease.
func (s *Service) Artifact(
	ctx context.Context,
	deliveryID uuid.UUID,
	expires string,
	signature string,
) (uuid.UUID, string, error) {
	path := deliveryPathStart + deliveryID.String() + "/export"
	if !s.catalog.ValidSignature(path, expires, signature) {
		return uuid.Nil, "", ErrArtifactNotFound
	}
	row, err := db.New(s.pool).DeliveryForArtifact(ctx, uuidValue(deliveryID))
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, "", ErrArtifactNotFound
	}
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("read a delivery artifact: %w", err)
	}
	return uuid.UUID(row.AssetID.Bytes), row.ChosenTarget.String, nil
}

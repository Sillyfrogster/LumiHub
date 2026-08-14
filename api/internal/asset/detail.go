package asset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	mediaproc "github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DetailTag is one of the creator's tags twice over: as they wrote it, and as
// browse matches it.
type DetailTag struct {
	Label string
	Value string
}

// DetailImage is one of an asset's images at the two sizes the page uses.
type DetailImage struct {
	ID        uuid.UUID
	Role      MediaRole
	DetailURL string
	ThumbURL  string
	Width     int
	Height    int
}

// Detail is everything an asset's own page shows. It holds no total of any
// kind, because nothing on that page displays one.
type Detail struct {
	ID        uuid.UUID
	Kind      string
	Name      string
	Blurb     string
	Tags      []DetailTag
	Creator   string
	IsNSFW    bool
	Discovery Discovery
	CreatedAt time.Time
	// Media is cover first.
	Media []DetailImage
	// Preview is the composed social preview a link unfurler fetches.
	Preview  *string
	Withhold *Withhold
}

type Withhold struct {
	Reason string
	Actor  string
	At     time.Time
}

// Detail returns one asset by id. Unlisted assets answer normally. A withheld
// asset answers only to its owner, and a deleted asset answers to nobody.
func (s *Service) Detail(
	ctx context.Context,
	id uuid.UUID,
	viewerID *uuid.UUID,
	visibility ContentVisibility,
) (Detail, error) {
	queries := db.New(s.pool)
	row, err := queries.AssetPage(ctx, db.AssetPageParams{
		ID: uuidToPgtype(id), ViewerID: uuidToNullable(viewerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, fmt.Errorf("read asset page: %w", err)
	}
	found := Detail{
		ID:        uuidFromPgtype(row.ID),
		Kind:      row.Kind,
		Name:      row.Name,
		Blurb:     row.Blurb,
		Tags:      detailTags(row.Tags),
		Creator:   row.Creator,
		IsNSFW:    row.IsNsfw,
		Discovery: Discovery(row.Discovery),
		CreatedAt: timeFromPgtype(row.CreatedAt),
		Media:     []DetailImage{},
	}
	if row.WithheldAt.Valid {
		found.Withhold = &Withhold{
			Reason: row.WithheldReason.String,
			Actor:  row.WithheldBy.String,
			At:     row.WithheldAt.Time,
		}
	}

	images, err := queries.AssetPageMedia(ctx, uuidToPgtype(id))
	if err != nil {
		return Detail{}, fmt.Errorf("read asset page media: %w", err)
	}
	blurred := found.IsNSFW && visibility != ContentShown
	for _, image := range images {
		mediaID := uuidFromPgtype(image.ID)
		found.Media = append(found.Media, DetailImage{
			ID:        mediaID,
			Role:      MediaRole(image.Role),
			DetailURL: variantURL(mediaID, "detail", blurred),
			ThumbURL:  variantURL(mediaID, "thumb", blurred),
			Width:     int(image.Width.Int32),
			Height:    int(image.Height.Int32),
		})
	}
	if len(found.Media) > 0 {
		// A link unfurler has no reader to ask, so a flagged preview is blurred.
		preview := variantURL(found.Media[0].ID, "og", found.IsNSFW)
		found.Preview = &preview
	}
	return found, nil
}

// detailTags pairs the creator's text with the form browse matches on.
func detailTags(tags []string) []DetailTag {
	out := make([]DetailTag, 0, len(tags))
	for _, tag := range tags {
		value := normalizeBrowseText(tag)
		if value == "" {
			continue
		}
		out = append(out, DetailTag{Label: tag, Value: value})
	}
	return out
}

func variantURL(mediaID uuid.UUID, variant string, blurred bool) string {
	if blurred {
		variant += "_blurred"
	}
	return fmt.Sprintf("/media/%s/%s/%d", mediaID, variant, mediaproc.DerivativeVersion)
}

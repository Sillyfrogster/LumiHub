package asset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
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
	IsCover   bool
	DetailURL string
	ThumbURL  string
	Width     int
	Height    int
}

// Detail is everything an asset's own page shows. It holds no total of any
// kind, because nothing on that page displays one.
type Detail struct {
	ID      uuid.UUID
	Kind    string
	Name    string
	Blurb   string
	Tags    []DetailTag
	Creator string
	// IsNSFW is nil while a draft has not been asked the question.
	IsNSFW    *bool
	Discovery Discovery
	Lifecycle Lifecycle
	// IsOwner says whether the reader owns the asset. The owner's page is the
	// reader's page with more on it, never a second page.
	IsOwner   bool
	CreatedAt time.Time
	// Blocks are the asset's content, in page order.
	Blocks []block.Block
	// Media puts the direct cover first, followed by the remaining roles.
	Media []DetailImage
	// Preview is the composed social preview a link unfurler fetches.
	Preview *string
	// Readiness is what publication waits on, and stands only while the owner
	// is reading their own draft.
	Readiness []ReadinessItem
	Withhold  *Withhold
}

type Withhold struct {
	Reason string
	Actor  string
	At     time.Time
}

// Detail returns one asset by id. Unlisted assets answer normally. A draft and
// a withheld asset answer only to their owner, and a deleted asset answers to
// nobody.
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
		IsNSFW:    boolFromPgtype(row.IsNsfw),
		Discovery: Discovery(row.Discovery),
		Lifecycle: Lifecycle(row.Lifecycle),
		IsOwner:   row.IsOwner,
		CreatedAt: timeFromPgtype(row.CreatedAt),
		Media:     []DetailImage{},
	}
	found.Blocks, err = readBlocks(ctx, s.pool, id)
	if err != nil {
		return Detail{}, err
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
	// Only a draft is unanswered, and only its owner can be looking at it.
	flagged := found.IsNSFW != nil && *found.IsNSFW
	blurred := flagged && visibility != ContentShown
	draft := found.Lifecycle == LifecycleDraft
	for _, image := range images {
		mediaID := uuidFromPgtype(image.ID)
		found.Media = append(found.Media, DetailImage{
			ID:        mediaID,
			Role:      MediaRole(image.Role),
			IsCover:   image.IsCover,
			DetailURL: s.variantURL(mediaID, "detail", blurred, draft),
			ThumbURL:  s.variantURL(mediaID, "thumb", blurred, draft),
			Width:     int(image.Width.Int32),
			Height:    int(image.Height.Int32),
		})
	}
	if draft {
		// A draft is not shareable, so nothing unfurls it and the readiness
		// list stands in the rail instead.
		if found.IsOwner {
			found.Readiness = readiness(found.Kind, found.Name, found.IsNSFW, found.Blocks)
		}
		return found, nil
	}
	for _, image := range found.Media {
		if image.IsCover {
			// A link unfurler has no reader to ask, so a flagged preview is blurred.
			preview := s.variantURL(image.ID, "og", flagged, false)
			found.Preview = &preview
			break
		}
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

// variantURL addresses one image at one size. A draft's images sit at the same
// address a published asset's do, signed and short-lived until it is
// published, so publishing re-keys nothing.
func (s *Service) variantURL(mediaID uuid.UUID, variant string, blurred, private bool) string {
	if blurred {
		variant += "_blurred"
	}
	path := fmt.Sprintf("/media/%s/%s/%d", mediaID, variant, mediaproc.DerivativeVersion)
	if !private {
		return path
	}
	return s.signer.Sign(path, s.now())
}

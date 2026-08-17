package asset

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	errIngestLeaseLost = errors.New("ingest lease lost")
	// errWrongKind marks a revision that reads as a different kind of thing
	// than the asset it would replace. Kind is immutable, so the asset and
	// its current revision are left exactly as they were.
	errWrongKind = errors.New("revision resolves to a different kind")
)

// RunIngestWorkers processes operations until ctx is cancelled.
func (s *Service) RunIngestWorkers(ctx context.Context, count int, report func(error)) {
	var workers sync.WaitGroup
	workers.Add(count)
	for range count {
		go func() {
			defer workers.Done()
			s.runIngestWorker(ctx, report)
		}()
	}
	workers.Wait()
}

func (s *Service) runIngestWorker(ctx context.Context, report func(error)) {
	for {
		processed, err := s.ProcessNextIngest(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			if report != nil {
				report(err)
			}
		}
		if err == nil && processed {
			continue
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type ingestJob struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	BlobID     uuid.UUID
	LeaseToken uuid.UUID
	Filename   string
	Kind       *string
	Name       *string
	Blurb      *string
	Tags       []string
	IsNSFW     *bool
	Discovery  Discovery
	ByteSize   int64
	Attempts   int
	// Target is the asset this file becomes a revision of. Nil means the
	// ingest is creating one.
	Target *revisionTarget
}

type revisionTarget struct {
	AssetID uuid.UUID
	Kind    string
}

type preparedIngest struct {
	Kind                string
	Format              string
	PassthroughPlatform *string
	Name                string
	Blurb               string
	Tags                []string
	IsNSFW              bool
	Discovery           Discovery
	Facets              []format.Facet
	Media               []preparedMedia
	CreatedAt           *time.Time
	MediaType           string
}

// ProcessNextIngest leases and processes one available operation.
func (s *Service) ProcessNextIngest(ctx context.Context) (bool, error) {
	if _, err := s.pool.Exec(ctx, `
		delete from ingest_operations
		 where status = 'needs_kind' and expires_at <= $1
	`, s.now()); err != nil {
		return false, fmt.Errorf("expire needs_kind ingests: %w", err)
	}
	job, ok, err := s.leaseNextIngest(ctx)
	if err != nil || !ok {
		return ok, err
	}

	inspected, err := probe.InspectWithLimits(
		ctx, s.store, job.BlobID, job.ByteSize, job.Filename, s.ingest.ProbeLimits,
	)
	if err != nil {
		if errors.Is(err, probe.ErrSafetyViolation) {
			return true, s.finishIngestFailure(ctx, job, format.FailureSafetyViolation)
		}
		if errors.Is(err, probe.ErrMalformedInput) {
			return true, s.finishIngestFailure(ctx, job, format.FailureMalformedInput)
		}
		return true, s.finishIngestFailure(ctx, job, format.FailureInternal)
	}
	resolution, claimed, err := s.reg.Resolve(inspected)
	if err != nil {
		if errors.Is(err, format.ErrUnsupportedFormat) {
			return true, s.finishIngestFailure(ctx, job, format.FailureUnsupportedFormat)
		}
		return true, s.finishIngestFailure(ctx, job, format.FailureInternal)
	}

	parsed := format.Parsed{Format: "unknown"}
	if claimed {
		parsed, err = resolution.Module.Parse(ctx, inspected, resolution.Claim)
		if err != nil {
			reason := format.FailureMalformedInput
			if classified, ok := format.FailureOf(err); ok {
				reason = classified
			}
			return true, s.finishIngestFailure(ctx, job, reason)
		}
	}

	prepared, needsKind, err := prepareIngest(job, parsed, claimed)
	if errors.Is(err, errWrongKind) {
		return true, s.finishIngestFailure(ctx, job, format.FailureWrongKind)
	}
	if err != nil {
		return true, s.finishIngestFailure(ctx, job, format.FailureInternal)
	}
	if needsKind {
		return true, s.pauseForKind(ctx, job)
	}
	prepared.MediaType = inspected.InlineMediaType()
	if prepared.MediaType == "" {
		prepared.MediaType = "application/octet-stream"
	}
	prepared.Media, err = s.prepareExtractedMedia(ctx, inspected, parsed.Media)
	if err != nil {
		return true, s.finishIngestFailure(ctx, job, mediaIngestFailure(err))
	}
	if err := s.finalizeIngest(ctx, job, prepared); err != nil {
		if errors.Is(err, errIngestLeaseLost) {
			return true, nil
		}
		return true, s.finishIngestFailure(ctx, job, format.FailureInternal)
	}
	return true, nil
}

func (s *Service) leaseNextIngest(ctx context.Context) (ingestJob, bool, error) {
	now := s.now()
	leaseToken := uuid.New()
	leaseExpires := now.Add(s.ingest.LeaseDuration)
	row := s.pool.QueryRow(ctx, `
		with candidate as (
			select id
			  from ingest_operations
			 where available_at <= $1
			   and (status = 'pending'
			        or (status = 'processing' and lease_expires_at <= $1))
			 order by available_at, created_at
			 for update skip locked
			 limit 1
		)
		update ingest_operations operation
		   set status = 'processing',
		       attempts = attempts + 1,
		       lease_token = $2,
		       lease_expires_at = $3,
		       updated_at = $1
		  from candidate
		 where operation.id = candidate.id
		returning operation.id, operation.owner_id, operation.blob_id,
		          operation.filename, operation.kind, operation.name, operation.blurb,
		          operation.tags, operation.is_nsfw, operation.discovery,
		          operation.attempts,
		          (select byte_size from blobs where id = operation.blob_id),
		          operation.target_asset_id,
		          (select kind from assets where id = operation.target_asset_id)
	`, now, leaseToken, leaseExpires)

	var job ingestJob
	var kind, name, blurb, targetKind pgtype.Text
	var isNSFW pgtype.Bool
	var targetAssetID pgtype.UUID
	err := row.Scan(
		&job.ID, &job.OwnerID, &job.BlobID, &job.Filename, &kind, &name, &blurb,
		&job.Tags, &isNSFW, &job.Discovery, &job.Attempts, &job.ByteSize,
		&targetAssetID, &targetKind,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ingestJob{}, false, nil
	}
	if err != nil {
		return ingestJob{}, false, fmt.Errorf("lease ingest: %w", err)
	}
	job.LeaseToken = leaseToken
	if targetAssetID.Valid {
		if !targetKind.Valid {
			return ingestJob{}, false, fmt.Errorf("ingest %s targets a missing asset", job.ID)
		}
		job.Target = &revisionTarget{
			AssetID: uuidFromPgtype(targetAssetID), Kind: targetKind.String,
		}
	}
	job.Kind = textToPointer(kind)
	job.Name = textToPointer(name)
	job.Blurb = textToPointer(blurb)
	if isNSFW.Valid {
		job.IsNSFW = &isNSFW.Bool
	}
	return job, true, nil
}

func prepareIngest(job ingestJob, parsed format.Parsed, claimed bool) (preparedIngest, bool, error) {
	kind := parsed.Kind
	platform := (*string)(nil)
	if claimed {
		if kind == "" {
			return preparedIngest{}, false, errors.New("claimed format did not declare a kind")
		}
		if parsed.PassthroughPlatform != nil {
			return preparedIngest{}, false, errors.New("claimed format returned a passthrough platform")
		}
		if job.Target != nil && kind != job.Target.Kind {
			return preparedIngest{}, false, errWrongKind
		}
	} else if job.Target != nil {
		// A revision LumiHub cannot read is still a revision of this asset,
		// so it takes the kind the asset already has.
		kind = job.Target.Kind
		if hint, ok := passthroughHint(job.Filename); ok {
			platform = &hint.platform
		}
	} else if job.Kind != nil {
		kind = *job.Kind
	} else {
		hint, ok := passthroughHint(job.Filename)
		if !ok {
			return preparedIngest{}, true, nil
		}
		kind = hint.kind
		platform = &hint.platform
	}

	name := parsed.Name
	if job.Name != nil {
		name = *job.Name
	}
	blurb := parsed.Blurb
	if job.Blurb != nil {
		blurb = *job.Blurb
	}
	tags := parsed.Tags
	if job.Tags != nil {
		tags = job.Tags
	}
	if tags == nil {
		tags = []string{}
	}
	isNSFW := false
	if parsed.IsNSFW != nil {
		isNSFW = *parsed.IsNSFW
	}
	if job.IsNSFW != nil {
		isNSFW = *job.IsNSFW
	}
	return preparedIngest{
		Kind: kind, Format: parsed.Format, PassthroughPlatform: platform,
		Name: name, Blurb: blurb, Tags: tags, IsNSFW: isNSFW,
		Discovery: job.Discovery, Facets: parsed.Facets, CreatedAt: parsed.CreatedAt,
	}, false, nil
}

// CompleteIngest supplies the two fields an unrecognised passthrough needs.
func (s *Service) CompleteIngest(
	ctx context.Context,
	ownerID, id uuid.UUID,
	kind, name string,
) (IngestOperation, error) {
	if !validKind(kind) || strings.TrimSpace(name) == "" {
		return IngestOperation{}, errors.New("kind and name are required")
	}
	now := s.now()
	result, err := s.pool.Exec(ctx, `
		update ingest_operations
		   set status = 'pending', kind = $3, name = $4, expires_at = null,
		       available_at = $5, updated_at = $5
		 where id = $1 and owner_id = $2 and status = 'needs_kind'
		   and expires_at > $5
	`, id, ownerID, kind, strings.TrimSpace(name), now)
	if err != nil {
		return IngestOperation{}, fmt.Errorf("complete ingest kind: %w", err)
	}
	if result.RowsAffected() == 0 {
		return IngestOperation{}, ErrIngestNotFound
	}
	return IngestOperation{ID: id, Status: IngestPending}, nil
}

func validKind(kind string) bool {
	switch kind {
	case "character", "lorebook", "preset", "theme":
		return true
	default:
		return false
	}
}

type passthroughExtensionHint struct {
	kind     string
	platform string
}

var passthroughExtensionHints = map[string]passthroughExtensionHint{
	".lumitheme": {kind: "theme", platform: "lumiverse"},
}

func passthroughHint(filename string) (passthroughExtensionHint, bool) {
	hint, ok := passthroughExtensionHints[strings.ToLower(filepath.Ext(filename))]
	return hint, ok
}

func (s *Service) pauseForKind(ctx context.Context, job ingestJob) error {
	_, err := s.pool.Exec(ctx, `
		update ingest_operations
		   set status = 'needs_kind', name = $3, lease_token = null,
		       lease_expires_at = null, expires_at = $4, updated_at = $5
		 where id = $1 and lease_token = $2 and status = 'processing'
	`, job.ID, job.LeaseToken, filenameStem(job.Filename), s.now().Add(s.ingest.NeedsKindTTL), s.now())
	if err != nil {
		return fmt.Errorf("pause ingest for kind: %w", err)
	}
	return nil
}

func filenameStem(filename string) string {
	base := filepath.Base(filename)
	extension := filepath.Ext(base)
	return strings.TrimSuffix(base, extension)
}

func (s *Service) finishIngestFailure(
	ctx context.Context,
	job ingestJob,
	reason format.FailureReason,
) error {
	if reason == format.FailureInternal && job.Attempts < s.ingest.MaxAttempts {
		return s.retryIngest(ctx, job)
	}
	return s.failIngest(ctx, job, string(reason))
}

func (s *Service) retryIngest(ctx context.Context, job ingestJob) error {
	delay := s.ingest.RetryBase
	for attempt := 1; attempt < job.Attempts; attempt++ {
		delay *= 2
	}
	now := s.now()
	result, err := s.pool.Exec(ctx, `
		update ingest_operations
		   set status = 'pending', available_at = $3, lease_token = null,
		       lease_expires_at = null, updated_at = $4
		 where id = $1 and lease_token = $2 and status = 'processing'
	`, job.ID, job.LeaseToken, now.Add(delay), now)
	if err != nil {
		return fmt.Errorf("retry ingest: %w", err)
	}
	if result.RowsAffected() == 0 {
		return errIngestLeaseLost
	}
	return nil
}

func (s *Service) failIngest(ctx context.Context, job ingestJob, reason string) error {
	now := s.now()
	result, err := s.pool.Exec(ctx, `
		update ingest_operations
		   set status = 'failed', failure_reason = $3, blob_id = null,
		       lease_token = null, lease_expires_at = null, updated_at = $4
		 where id = $1 and lease_token = $2 and status = 'processing'
	`, job.ID, job.LeaseToken, reason, now)
	if err != nil {
		return fmt.Errorf("fail ingest: %w", err)
	}
	if result.RowsAffected() == 0 {
		return errIngestLeaseLost
	}
	return nil
}

func (s *Service) finalizeIngest(ctx context.Context, job ingestJob, prepared preparedIngest) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ingest finalization: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := lockBlobDigest(ctx, tx, job.BlobID); errors.Is(err, pgx.ErrNoRows) {
		return errIngestLeaseLost
	} else if err != nil {
		return fmt.Errorf("lock ingest digest: %w", err)
	}

	var status IngestStatus
	var lease pgtype.UUID
	if err := tx.QueryRow(ctx, `
		select status, lease_token from ingest_operations where id = $1 for update
	`, job.ID).Scan(&status, &lease); err != nil {
		return fmt.Errorf("lock ingest finalization: %w", err)
	}
	if status == IngestSuccess {
		return nil
	}
	if status != IngestProcessing || !lease.Valid || uuidFromPgtype(lease) != job.LeaseToken {
		return errIngestLeaseLost
	}

	assetID, err := writeIngestResult(ctx, tx, job, prepared)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		update ingest_operations
		   set status = 'success', asset_id = $3, blob_id = null,
		       lease_token = null, lease_expires_at = null, updated_at = $4
		 where id = $1 and lease_token = $2 and status = 'processing'
	`, job.ID, job.LeaseToken, assetID, s.now())
	if err != nil {
		return fmt.Errorf("finish ingest operation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return errIngestLeaseLost
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ingest finalization: %w", err)
	}
	return nil
}

// writeIngestResult turns a finished parse into rows. Either it publishes a new
// catalog entry or it adds a revision to one that already exists, and both end
// with the asset pointing at the revision just written.
func writeIngestResult(
	ctx context.Context,
	tx pgx.Tx,
	job ingestJob,
	prepared preparedIngest,
) (uuid.UUID, error) {
	if job.Target != nil {
		return job.Target.AssetID, appendRevision(ctx, tx, job, prepared)
	}
	assetID := uuid.New()
	isNSFW := prepared.IsNSFW
	a := Asset{
		ID: assetID, Kind: prepared.Kind, Format: prepared.Format,
		PassthroughPlatform: prepared.PassthroughPlatform,
		Name:                prepared.Name, Blurb: prepared.Blurb, Tags: prepared.Tags,
		IsNSFW: &isNSFW, Discovery: prepared.Discovery,
		Lifecycle: LifecyclePublished,
	}
	if _, err := insertAsset(ctx, tx, a, job.OwnerID, prepared.CreatedAt); err != nil {
		return uuid.Nil, err
	}
	return assetID, writeRevision(ctx, tx, assetID, 1, job, prepared)
}

// appendRevision adds a set of source bytes to an asset that already exists.
// Catalog metadata is never re-seeded here: it was the creator's from the
// moment the asset was made.
func appendRevision(
	ctx context.Context,
	tx pgx.Tx,
	job ingestJob,
	prepared preparedIngest,
) error {
	var withheldAt pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		select withheld_at
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
		 for update
	`, job.Target.AssetID, job.OwnerID).Scan(&withheldAt)
	if err != nil {
		return fmt.Errorf("lock revision target: %w", err)
	}
	if withheldAt.Valid {
		return fmt.Errorf("asset %s is frozen", job.Target.AssetID)
	}
	var next int
	if err := tx.QueryRow(ctx, `
		select coalesce(max(revision), 0) + 1 from asset_revisions where asset_id = $1
	`, job.Target.AssetID).Scan(&next); err != nil {
		return fmt.Errorf("number the new revision: %w", err)
	}
	return writeRevision(ctx, tx, job.Target.AssetID, next, job, prepared)
}

func writeRevision(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	number int,
	job ingestJob,
	prepared preparedIngest,
) error {
	revisionID := uuid.New()
	if err := insertRevision(ctx, tx, revisionID, assetID, revisionRow{
		Revision: number, BlobID: job.BlobID, MediaType: prepared.MediaType,
		Format: prepared.Format, PassthroughPlatform: prepared.PassthroughPlatform,
	}); err != nil {
		return err
	}
	if err := insertFacets(ctx, tx, revisionID, prepared.Facets); err != nil {
		return err
	}
	if err := supersedeExtractedMedia(ctx, tx, assetID); err != nil {
		return err
	}
	if err := insertAssetMedia(ctx, tx, assetID, prepared.Media); err != nil {
		return err
	}
	if err := setCurrentRevision(ctx, tx, assetID, revisionID); err != nil {
		return err
	}
	if coverID := avatarMedia(prepared.Media); coverID != nil {
		return setCoverMedia(ctx, tx, assetID, coverID)
	}
	return clearSupersededCover(ctx, tx, assetID)
}

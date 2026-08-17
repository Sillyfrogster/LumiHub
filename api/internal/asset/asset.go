package asset

import (
	"io"
	"time"

	"github.com/google/uuid"
)

// Lifecycle is an asset's own axis, separate from discovery state. A draft is
// owner-only, and discovery applies to a published asset alone.
type Lifecycle string

const (
	LifecycleDraft     Lifecycle = "draft"
	LifecyclePublished Lifecycle = "published"
)

type Discovery string

const (
	DiscoveryListed   Discovery = "listed"
	DiscoveryUnlisted Discovery = "unlisted"
)

func (d Discovery) Valid() bool {
	return d == DiscoveryListed || d == DiscoveryUnlisted
}

// Asset is a catalog entry. Format specific content lives in the stored
// revision, not here.
type Asset struct {
	ID                  uuid.UUID
	Kind                string
	Format              string
	PassthroughPlatform *string
	Name                string
	Blurb               string
	Tags                []string
	// IsNSFW is nil while a draft has not been asked the adult content
	// question, so nothing answers it for the creator.
	IsNSFW            *bool
	Discovery         Discovery
	Lifecycle         Lifecycle
	CurrentRevisionID uuid.UUID
	// CreatedAt is when the asset was made, not when we got it.
	CreatedAt time.Time
}

// CreateInput carries everything needed to publish a new asset.
type CreateInput struct {
	OwnerID  uuid.UUID
	Kind     string
	Filename string
	// File is read once, straight through to storage.
	File      io.Reader
	Name      string
	Blurb     string
	Tags      []string
	IsNSFW    bool
	Discovery Discovery
	// CreatedAt is a made date from the caller. Nil falls back to the file,
	// then to now.
	CreatedAt *time.Time
}

// IngestInput is the upload and any catalog fields the creator confirmed.
// Nil catalog fields are seeded from the parse.
type IngestInput struct {
	OwnerID   uuid.UUID
	Filename  string
	File      io.Reader
	Name      *string
	Blurb     *string
	Tags      *[]string
	IsNSFW    *bool
	Discovery Discovery
}

// RevisionInput is a new set of source bytes for an asset that already exists.
// Nothing in the catalog entry comes from it; a revision is the file only.
type RevisionInput struct {
	OwnerID  uuid.UUID
	AssetID  uuid.UUID
	Filename string
	File     io.Reader
}

type IngestStatus string

const (
	IngestPending    IngestStatus = "pending"
	IngestProcessing IngestStatus = "processing"
	IngestNeedsKind  IngestStatus = "needs_kind"
	IngestFailed     IngestStatus = "failed"
	IngestSuccess    IngestStatus = "success"
)

// IngestOperation is the durable progress of one accepted upload.
type IngestOperation struct {
	ID        uuid.UUID
	Status    IngestStatus
	NeedsKind *NeedsKind
	Failure   *IngestFailure
	Asset     *Asset
}

type NeedsKind struct {
	Kind *string
	Name string
}

type IngestFailure struct {
	Reason  string
	Message string
}

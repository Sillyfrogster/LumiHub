// Package delivery queues an asset for a linked instance and mirrors what it installed.
package delivery

import (
	"errors"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/google/uuid"
)

// State is where one queued delivery has got to.
type State string

const (
	StateQueued   State = "queued"
	StateReleased State = "released"
	StateFailed   State = "failed"
)

// Reason says why a delivery stopped, and stands only on a failed one.
type Reason string

const (
	ReasonWithdrawn   Reason = "withdrawn"
	ReasonUnsupported Reason = "unsupported"
	ReasonAbandoned   Reason = "abandoned"
)

var (
	ErrInstanceNotFound  = errors.New("no live instance of yours has that id")
	ErrMissingScope      = errors.New("that instance was not granted the scope this needs")
	ErrAssetNotFound     = errors.New("no such asset")
	ErrAssetNotSendable  = errors.New("only a published asset can be sent to an instance")
	ErrNoTarget          = errors.New("that instance accepts no format this asset can be written in")
	ErrQueueFull         = errors.New("that instance already has as many waiting deliveries as it may hold")
	ErrDeliveryNotFound  = errors.New("no waiting delivery of yours has that id")
	ErrTooManyRequests   = errors.New("too many requests")
	ErrTooManyCollectors = errors.New("too many instances are waiting for work at once")
	ErrLibraryTooLarge   = errors.New("the library report is larger than one request may carry")
	ErrLibraryReport     = errors.New("the library report is not valid")
	ErrAcknowledgement   = errors.New("the acknowledgement list is not valid")
)

// Delivery is one queued send, as the asset page shows it.
type Delivery struct {
	ID         uuid.UUID
	InstanceID uuid.UUID
	AssetID    uuid.UUID
	State      State
	Reason     Reason
	QueuedAt   time.Time
	ExpiresAt  time.Time
}

// Work is one released delivery naming what to fetch, never the bytes, so a large file is retryable.
type Work struct {
	ID                uuid.UUID
	AssetID           uuid.UUID
	ContentGeneration int
	Kind              string
	Name              string
	Format            string
	Label             string
	QueuedAt          time.Time
	LeaseExpiresAt    time.Time
	Artifacts         []Artifact
}

// Artifact is one file behind one short-lived signed URL.
type Artifact struct {
	Kind    string
	URL     string
	MediaID *uuid.UUID
	Role    string
	IsCover bool
}

// Artifact kinds are the asset written in the chosen format and the pictures beside it.
const (
	ArtifactExport  = "export"
	ArtifactPicture = "picture"
)

// InstanceState is one of a creator's instances as it stands for one asset.
type InstanceState struct {
	InstanceID          uuid.UUID
	ApplicationName     string
	InstanceName        string
	LastSeenAt          *time.Time
	CanReceive          bool
	ReportsLibrary      bool
	Delivery            *Delivery
	InstalledGeneration *int
	UpdateAvailable     bool
}

// AssetInstances is every instance one asset could be sent to, and what each already has.
type AssetInstances struct {
	ContentGeneration int
	Items             []InstanceState
}

// LibraryCounts is how much of one instance's mirror is behind the catalog.
type LibraryCounts struct {
	Installed        int
	UpdatesAvailable int
}

// LibraryReport is what an instance says it has installed.
type LibraryReport struct {
	Snapshot bool
	Entries  []LibraryEntry
	Removed  []uuid.UUID
}

// LibraryEntry is one installed asset, and no generation means current rather than stale (ADR-0023).
type LibraryEntry struct {
	AssetID           uuid.UUID
	ContentGeneration *int
}

// LibraryResult counts what a report changed without naming any asset back.
type LibraryResult struct {
	Accepted int
	Removed  int
	Ignored  int
}

// chooseTarget takes the first accepted format Illarin offers, so a declared but unoffered one selects nothing.
func chooseTarget(accepted []string, offered []asset.DeliveryTarget, hasOriginal bool) (string, string, bool) {
	byID := make(map[string]asset.DeliveryTarget, len(offered))
	for _, target := range offered {
		byID[target.Format] = target
	}
	for _, wanted := range accepted {
		if target, offers := byID[wanted]; offers {
			return target.Format, target.Label, true
		}
		if wanted == asset.RawDownloadTarget && hasOriginal {
			return asset.RawDownloadTarget, rawLabel, true
		}
	}
	if hasOriginal {
		return asset.RawDownloadTarget, rawLabel, true
	}
	return "", "", false
}

const rawLabel = "The creator's own file"

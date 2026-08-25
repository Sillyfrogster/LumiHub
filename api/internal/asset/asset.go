package asset

import (
	"io"
	"time"

	"github.com/google/uuid"
)

// Asset is a catalog entry. Format specific content lives in the stored
// revision, not here.
type Asset struct {
	ID                uuid.UUID
	Kind              string
	Platform          *string
	Format            string
	FormatVersion     string
	Name              string
	Description       string
	Tags              []string
	IsNSFW            bool
	Publication       string
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
	File        io.Reader
	Name        string
	Description string
	Tags        []string
	IsNSFW      bool
	Publication string
	// CreatedAt is a made date from the caller. Nil falls back to the file,
	// then to now.
	CreatedAt *time.Time
}

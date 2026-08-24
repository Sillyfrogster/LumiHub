package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/google/uuid"
)

var (
	ErrBlobNotFound       = errors.New("blob not found")
	ErrTombstoned         = errors.New("blob digest is tombstoned")
	ErrInvalidRange       = errors.New("invalid blob range")
	ErrDerivativeNotFound = errors.New("derivative not found")
	ErrInsufficientSpace  = errors.New("storage reserve would be crossed")
)

// Capacity protects room for the largest canonical write while keeping a
// free-space reserve.
type Capacity struct {
	FreeSpaceReserveBytes int64
	MaximumBlobWriteBytes int64
}

// StoredBlob identifies one distinct byte sequence.
type StoredBlob struct {
	ID       uuid.UUID
	Digest   [sha256.Size]byte
	ByteSize int64
}

// DerivativeID names one rendering of a source blob.
type DerivativeID struct {
	SourceDigest [sha256.Size]byte
	Variant      string
	Version      uint32
}

// Store keeps canonical bytes and records their content address.
type Store interface {
	Put(ctx context.Context, r io.Reader) (StoredBlob, error)
	RecordOrphans(ctx context.Context) (int, error)
	Open(ctx context.Context, id uuid.UUID) (io.ReadCloser, error)
	ReadRange(ctx context.Context, id uuid.UUID, offset, length int64) (io.ReadCloser, error)
	InternalRedirect(ctx context.Context, id uuid.UUID) (string, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteDerivatives(ctx context.Context, digest [sha256.Size]byte) error
	PutDerivative(ctx context.Context, id DerivativeID, body []byte) error
	OpenDerivative(ctx context.Context, id DerivativeID) (io.ReadCloser, error)
	InternalDerivativeRedirect(ctx context.Context, id DerivativeID) (string, error)
	ClearDerivatives(ctx context.Context) error
}

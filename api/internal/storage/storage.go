package storage

import (
	"context"
	"io"
)

/** Where original uploads and derived files are kept */
type Blob interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

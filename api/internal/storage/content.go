package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contentStore struct {
	queries *db.Queries
	root    string
}

// NewStore opens a content-addressed store rooted at root.
func NewStore(pool *pgxpool.Pool, root string) (Store, error) {
	for _, directory := range []string{filepath.Join(root, "blobs"), filepath.Join(root, "derivatives")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create storage directory: %w", err)
		}
	}
	return &contentStore{queries: db.New(pool), root: root}, nil
}

func (s *contentStore) Put(ctx context.Context, r io.Reader) (StoredBlob, error) {
	temporary, err := os.CreateTemp(filepath.Join(s.root, "blobs"), ".incoming-*")
	if err != nil {
		return StoredBlob{}, fmt.Errorf("create temporary blob: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	hash := sha256.New()
	byteSize, copyErr := io.Copy(io.MultiWriter(temporary, hash), r)
	closeErr := temporary.Close()
	if copyErr != nil {
		return StoredBlob{}, fmt.Errorf("store blob: %w", copyErr)
	}
	if closeErr != nil {
		return StoredBlob{}, fmt.Errorf("close blob: %w", closeErr)
	}

	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	key := blobKey(digest)
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return StoredBlob{}, fmt.Errorf("create blob directory: %w", err)
	}
	if err := installFile(temporaryName, path); err != nil {
		return StoredBlob{}, fmt.Errorf("install blob: %w", err)
	}

	row, err := s.queries.UpsertBlob(ctx, db.UpsertBlobParams{
		ID:         pgUUID(uuid.New()),
		Sha256:     digest[:],
		ByteSize:   byteSize,
		StorageKey: key,
	})
	if err != nil {
		return StoredBlob{}, fmt.Errorf("record blob: %w", err)
	}
	stored := StoredBlob{ID: uuid.UUID(row.ID.Bytes), ByteSize: row.ByteSize}
	copy(stored.Digest[:], row.Sha256)
	if row.StorageKey != key {
		_ = os.Remove(path)
	}
	return stored, nil
}

func (s *contentStore) Open(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	location, err := s.blobLocation(ctx, id)
	if err != nil {
		return nil, err
	}
	path, err := s.path(location.StorageKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open blob: %w", err)
	}
	return file, nil
}

func (s *contentStore) ReadRange(ctx context.Context, id uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	location, err := s.blobLocation(ctx, id)
	if err != nil {
		return nil, err
	}
	if offset < 0 || length <= 0 || offset > location.ByteSize || length > location.ByteSize-offset {
		return nil, fmt.Errorf("read bytes %d through %d from %d-byte blob: %w",
			offset, offset+length, location.ByteSize, ErrInvalidRange)
	}

	path, err := s.path(location.StorageKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open blob: %w", err)
	}
	return &sectionReadCloser{
		Reader: io.NewSectionReader(file, offset, length),
		Closer: file,
	}, nil
}

func (s *contentStore) Delete(ctx context.Context, id uuid.UUID) error {
	key, err := s.queries.DeleteBlob(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBlobNotFound
	}
	if err != nil {
		return fmt.Errorf("delete blob row: %w", err)
	}
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete blob bytes: %w", err)
	}
	return nil
}

func (s *contentStore) blobLocation(ctx context.Context, id uuid.UUID) (db.BlobLocationRow, error) {
	location, err := s.queries.BlobLocation(ctx, pgUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.BlobLocationRow{}, ErrBlobNotFound
	}
	if err != nil {
		return db.BlobLocationRow{}, fmt.Errorf("find blob: %w", err)
	}
	return location, nil
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func (s *contentStore) PutDerivative(ctx context.Context, id DerivativeID, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.derivativePath(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create derivative directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".incoming-*")
	if err != nil {
		return fmt.Errorf("create temporary derivative: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	_, copyErr := io.Copy(temporary, r)
	closeErr := temporary.Close()
	if copyErr != nil {
		return fmt.Errorf("store derivative: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close derivative: %w", closeErr)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("install derivative: %w", err)
	}
	return nil
}

func (s *contentStore) OpenDerivative(ctx context.Context, id DerivativeID) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.derivativePath(id)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open derivative: %w", err)
	}
	return file, nil
}

func (s *contentStore) ClearDerivatives(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := filepath.Join(s.root, "derivatives")
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clear derivatives: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("recreate derivative directory: %w", err)
	}
	return nil
}

func (s *contentStore) derivativePath(id DerivativeID) (string, error) {
	if id.Variant == "" {
		return "", fmt.Errorf("derivative variant is empty")
	}
	source := hex.EncodeToString(id.SourceDigest[:])
	variant := sha256.Sum256([]byte(id.Variant))
	return filepath.Join(
		s.root,
		"derivatives",
		source[:2],
		source,
		hex.EncodeToString(variant[:]),
		strconv.FormatUint(uint64(id.Version), 10),
	), nil
}

func (s *contentStore) path(key string) (string, error) {
	if !filepath.IsLocal(key) {
		return "", fmt.Errorf("invalid storage key")
	}
	return filepath.Join(s.root, filepath.FromSlash(key)), nil
}

type sectionReadCloser struct {
	io.Reader
	io.Closer
}

func blobKey(digest [sha256.Size]byte) string {
	encoded := hex.EncodeToString(digest[:])
	return filepath.ToSlash(filepath.Join("blobs", encoded[:2], encoded))
}

func installFile(from, to string) error {
	err := os.Link(from, to)
	if err == nil || os.IsExist(err) {
		return nil
	}
	return err
}

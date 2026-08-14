package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contentStore struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	root    string
}

const (
	internalBlobPrefix       = "/_lumihub/blobs/"
	internalDerivativePrefix = "/_lumihub/derivatives/"
)

// NewStore opens a content-addressed store rooted at root.
func NewStore(pool *pgxpool.Pool, root string) (Store, error) {
	for _, directory := range []string{filepath.Join(root, "blobs"), filepath.Join(root, "derivatives")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create storage directory: %w", err)
		}
	}
	return &contentStore{queries: db.New(pool), pool: pool, root: root}, nil
}

func (s *contentStore) Put(ctx context.Context, r io.Reader) (StoredBlob, error) {
	hash := sha256.New()
	temporaryName, byteSize, err := stage(filepath.Join(s.root, "blobs"), r, hash)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("stage blob: %w", err)
	}
	defer os.Remove(temporaryName)

	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	key := blobKey(digest)
	path := filepath.Join(s.root, filepath.FromSlash(key))

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return StoredBlob{}, fmt.Errorf("begin blob write: %w", err)
	}
	defer tx.Rollback(ctx)
	var locked int
	if err := tx.QueryRow(ctx, `
		select 1 from pg_advisory_xact_lock(
		    hashtextextended('lumihub-blob:' || encode($1::bytea, 'hex'), 0)
		)
	`, digest[:]).Scan(&locked); err != nil {
		return StoredBlob{}, fmt.Errorf("lock blob digest: %w", err)
	}
	var tombstoned bool
	if err := tx.QueryRow(ctx,
		`select exists (select 1 from blob_tombstones where sha256 = $1)`, digest[:],
	).Scan(&tombstoned); err != nil {
		return StoredBlob{}, fmt.Errorf("check blob tombstone: %w", err)
	}
	if tombstoned {
		return StoredBlob{}, ErrTombstoned
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return StoredBlob{}, fmt.Errorf("create blob directory: %w", err)
	}
	if err := installFile(temporaryName, path); err != nil {
		return StoredBlob{}, fmt.Errorf("install blob: %w", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return StoredBlob{}, fmt.Errorf("make blob readable by the byte server: %w", err)
	}

	row, err := db.New(tx).UpsertBlob(ctx, db.UpsertBlobParams{
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
	if _, err := tx.Exec(ctx, `delete from blob_sweep_marks where blob_id = $1`, row.ID); err != nil {
		return StoredBlob{}, fmt.Errorf("clear blob sweep mark: %w", err)
	}
	if row.StorageKey != key {
		_ = os.Remove(path)
	}
	if err := tx.Commit(ctx); err != nil {
		return StoredBlob{}, fmt.Errorf("commit blob write: %w", err)
	}
	return stored, nil
}

func (s *contentStore) RecordOrphans(ctx context.Context) (int, error) {
	root := filepath.Join(s.root, "blobs")
	rows, err := s.pool.Query(ctx, `select encode(sha256, 'hex') from blobs`)
	if err != nil {
		return 0, fmt.Errorf("list recorded blobs: %w", err)
	}
	recordedDigests := make(map[string]struct{})
	for rows.Next() {
		var digest string
		if err := rows.Scan(&digest); err != nil {
			rows.Close()
			return 0, fmt.Errorf("read recorded blob digest: %w", err)
		}
		recordedDigests[digest] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("list recorded blobs: %w", err)
	}
	rows.Close()

	recorded := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		encoded := entry.Name()
		digestBytes, err := hex.DecodeString(encoded)
		if err != nil || len(digestBytes) != sha256.Size || filepath.Base(filepath.Dir(path)) != encoded[:2] {
			return nil
		}
		if _, exists := recordedDigests[encoded]; exists {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open orphan candidate: %w", err)
		}
		hash := sha256.New()
		byteSize, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf("hash orphan candidate: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close orphan candidate: %w", closeErr)
		}
		if !bytes.Equal(hash.Sum(nil), digestBytes) {
			return fmt.Errorf("orphan candidate %q does not match its content address", path)
		}

		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin orphan recovery: %w", err)
		}
		defer tx.Rollback(ctx)
		var locked int
		if err := tx.QueryRow(ctx, `
			select 1 from pg_advisory_xact_lock(
			    hashtextextended('lumihub-blob:' || encode($1::bytea, 'hex'), 0)
			)
		`, digestBytes).Scan(&locked); err != nil {
			return fmt.Errorf("lock orphan digest: %w", err)
		}
		result, err := tx.Exec(ctx, `
			insert into blobs (id, sha256, byte_size, storage_key)
			values ($1, $2, $3, $4)
			on conflict (sha256) do nothing
		`, uuid.New(), digestBytes, byteSize, filepath.ToSlash(filepath.Join("blobs", encoded[:2], encoded)))
		if err != nil {
			return fmt.Errorf("record orphan blob: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit orphan recovery: %w", err)
		}
		if result.RowsAffected() == 1 {
			recorded++
		}
		return nil
	})
	if err != nil {
		return recorded, fmt.Errorf("scan canonical blobs: %w", err)
	}
	return recorded, nil
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

func (s *contentStore) InternalRedirect(ctx context.Context, id uuid.UUID) (string, error) {
	location, err := s.blobLocation(ctx, id)
	if err != nil {
		return "", err
	}
	path, err := s.path(location.StorageKey)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("find blob file: %w", err)
	}

	key := filepath.ToSlash(location.StorageKey)
	relative, ok := strings.CutPrefix(key, "blobs/")
	if !ok || relative == "" {
		return "", fmt.Errorf("blob storage key is outside the blob directory")
	}
	return internalBlobPrefix + relative, nil
}

func (s *contentStore) Delete(ctx context.Context, id uuid.UUID) error {
	location, err := s.blobLocation(ctx, id)
	if err != nil {
		return err
	}
	path, err := s.path(location.StorageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}

func (s *contentStore) DeleteDerivatives(ctx context.Context, digest [sha256.Size]byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded := hex.EncodeToString(digest[:])
	path := filepath.Join(s.root, "derivatives", encoded[:2], encoded)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete blob derivatives: %w", err)
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
	temporaryName, _, err := stage(filepath.Dir(path), r, nil)
	if err != nil {
		return fmt.Errorf("stage derivative: %w", err)
	}
	defer os.Remove(temporaryName)
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("install derivative: %w", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return fmt.Errorf("make derivative readable by the byte server: %w", err)
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
	if os.IsNotExist(err) {
		return nil, ErrDerivativeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open derivative: %w", err)
	}
	return file, nil
}

func (s *contentStore) InternalDerivativeRedirect(ctx context.Context, id DerivativeID) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	path, err := s.derivativePath(id)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", ErrDerivativeNotFound
	} else if err != nil {
		return "", fmt.Errorf("find derivative file: %w", err)
	}
	relative, err := filepath.Rel(filepath.Join(s.root, "derivatives"), path)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("derivative path is outside the derivative directory")
	}
	return internalDerivativePrefix + filepath.ToSlash(relative), nil
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

func stage(directory string, r io.Reader, observer io.Writer) (name string, byteSize int64, err error) {
	temporary, err := os.CreateTemp(directory, ".incoming-*")
	if err != nil {
		return "", 0, err
	}
	name = temporary.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
		}
	}()

	destination := io.Writer(temporary)
	if observer != nil {
		destination = io.MultiWriter(temporary, observer)
	}
	byteSize, copyErr := io.Copy(destination, r)
	closeErr := temporary.Close()
	if copyErr != nil {
		return "", 0, copyErr
	}
	if closeErr != nil {
		return "", 0, closeErr
	}
	return name, byteSize, nil
}

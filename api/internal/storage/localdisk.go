package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type localDisk struct {
	root string
}

func NewLocalDisk(root string) (Blob, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return localDisk{root: root}, nil
}

func (d localDisk) path(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return filepath.Join(d.root, filepath.FromSlash(key)), nil
}

func (d localDisk) Put(_ context.Context, key string, r io.Reader) error {
	full, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}

	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func (d localDisk) Get(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := d.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (d localDisk) Delete(_ context.Context, key string) error {
	full, err := d.path(key)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

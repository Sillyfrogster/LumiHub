package migration

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

// FileBackup is v1's uploads directory as the compressed archive that holds it.
type FileBackup struct {
	path string
}

// backupEntry is one regular file, readable only while its visit is running.
type backupEntry struct {
	Name    string
	Size    int64
	ModTime time.Time
	Body    io.Reader
}

// OpenFileBackup names an archive without reading it.
func OpenFileBackup(archive string) (*FileBackup, error) {
	if _, err := os.Stat(archive); err != nil {
		return nil, fmt.Errorf("open the v1 file backup: %w", err)
	}
	return &FileBackup{path: archive}, nil
}

// each walks every regular file once, so matching hundreds of paths reads the archive once.
func (backup *FileBackup) each(visit func(backupEntry) error) error {
	file, err := os.Open(backup.path)
	if err != nil {
		return fmt.Errorf("open the v1 file backup: %w", err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read the v1 file backup as gzip: %w", err)
	}
	defer compressed.Close()

	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read the v1 file backup: %w", err)
		}
		if !header.FileInfo().Mode().IsRegular() {
			continue
		}
		if err := visit(backupEntry{
			Name: header.Name, Size: header.Size, ModTime: header.ModTime, Body: reader,
		}); err != nil {
			return err
		}
	}
}

// backupIndex answers which record an entry belongs to, matching a stored path against the end of an entry name.
type backupIndex struct {
	paths map[string]int
	stems map[string]int
}

func newBackupIndex() *backupIndex {
	return &backupIndex{paths: make(map[string]int), stems: make(map[string]int)}
}

// want claims the entry whose name ends with this stored path.
func (index *backupIndex) want(stored string, at int) {
	if cleaned := cleanBackupPath(stored); cleaned != "" {
		index.paths[cleaned] = at
	}
}

// wantStem claims the entry whose filename without its extension is this identifier.
func (index *backupIndex) wantStem(identifier string, at int) {
	if identifier != "" {
		index.stems[identifier] = at
	}
}

func (index *backupIndex) find(name string) (int, bool) {
	cleaned := cleanBackupPath(name)
	if cleaned == "" {
		return 0, false
	}
	parts := strings.Split(cleaned, "/")
	for i := range parts {
		if at, found := index.paths[strings.Join(parts[i:], "/")]; found {
			return at, true
		}
	}
	base := path.Base(cleaned)
	if at, found := index.stems[strings.TrimSuffix(base, path.Ext(base))]; found {
		return at, true
	}
	return 0, false
}

func cleanBackupPath(value string) string {
	cleaned := path.Clean(strings.TrimPrefix(strings.ReplaceAll(value, "\\", "/"), "./"))
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

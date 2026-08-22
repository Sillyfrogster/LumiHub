package config

import (
	"bytes"
	"testing"
)

func setLinkingKey(t *testing.T) {
	t.Helper()
	t.Setenv("LINKING_HMAC_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setLinkingKey(t)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL is missing")
	}
}

func TestLoadUsesDefaultPort(t *testing.T) {
	setLinkingKey(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
}

func TestLoadRequiresAnExactUnpaddedLinkingKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")

	for _, key := range []string{
		"",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		t.Setenv("LINKING_HMAC_KEY", key)
		if _, err := Load(); err == nil {
			t.Errorf("Load accepted linking key %q", key)
		}
	}

	t.Setenv("LINKING_HMAC_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with a 32-byte key: %v", err)
	}
	if len(cfg.LinkingHMACKey) != 32 || !bytes.Equal(cfg.LinkingHMACKey, make([]byte, 32)) {
		t.Errorf("decoded linking key has %d bytes or the wrong value", len(cfg.LinkingHMACKey))
	}
}

func TestLoadRejectsAnUnreadableUploadCeiling(t *testing.T) {
	setLinkingKey(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("MAX_UPLOAD_BYTES", "55mb")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error: a ceiling nobody can read must not fall back to the default")
	}
}

func TestLoadRejectsIncompleteSMTPSettings(t *testing.T) {
	setLinkingKey(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("SMTP_ADDR", "smtp.example.com:587")
	t.Setenv("SMTP_FROM", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when an SMTP server has no sender address")
	}
}

func TestLoadUsesSettledIngestLimits(t *testing.T) {
	setLinkingKey(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	for _, name := range []string{
		"MAX_UPLOAD_BYTES",
		"MAX_ARCHIVE_ENTRIES",
		"MAX_ARCHIVE_ENTRY_BYTES",
		"MAX_ARCHIVE_BYTES",
		"MAX_ARCHIVE_COMPRESSION_RATIO",
		"INGEST_WORKERS",
	} {
		t.Setenv(name, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxUploadBytes != 32<<20 {
		t.Errorf("upload limit = %d, want 32 MB", cfg.MaxUploadBytes)
	}
	if cfg.ProbeLimits.MaxArchiveEntries != 512 {
		t.Errorf("archive entries = %d, want 512", cfg.ProbeLimits.MaxArchiveEntries)
	}
	if cfg.ProbeLimits.MaxEntryBytes != 32<<20 {
		t.Errorf("entry limit = %d, want 32 MB", cfg.ProbeLimits.MaxEntryBytes)
	}
	if cfg.ProbeLimits.MaxArchiveBytes != 128<<20 {
		t.Errorf("archive limit = %d, want 128 MB", cfg.ProbeLimits.MaxArchiveBytes)
	}
	if cfg.ProbeLimits.MaxCompressionRatio != 100 {
		t.Errorf("compression ratio = %v, want 100", cfg.ProbeLimits.MaxCompressionRatio)
	}
	if cfg.IngestWorkers != 2 {
		t.Errorf("ingest workers = %d, want 2", cfg.IngestWorkers)
	}
}

func TestLoadReadsIngestLimitOverrides(t *testing.T) {
	setLinkingKey(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("MAX_UPLOAD_BYTES", "101")
	t.Setenv("MAX_ARCHIVE_ENTRIES", "102")
	t.Setenv("MAX_ARCHIVE_ENTRY_BYTES", "103")
	t.Setenv("MAX_ARCHIVE_BYTES", "104")
	t.Setenv("MAX_ARCHIVE_COMPRESSION_RATIO", "10.5")
	t.Setenv("INGEST_WORKERS", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxUploadBytes != 101 || cfg.ProbeLimits.MaxArchiveEntries != 102 ||
		cfg.ProbeLimits.MaxEntryBytes != 103 || cfg.ProbeLimits.MaxArchiveBytes != 104 ||
		cfg.ProbeLimits.MaxCompressionRatio != 10.5 || cfg.IngestWorkers != 3 {
		t.Fatalf("overridden ingest settings = %+v", cfg)
	}
}

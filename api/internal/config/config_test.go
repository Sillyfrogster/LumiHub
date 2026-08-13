package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when DATABASE_URL is missing")
	}
}

func TestLoadUsesDefaultPort(t *testing.T) {
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

func TestLoadRejectsAnUnreadableUploadCeiling(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("MAX_UPLOAD_BYTES", "55mb")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error: a ceiling nobody can read must not fall back to the default")
	}
}

func TestLoadRejectsIncompleteSMTPSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/lumihub_dev")
	t.Setenv("UPLOADS_DIR", "/tmp/uploads")
	t.Setenv("SMTP_ADDR", "smtp.example.com:587")
	t.Setenv("SMTP_FROM", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when an SMTP server has no sender address")
	}
}

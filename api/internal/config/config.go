package config

import (
	"fmt"
	"os"
	"strconv"

	apihttp "github.com/Sillyfrogster/LumiHub/api/internal/http"
	"github.com/Sillyfrogster/LumiHub/api/internal/postgres"
)

// defaultMaxUploadBytes is 55 MB. The largest asset in production is a
// character of 9.45 MB plus an avatar of up to 1.26 MB, so this is about five
// times the biggest real file.
const defaultMaxUploadBytes = 55 << 20

// Config holds every setting the service runs on. They are gathered here and
// handed down, so nothing reaches for a setting on its own.
type Config struct {
	Port           string
	SiteURL        string
	SMTP           SMTPSettings
	Database       postgres.Settings
	UploadsDir     string
	MaxUploadBytes int64
	Server         apihttp.Timeouts
	Deadlines      apihttp.Deadlines
}

type SMTPSettings struct {
	Address  string
	From     string
	Username string
	Password string
}

// Load reads settings from the environment and rejects anything missing.
func Load() (Config, error) {
	databaseURL := get("DATABASE_URL", "")
	cfg := Config{
		Port:       get("PORT", "8080"),
		SiteURL:    get("SITE_URL", "http://localhost:3000"),
		Database:   postgres.DefaultSettings(databaseURL),
		UploadsDir: get("UPLOADS_DIR", ""),
		Server:     apihttp.DefaultTimeouts(),
		Deadlines:  apihttp.DefaultDeadlines(),
		SMTP: SMTPSettings{
			Address:  get("SMTP_ADDR", ""),
			From:     get("SMTP_FROM", ""),
			Username: get("SMTP_USERNAME", ""),
			Password: get("SMTP_PASSWORD", ""),
		},
	}

	for name, value := range map[string]string{
		"DATABASE_URL": databaseURL,
		"UPLOADS_DIR":  cfg.UploadsDir,
	} {
		if value == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}

	max, err := bytesOrDefault("MAX_UPLOAD_BYTES", defaultMaxUploadBytes)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxUploadBytes = max
	if (cfg.SMTP.Address == "") != (cfg.SMTP.From == "") {
		return Config{}, fmt.Errorf("SMTP_ADDR and SMTP_FROM must be set together")
	}
	if (cfg.SMTP.Username == "") != (cfg.SMTP.Password == "") {
		return Config{}, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be set together")
	}
	if cfg.SMTP.Address == "" && cfg.SMTP.Username != "" {
		return Config{}, fmt.Errorf("SMTP credentials need SMTP_ADDR and SMTP_FROM")
	}

	return cfg, nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func bytesOrDefault(key string, fallback int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive number of bytes, got %q", key, raw)
	}
	return n, nil
}

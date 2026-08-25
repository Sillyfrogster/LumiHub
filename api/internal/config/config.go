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
	Database       postgres.Settings
	UploadsDir     string
	MaxUploadBytes int64
	Server         apihttp.Timeouts
	Deadlines      apihttp.Deadlines
}

// Load reads settings from the environment and rejects anything missing.
func Load() (Config, error) {
	databaseURL := get("DATABASE_URL", "")
	cfg := Config{
		Port:       get("PORT", "8080"),
		Database:   postgres.DefaultSettings(databaseURL),
		UploadsDir: get("UPLOADS_DIR", ""),
		Server:     apihttp.DefaultTimeouts(),
		Deadlines:  apihttp.DefaultDeadlines(),
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

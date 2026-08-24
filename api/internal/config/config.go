package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	apihttp "github.com/Sillyfrogster/Illarin/api/internal/http"
	"github.com/Sillyfrogster/Illarin/api/internal/postgres"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
)

const (
	defaultMaxUploadBytes               = 32 << 20
	defaultStorageFreeSpaceReserveBytes = 5 << 30
	defaultAccountStorageCapBytes       = 1 << 30
)

// Config holds every setting the service runs on. They are gathered here and
// handed down, so nothing reaches for a setting on its own.
type Config struct {
	Port                         string
	SiteURL                      string
	SMTP                         SMTPSettings
	Microsoft365                 Microsoft365Settings
	Discord                      DiscordSettings
	Database                     postgres.Settings
	UploadsDir                   string
	MaxUploadBytes               int64
	StorageFreeSpaceReserveBytes int64
	AccountStorageCapBytes       int64
	LinkingHMACKey               []byte
	ProbeLimits                  probe.Limits
	IngestWorkers                int
	Server                       apihttp.Timeouts
	Deadlines                    apihttp.Deadlines
}

type SMTPSettings struct {
	Address  string
	From     string
	Username string
	Password string
}

type Microsoft365Settings struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	Mailbox      string
}

type DiscordSettings struct {
	ClientID     string
	ClientSecret string
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
		Microsoft365: Microsoft365Settings{
			TenantID: get("MICROSOFT_365_TENANT_ID", ""),
			ClientID: get("MICROSOFT_365_CLIENT_ID", ""),
			Mailbox:  get("MICROSOFT_365_MAILBOX", ""),
		},
		Discord: DiscordSettings{
			ClientID:     get("DISCORD_CLIENT_ID", ""),
			ClientSecret: get("DISCORD_CLIENT_SECRET", ""),
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
	reserve, err := bytesOrDefault(
		"STORAGE_FREE_SPACE_RESERVE_BYTES", defaultStorageFreeSpaceReserveBytes,
	)
	if err != nil {
		return Config{}, err
	}
	cfg.StorageFreeSpaceReserveBytes = reserve
	accountCap, err := bytesOrDefault("ACCOUNT_STORAGE_CAP_BYTES", defaultAccountStorageCapBytes)
	if err != nil {
		return Config{}, err
	}
	cfg.AccountStorageCapBytes = accountCap
	linkingKey, err := base64.RawURLEncoding.DecodeString(get("LINKING_HMAC_KEY", ""))
	if err != nil || len(linkingKey) != 32 {
		return Config{}, fmt.Errorf("LINKING_HMAC_KEY must be 32 bytes encoded as unpadded base64url")
	}
	cfg.LinkingHMACKey = linkingKey
	limits := probe.DefaultLimits()
	entries, err := intOrDefault("MAX_ARCHIVE_ENTRIES", limits.MaxArchiveEntries)
	if err != nil {
		return Config{}, err
	}
	entryBytes, err := bytesOrDefault("MAX_ARCHIVE_ENTRY_BYTES", int64(limits.MaxEntryBytes))
	if err != nil {
		return Config{}, err
	}
	archiveBytes, err := bytesOrDefault("MAX_ARCHIVE_BYTES", int64(limits.MaxArchiveBytes))
	if err != nil {
		return Config{}, err
	}
	ratio, err := floatOrDefault("MAX_ARCHIVE_COMPRESSION_RATIO", limits.MaxCompressionRatio)
	if err != nil {
		return Config{}, err
	}
	cfg.ProbeLimits = probe.Limits{
		MaxArchiveEntries:   entries,
		MaxEntryBytes:       uint64(entryBytes),
		MaxArchiveBytes:     uint64(archiveBytes),
		MaxCompressionRatio: ratio,
	}
	workers, err := intOrDefault("INGEST_WORKERS", 2)
	if err != nil {
		return Config{}, err
	}
	cfg.IngestWorkers = workers
	if (cfg.SMTP.Address == "") != (cfg.SMTP.From == "") {
		return Config{}, fmt.Errorf("SMTP_ADDR and SMTP_FROM must be set together")
	}
	if (cfg.SMTP.Username == "") != (cfg.SMTP.Password == "") {
		return Config{}, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be set together")
	}
	if cfg.SMTP.Address == "" && cfg.SMTP.Username != "" {
		return Config{}, fmt.Errorf("SMTP credentials need SMTP_ADDR and SMTP_FROM")
	}
	microsoftSecretFile := get("MICROSOFT_365_CLIENT_SECRET_FILE", "")
	microsoftValues := []string{
		cfg.Microsoft365.TenantID,
		cfg.Microsoft365.ClientID,
		microsoftSecretFile,
		cfg.Microsoft365.Mailbox,
	}
	microsoftSet := 0
	for _, value := range microsoftValues {
		if value != "" {
			microsoftSet++
		}
	}
	if microsoftSet != 0 && microsoftSet != len(microsoftValues) {
		return Config{}, fmt.Errorf("Microsoft 365 needs tenant id, client id, client secret file and mailbox")
	}
	if microsoftSet > 0 && cfg.SMTP.Address != "" {
		return Config{}, fmt.Errorf("configure either Microsoft 365 or SMTP, not both")
	}
	if microsoftSecretFile != "" {
		secret, err := os.ReadFile(microsoftSecretFile)
		if err != nil {
			return Config{}, fmt.Errorf("read MICROSOFT_365_CLIENT_SECRET_FILE: %w", err)
		}
		cfg.Microsoft365.ClientSecret = strings.TrimSpace(string(secret))
		if cfg.Microsoft365.ClientSecret == "" {
			return Config{}, fmt.Errorf("MICROSOFT_365_CLIENT_SECRET_FILE is empty")
		}
	}
	if (cfg.Discord.ClientID == "") != (cfg.Discord.ClientSecret == "") {
		return Config{}, fmt.Errorf("DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET must be set together")
	}

	return cfg, nil
}

func intOrDefault(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, raw)
	}
	return n, nil
}

func floatOrDefault(key string, fallback float64) (float64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive number, got %q", key, raw)
	}
	return n, nil
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

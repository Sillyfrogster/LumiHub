package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const backupLockName = "lumihub-backup:blob-deletion"

type settings struct {
	databaseURL    string
	uploadsDir     string
	workDir        string
	repository     string
	passwordFile   string
	keepDaily      int
	commandTimeout time.Duration
}

type commandRunner interface {
	run(context.Context, string, []string, []string) error
}

type lockSession interface {
	acquire(context.Context) (func() error, error)
}

type processRunner struct{}

func (processRunner) run(ctx context.Context, name string, args, environment []string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = append(os.Environ(), environment...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

type postgresLock struct {
	databaseURL string
}

func (l postgresLock) acquire(ctx context.Context) (func() error, error) {
	connection, err := pgx.Connect(ctx, l.databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect for backup lock: %w", err)
	}

	var locked int
	if err := connection.QueryRow(ctx, `select 1 from pg_advisory_lock(hashtextextended($1, 0))`, backupLockName).Scan(&locked); err != nil {
		_ = connection.Close(context.Background())
		return nil, fmt.Errorf("take backup lock: %w", err)
	}

	return func() error {
		closeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := connection.Close(closeContext); err != nil {
			return fmt.Errorf("release backup lock: %w", err)
		}
		return nil
	}, nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("backup failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return errors.New("usage: backup <run|init|check>")
	}

	runner := processRunner{}
	switch os.Args[1] {
	case "run":
		cfg, err := loadSettings(os.Getenv)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.commandTimeout)
		defer cancel()

		return takeBackup(ctx, cfg, postgresLock{databaseURL: cfg.databaseURL}, runner)
	case "init":
		cfg, err := loadResticSettings(os.Getenv)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.commandTimeout)
		defer cancel()

		return runner.run(ctx, "restic", []string{"init"}, resticEnvironment(cfg))
	case "check":
		cfg, err := loadResticSettings(os.Getenv)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.commandTimeout)
		defer cancel()

		return runner.run(ctx, "restic", []string{"check"}, resticEnvironment(cfg))
	default:
		return errors.New("usage: backup <run|init|check>")
	}
}

func loadSettings(get func(string) string) (settings, error) {
	cfg, err := loadResticSettings(get)
	if err != nil {
		return settings{}, err
	}

	cfg.databaseURL = strings.TrimSpace(get("DATABASE_URL"))
	cfg.uploadsDir = strings.TrimSpace(get("UPLOADS_DIR"))
	for name, value := range map[string]string{
		"DATABASE_URL": cfg.databaseURL,
		"UPLOADS_DIR":  cfg.uploadsDir,
	} {
		if value == "" {
			return settings{}, fmt.Errorf("%s is required", name)
		}
	}

	return cfg, nil
}

func loadResticSettings(get func(string) string) (settings, error) {
	keepDaily := 30
	if value := strings.TrimSpace(get("RESTIC_KEEP_DAILY")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return settings{}, errors.New("RESTIC_KEEP_DAILY must be a positive number")
		}
		keepDaily = parsed
	}

	commandTimeout := 12 * time.Hour
	if value := strings.TrimSpace(get("BACKUP_TIMEOUT")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return settings{}, errors.New("BACKUP_TIMEOUT must be a positive duration")
		}
		commandTimeout = parsed
	}

	cfg := settings{
		workDir:        strings.TrimSpace(get("BACKUP_WORK_DIR")),
		repository:     strings.TrimSpace(get("RESTIC_REPOSITORY")),
		passwordFile:   strings.TrimSpace(get("RESTIC_PASSWORD_FILE")),
		keepDaily:      keepDaily,
		commandTimeout: commandTimeout,
	}
	if cfg.workDir == "" {
		cfg.workDir = "/backup-work"
	}

	required := map[string]string{
		"RESTIC_REPOSITORY":    cfg.repository,
		"RESTIC_PASSWORD_FILE": cfg.passwordFile,
	}
	for name, value := range required {
		if value == "" {
			return settings{}, fmt.Errorf("%s is required", name)
		}
	}

	return cfg, nil
}

func takeBackup(ctx context.Context, cfg settings, locker lockSession, runner commandRunner) error {
	blobsDir := filepath.Join(cfg.uploadsDir, "blobs")
	if info, err := os.Stat(blobsDir); err != nil {
		return fmt.Errorf("read blob directory: %w", err)
	} else if !info.IsDir() {
		return errors.New("blob path is not a directory")
	}

	resticEnv := resticEnvironment(cfg)
	if err := runner.run(ctx, "restic", []string{"snapshots", "--latest", "1"}, resticEnv); err != nil {
		return fmt.Errorf("check backup repository: %w", err)
	}

	if err := os.MkdirAll(cfg.workDir, 0o700); err != nil {
		return fmt.Errorf("make backup work directory: %w", err)
	}
	workDir, err := os.MkdirTemp(cfg.workDir, "illarin-")
	if err != nil {
		return fmt.Errorf("make backup workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	release, err := locker.acquire(ctx)
	if err != nil {
		return err
	}
	locked := true
	defer func() {
		if locked {
			_ = release()
		}
	}()

	dumpPath := filepath.Join(workDir, "database.dump")
	if err := runner.run(ctx, "pg_dump", []string{
		"--format=custom",
		"--no-owner",
		"--no-privileges",
		"--file=" + dumpPath,
	}, []string{"PGDATABASE=" + cfg.databaseURL}); err != nil {
		return fmt.Errorf("dump database: %w", err)
	}

	if err := runner.run(ctx, "restic", []string{
		"backup",
		"--tag", "illarin",
		dumpPath,
		blobsDir,
	}, resticEnv); err != nil {
		return fmt.Errorf("store backup: %w", err)
	}

	releaseErr := release()
	locked = false
	if releaseErr != nil {
		return releaseErr
	}

	if err := runner.run(ctx, "restic", []string{
		"forget",
		"--tag", "illarin",
		"--keep-daily", strconv.Itoa(cfg.keepDaily),
		"--prune",
	}, resticEnv); err != nil {
		return fmt.Errorf("apply backup retention: %w", err)
	}

	return nil
}

func resticEnvironment(cfg settings) []string {
	return []string{
		"RESTIC_REPOSITORY=" + cfg.repository,
		"RESTIC_PASSWORD_FILE=" + cfg.passwordFile,
	}
}

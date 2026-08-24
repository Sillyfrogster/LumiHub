package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakeLock struct {
	locked   bool
	releases int
}

func (l *fakeLock) acquire(context.Context) (func() error, error) {
	l.locked = true
	return func() error {
		l.locked = false
		l.releases++
		return nil
	}, nil
}

type fakeRunner struct {
	lock      *fakeLock
	commands  []string
	failNamed string
	t         *testing.T
}

func (r *fakeRunner) run(_ context.Context, name string, args, _ []string) error {
	command := strings.Join(append([]string{name}, args...), " ")
	r.commands = append(r.commands, command)

	if (name == "pg_dump" || name == "restic" && len(args) > 0 && args[0] == "backup") && !r.lock.locked {
		r.t.Errorf("%s ran without the deletion lock", name)
	}
	if name == "restic" && len(args) > 0 && args[0] == "forget" && r.lock.locked {
		r.t.Error("retention ran while the deletion lock was held")
	}
	if name == r.failNamed {
		return errors.New("refused")
	}
	return nil
}

func TestTakeBackupKeepsDeletionLockedThroughDumpAndBlobCopy(t *testing.T) {
	uploadsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(uploadsDir, "blobs"), 0o700); err != nil {
		t.Fatal(err)
	}

	lock := &fakeLock{}
	runner := &fakeRunner{lock: lock, t: t}
	cfg := testSettings(uploadsDir, t.TempDir())

	if err := takeBackup(context.Background(), cfg, lock, runner); err != nil {
		t.Fatal(err)
	}

	if lock.locked || lock.releases != 1 {
		t.Fatalf("lock state after backup = locked %v, releases %d", lock.locked, lock.releases)
	}
	wantPrefixes := []string{
		"restic snapshots",
		"pg_dump --format=custom",
		"restic backup",
		"restic forget",
	}
	if len(runner.commands) != len(wantPrefixes) {
		t.Fatalf("commands = %v", runner.commands)
	}
	for i, prefix := range wantPrefixes {
		if !strings.HasPrefix(runner.commands[i], prefix) {
			t.Errorf("command %d = %q, want prefix %q", i, runner.commands[i], prefix)
		}
	}
	if !strings.Contains(runner.commands[1], "--dbname=postgres://db/illarin") {
		t.Fatalf("database dump has no connection string: %q", runner.commands[1])
	}
}

func TestTakeBackupReleasesLockAfterDumpFailure(t *testing.T) {
	uploadsDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(uploadsDir, "blobs"), 0o700); err != nil {
		t.Fatal(err)
	}

	lock := &fakeLock{}
	runner := &fakeRunner{lock: lock, failNamed: "pg_dump", t: t}

	err := takeBackup(context.Background(), testSettings(uploadsDir, t.TempDir()), lock, runner)
	if err == nil || !strings.Contains(err.Error(), "dump database") {
		t.Fatalf("error = %v", err)
	}
	if lock.locked || lock.releases != 1 {
		t.Fatalf("lock state after failure = locked %v, releases %d", lock.locked, lock.releases)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("commands after failure = %v", runner.commands)
	}
}

func TestLoadSettingsUsesThirtyDayRetention(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":         "postgres://db/illarin",
		"UPLOADS_DIR":          "/uploads",
		"RESTIC_REPOSITORY":    "s3:bucket",
		"RESTIC_PASSWORD_FILE": "/run/secrets/restic",
	}

	cfg, err := loadSettings(func(name string) string { return values[name] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.keepDaily != 30 || cfg.commandTimeout != 12*time.Hour {
		t.Fatalf("defaults = keep %d, timeout %s", cfg.keepDaily, cfg.commandTimeout)
	}

	wantEnvironment := []string{"RESTIC_REPOSITORY=s3:bucket", "RESTIC_PASSWORD_FILE=/run/secrets/restic"}
	if !reflect.DeepEqual(resticEnvironment(cfg), wantEnvironment) {
		t.Fatalf("restic environment = %v", resticEnvironment(cfg))
	}
}

func TestPostgresLockBlocksBlobDeletionLock(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	release, err := (postgresLock{databaseURL: databaseURL}).acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}

	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())

	var sharedLockAvailable bool
	if err := connection.QueryRow(ctx, `select pg_try_advisory_lock_shared(hashtextextended($1, 0))`, backupLockName).Scan(&sharedLockAvailable); err != nil {
		t.Fatal(err)
	}
	if sharedLockAvailable {
		t.Fatal("blob deletion lock was available during backup")
	}

	if err := release(); err != nil {
		t.Fatal(err)
	}
	if err := connection.QueryRow(ctx, `select pg_try_advisory_lock_shared(hashtextextended($1, 0))`, backupLockName).Scan(&sharedLockAvailable); err != nil {
		t.Fatal(err)
	}
	if !sharedLockAvailable {
		t.Fatal("blob deletion lock stayed blocked after backup")
	}
	if _, err := connection.Exec(ctx, `select pg_advisory_unlock_shared(hashtextextended($1, 0))`, backupLockName); err != nil {
		t.Fatal(err)
	}
}

func testSettings(uploadsDir, workDir string) settings {
	return settings{
		databaseURL:    "postgres://db/illarin",
		uploadsDir:     uploadsDir,
		workDir:        workDir,
		repository:     "s3:bucket",
		passwordFile:   "/run/secrets/restic",
		keepDaily:      30,
		commandTimeout: time.Hour,
	}
}

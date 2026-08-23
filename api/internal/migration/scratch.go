package migration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ScratchDatabase is a throwaway database the migration reads its source from, so nothing live is touched.
type ScratchDatabase struct {
	Pool *pgxpool.Pool
	name string
	base string
}

// NewScratchDatabase creates an empty database beside the one the base URL addresses.
func NewScratchDatabase(ctx context.Context, base, prefix string) (*ScratchDatabase, error) {
	name := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")
	admin, err := connectAdmin(ctx, base)
	if err != nil {
		return nil, err
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx,
		"create database "+pgx.Identifier{name}.Sanitize()+" template template0"); err != nil {
		return nil, fmt.Errorf("create scratch database: %w", err)
	}

	config, err := pgxpool.ParseConfig(base)
	if err != nil {
		return nil, fmt.Errorf("parse the scratch database settings: %w", err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open the scratch database: %w", err)
	}
	return &ScratchDatabase{Pool: pool, name: name, base: base}, nil
}

// RestoreDump loads a pg_dump script into the scratch database.
func (s *ScratchDatabase) RestoreDump(ctx context.Context, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("open the dump: %w", err)
	}
	config, err := pgx.ParseConfig(s.base)
	if err != nil {
		return fmt.Errorf("parse the scratch database settings: %w", err)
	}
	command := exec.CommandContext(ctx, "psql", "-X", "--quiet", "--set", "ON_ERROR_STOP=1",
		"--host", config.Host, "--port", strconv.FormatUint(uint64(config.Port), 10),
		"--username", config.User, "--dbname", s.name)
	command.Env = append(os.Environ(), "PGPASSWORD="+config.Password)
	command.Stdin = bytes.NewReader(withoutOwnerStatements(body))
	var failure bytes.Buffer
	command.Stderr = &failure
	if err := command.Run(); err != nil {
		return fmt.Errorf("restore the dump: %s", strings.TrimSpace(failure.String()))
	}
	return nil
}

// Drop closes the pool and removes the scratch database.
func (s *ScratchDatabase) Drop(ctx context.Context) error {
	s.Pool.Close()
	admin, err := connectAdmin(ctx, s.base)
	if err != nil {
		return err
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx,
		"drop database if exists "+pgx.Identifier{s.name}.Sanitize()+" with (force)"); err != nil {
		return fmt.Errorf("drop scratch database: %w", err)
	}
	return nil
}

func connectAdmin(ctx context.Context, base string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(base)
	if err != nil {
		return nil, fmt.Errorf("parse the database settings: %w", err)
	}
	config.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect as the database owner: %w", err)
	}
	return admin, nil
}

// withoutOwnerStatements strips the ownership lines a dump carries, which only its original owner may run.
func withoutOwnerStatements(body []byte) []byte {
	filtered := bytes.NewBuffer(make([]byte, 0, len(body)))
	filtered.WriteString("\\set VERBOSITY sqlstate\n")
	for _, line := range bytes.SplitAfter(body, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("ALTER ")) &&
			bytes.HasSuffix(trimmed, []byte(" OWNER TO postgres;")) {
			continue
		}
		filtered.Write(line)
	}
	return filtered.Bytes()
}

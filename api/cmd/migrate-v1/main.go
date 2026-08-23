// Command migrate-v1 carries the v1 catalog into Illarin, once.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/config"
	"github.com/Sillyfrogster/Illarin/api/internal/format/modules"
	"github.com/Sillyfrogster/Illarin/api/internal/migration"
	"github.com/Sillyfrogster/Illarin/api/internal/postgres"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	source := flag.String("source", "", "connection string for the restored v1 database")
	backup := flag.String("backup", "", "path to the v1 file backup archive")
	hosts := flag.String("image-hosts", "",
		"comma-separated hosts the one bounded image fetch may reach")
	flag.Parse()
	if *source == "" || *backup == "" {
		return fmt.Errorf("migrate-v1 needs -source and -backup")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	target, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("target database: %w", err)
	}
	defer target.Close()
	from, err := pgxpool.New(ctx, *source)
	if err != nil {
		return fmt.Errorf("source database: %w", err)
	}
	defer from.Close()

	blob, err := storage.NewStore(target, cfg.UploadsDir)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	registry, err := modules.Registry()
	if err != nil {
		return err
	}
	archive, err := migration.OpenFileBackup(*backup)
	if err != nil {
		return err
	}

	allowed := make([]string, 0)
	for _, host := range strings.Split(*hosts, ",") {
		if trimmed := strings.TrimSpace(host); trimmed != "" {
			allowed = append(allowed, trimmed)
		}
	}
	carried, err := migration.AccountsMigrated(ctx, target)
	if err != nil {
		return err
	}
	if !carried {
		accounts, err := migration.MigrateAccounts(ctx, from, target)
		if err != nil {
			return err
		}
		log.Printf("migrated %d accounts", accounts.Accounts)
	}

	report, err := migration.Run(ctx, migration.Settings{
		Source: from, Target: target, Backup: archive,
		Assets:  asset.NewServiceForSite(target, registry, blob, cfg.ProbeLimits, cfg.SiteURL),
		Fetcher: migration.NewAllowlistedFetcher(allowed, migration.DefaultFetchLimits()),
	})
	if err != nil {
		return err
	}
	printReport(report)
	return nil
}

func printReport(report migration.Report) {
	kinds := make([]string, 0, len(report.Kinds))
	for kind := range report.Kinds {
		kinds = append(kinds, fmt.Sprintf("%s %d", kind, report.Kinds[kind]))
	}
	sort.Strings(kinds)
	log.Printf("migrated %d assets: %s", report.Assets, strings.Join(kinds, ", "))
	log.Printf(
		"staged %d images, fetched %d, reused %d, failed %d",
		report.Staging.Stored, report.Staging.Fetched, report.Staging.Reused, report.Staging.Failed,
	)
	log.Printf(
		"stored %d v1 addresses, %d preserved records, and marked %d assets below the publish floor",
		report.LegacyPaths, report.Preserved, report.BelowFloor,
	)
	counts := make(map[string]int, len(report.Exceptions))
	for _, entry := range report.Exceptions {
		counts[entry.Kind]++
	}
	kinds = kinds[:0]
	for kind, count := range counts {
		kinds = append(kinds, fmt.Sprintf("%s %d", kind, count))
	}
	sort.Strings(kinds)
	log.Printf("ledgered %d exceptions: %s", len(report.Exceptions), strings.Join(kinds, ", "))
}

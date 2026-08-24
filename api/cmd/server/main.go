package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/account"
	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/config"
	"github.com/Sillyfrogster/Illarin/api/internal/delivery"
	"github.com/Sillyfrogster/Illarin/api/internal/discord"
	"github.com/Sillyfrogster/Illarin/api/internal/format/modules"
	apihttp "github.com/Sillyfrogster/Illarin/api/internal/http"
	"github.com/Sillyfrogster/Illarin/api/internal/linking"
	"github.com/Sillyfrogster/Illarin/api/internal/postgres"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/gin-gonic/gin"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtimeContext, cancelRuntime := context.WithCancel(signalContext)
	defer cancelRuntime()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	pool, err := postgres.NewPool(runtimeContext, cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(runtimeContext); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	blob, err := storage.NewStore(pool, cfg.UploadsDir)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	registry, err := modules.Registry()
	if err != nil {
		return err
	}

	svc := asset.NewServiceForSite(pool, registry, blob, cfg.ProbeLimits, cfg.SiteURL)
	// Changing what a format declares or what the facet catalog holds is a
	// deploy, so the deploy is what recomputes the projections it invalidated.
	recomputed, err := svc.RecomputeStaleExportProjections(runtimeContext)
	if err != nil {
		return fmt.Errorf("export projections: %w", err)
	}
	if recomputed > 0 {
		log.Printf("recomputed the export projection for %d assets", recomputed)
	}
	remeasured, err := svc.RecomputeStaleFacetProjections(runtimeContext)
	if err != nil {
		return fmt.Errorf("facet projections: %w", err)
	}
	if remeasured > 0 {
		log.Printf("recomputed the facet projection for %d assets", remeasured)
	}
	var background sync.WaitGroup
	background.Add(2)
	go func() {
		defer background.Done()
		svc.RunIngestWorkers(runtimeContext, cfg.IngestWorkers, func(err error) {
			log.Printf("ingest worker: %v", err)
		})
	}()
	go func() {
		defer background.Done()
		svc.RunSweeper(runtimeContext, func(err error) {
			log.Printf("blob sweeper: %v", err)
		})
	}()
	defer func() {
		cancelRuntime()
		background.Wait()
	}()
	var verificationSender account.EmailSender = account.NewLogVerificationSender(log.Default())
	if cfg.Microsoft365.ClientID != "" {
		verificationSender, err = account.NewMicrosoftGraphSender(account.MicrosoftGraphSettings{
			TenantID:     cfg.Microsoft365.TenantID,
			ClientID:     cfg.Microsoft365.ClientID,
			ClientSecret: cfg.Microsoft365.ClientSecret,
			Mailbox:      cfg.Microsoft365.Mailbox,
		})
		if err != nil {
			return fmt.Errorf("Microsoft 365 email: %w", err)
		}
	} else if cfg.SMTP.Address != "" {
		verificationSender, err = account.NewSMTPSender(account.SMTPSettings{
			Address:  cfg.SMTP.Address,
			From:     cfg.SMTP.From,
			Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password,
		})
		if err != nil {
			return fmt.Errorf("verification email: %w", err)
		}
	}
	var discordProvider account.DiscordProvider
	if cfg.Discord.ClientID != "" {
		discordProvider, err = discord.NewClient(discord.DefaultConfig(
			cfg.Discord.ClientID,
			cfg.Discord.ClientSecret,
			strings.TrimRight(cfg.SiteURL, "/")+"/api/v1/auth/discord/callback",
		), nil)
		if err != nil {
			return fmt.Errorf("Discord sign-in: %w", err)
		}
	}
	accounts := account.NewService(pool, verificationSender, discordProvider, cfg.SiteURL)
	links := linking.NewService(pool, cfg.SiteURL, cfg.LinkingHMACKey)
	deliveries := delivery.NewService(pool, svc, links, delivery.DefaultSettings())
	background.Add(1)
	go func() {
		defer background.Done()
		deliveries.RunSweeper(runtimeContext, func(err error) {
			log.Printf("delivery sweeper: %v", err)
		})
	}()

	r := gin.New()
	r.Use(apihttp.Recovery(log.Default()))
	handlers := apihttp.NewHandlers(svc, accounts, links, deliveries, cfg.MaxUploadBytes)
	readiness := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		for _, directory := range []string{"blobs", "derivatives"} {
			info, err := os.Stat(filepath.Join(cfg.UploadsDir, directory))
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", directory)
			}
		}
		return nil
	}
	if err := apihttp.Register(r, handlers, cfg.Deadlines, readiness); err != nil {
		return fmt.Errorf("routes: %w", err)
	}

	server := apihttp.NewServer(":"+cfg.Port, r, cfg.Server)
	log.Printf("listening on %s", server.Addr)
	serverError := make(chan error, 1)
	go func() { serverError <- server.ListenAndServe() }()

	select {
	case err := <-serverError:
		cancelRuntime()
		background.Wait()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server: %w", err)
	case <-signalContext.Done():
		log.Print("shutting down")
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelShutdown()
		shutdownError := server.Shutdown(shutdownContext)
		cancelRuntime()
		background.Wait()
		if shutdownError != nil {
			_ = server.Close()
			return fmt.Errorf("server shutdown: %w", shutdownError)
		}
		return nil
	}
}

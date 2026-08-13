package main

import (
	"context"
	"log"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/config"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/passthrough"
	apihttp "github.com/Sillyfrogster/LumiHub/api/internal/http"
	"github.com/Sillyfrogster/LumiHub/api/internal/postgres"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := postgres.NewPool(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	blob, err := storage.NewStore(pool, cfg.UploadsDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	// Every format module is registered here and nowhere else.
	registry := format.NewRegistry(passthrough.New())

	svc := asset.NewService(pool, registry, blob)

	r := gin.New()
	r.Use(gin.Recovery())
	if err := apihttp.Register(r, apihttp.NewHandlers(svc, cfg.MaxUploadBytes), cfg.Deadlines); err != nil {
		log.Fatalf("routes: %v", err)
	}

	server := apihttp.NewServer(":"+cfg.Port, r, cfg.Server)
	log.Printf("listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

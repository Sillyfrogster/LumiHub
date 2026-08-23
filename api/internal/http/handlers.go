package http

import (
	"github.com/Sillyfrogster/Illarin/api/internal/account"
	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/linking"
)

// Handlers turns HTTP requests into catalog calls.
type Handlers struct {
	assets         *asset.Service
	accounts       *account.Service
	links          *linking.Service
	maxUploadBytes int64
}

func NewHandlers(
	assets *asset.Service,
	accounts *account.Service,
	links *linking.Service,
	maxUploadBytes int64,
) *Handlers {
	return &Handlers{
		assets:         assets,
		accounts:       accounts,
		links:          links,
		maxUploadBytes: maxUploadBytes,
	}
}

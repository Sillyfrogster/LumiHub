package http

import (
	"net/http"
	"time"
)

// Timeouts are the limits the server puts on a connection rather than on any
// one request.
type Timeouts struct {
	ReadHeader time.Duration
	Idle       time.Duration
}

// DefaultTimeouts are what the server runs with. A test takes these and
// changes the one limit it is about.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		ReadHeader: 10 * time.Second,
		Idle:       2 * time.Minute,
	}
}

// NewServer builds the server the API runs on.
func NewServer(addr string, handler http.Handler, t Timeouts) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,

		// Catches a client that opens a connection and then never finishes
		// asking for anything.
		ReadHeaderTimeout: t.ReadHeader,
		IdleTimeout:       t.Idle,

		// ReadTimeout and WriteTimeout are left unset on purpose, so read
		// this before adding one. Both are the total time a request may take,
		// counted from the moment it arrives, not the time since anything
		// last moved. An upload may be 32 MB, and 32 MB over a home
		// connection is several minutes of entirely healthy transfer, so any
		// value short enough to catch a stalled client also cuts off a
		// transfer that is going fine. The header limit above catches the
		// stalled client, and each route sets the limit for its own work in
		// Register.
	}
}

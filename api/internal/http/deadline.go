package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Deadlines is how long each kind of route may take. A route answering out of
// the database and a route sending 32 MB have nothing in common, so they do
// not share a number.
type Deadlines struct {
	JSON     time.Duration
	Upload   time.Duration
	Download time.Duration
}

// DefaultDeadlines are what the server runs with. Fifteen minutes for a file
// still gets 32 MB through on a connection carrying 64 KB a second, which is
// slower than anything we expect to see.
func DefaultDeadlines() Deadlines {
	return Deadlines{
		JSON:     5 * time.Second,
		Upload:   15 * time.Minute,
		Download: 15 * time.Minute,
	}
}

// Register puts every route the service answers on the router, each under a
// deadline. A route without one is refused rather than served with no limit at
// all, so adding a route means saying how long it may take.
func Register(r *gin.Engine, h *Handlers, d Deadlines) error {
	limits := map[string]time.Duration{
		routeKey(http.MethodGet, "/healthz"):                          d.JSON,
		routeKey(http.MethodPatch, "/v1/account/email"):               d.JSON,
		routeKey(http.MethodPatch, "/v1/account/handle"):              d.JSON,
		routeKey(http.MethodPut, "/v1/account/password"):              d.JSON,
		routeKey(http.MethodPut, "/v1/account/nsfw-visibility"):       d.JSON,
		routeKey(http.MethodDelete, "/v1/account/discord"):            d.JSON,
		routeKey(http.MethodGet, "/v1/auth/session"):                  d.JSON,
		routeKey(http.MethodGet, "/v1/auth/discord"):                  d.JSON,
		routeKey(http.MethodGet, "/v1/auth/discord/callback"):         d.JSON,
		routeKey(http.MethodPost, "/v1/auth/sign-in"):                 d.JSON,
		routeKey(http.MethodPost, "/v1/auth/sign-out"):                d.JSON,
		routeKey(http.MethodPost, "/v1/auth/sign-up"):                 d.JSON,
		routeKey(http.MethodPost, "/v1/auth/verify-email"):            d.JSON,
		routeKey(http.MethodPost, "/v1/auth/password-reset"):          d.JSON,
		routeKey(http.MethodPost, "/v1/auth/password-reset/complete"): d.JSON,
		routeKey(http.MethodGet, "/v1/profiles/:handle"):              d.JSON,
		routeKey(http.MethodGet, "/v1/assets"):                        d.JSON,
		routeKey(http.MethodPost, "/v1/assets"):                       d.Upload,
		routeKey(http.MethodGet, "/v1/assets/:id"):                    d.JSON,
		routeKey(http.MethodPut, "/v1/assets/:id/discovery"):          d.JSON,
		routeKey(http.MethodPut, "/v1/assets/:id/withhold"):           d.JSON,
		routeKey(http.MethodDelete, "/v1/assets/:id/withhold"):        d.JSON,
		routeKey(http.MethodPost, "/v1/assets/:id/media"):             d.Upload,
		routeKey(http.MethodGet, "/v1/assets/:id/media"):              d.JSON,
		routeKey(http.MethodGet, "/v1/ingests/:id"):                   d.JSON,
		routeKey(http.MethodPatch, "/v1/ingests/:id"):                 d.JSON,
		// :id is gin's way of writing "any id here".
		routeKey(http.MethodGet, "/download/:id"):                                 d.Download,
		routeKey(http.MethodGet, "/media/:media_id/:variant/:derivative_version"): d.Download,
	}

	routes := r.Group("", deadlineByRoute(limits))
	routes.GET("/healthz", health)
	RegisterHandlers(routes, h)

	for _, route := range r.Routes() {
		key := routeKey(route.Method, route.Path)
		if limits[key] == 0 {
			return fmt.Errorf("%s has no deadline, give it one in Register", key)
		}
	}
	return nil
}

// health says the service is up. It checks nothing else, so a database
// outage does not read as a dead service.
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// withDeadline limits a route to d, both on the work it does and on how long
// it may hold the connection while answering.
func withDeadline(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		// A recorded response in a test has no connection to put a deadline on.
		_ = http.NewResponseController(c.Writer).SetWriteDeadline(time.Now().Add(d))

		c.Next()
	}
}

// deadlineByRoute applies whichever deadline belongs to the route that was
// matched. The generated registration puts every route on one router, so it
// cannot carry a different middleware for each.
func deadlineByRoute(limits map[string]time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		d, ok := limits[routeKey(c.Request.Method, c.FullPath())]
		if !ok {
			c.Next()
			return
		}
		withDeadline(d)(c)
	}
}

func routeKey(method, path string) string { return method + " " + path }

package http

import (
	"log"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
)

// Recovery reports the matched route without logging request path values.
func Recovery(logger *log.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		route := c.FullPath()
		if route == "" {
			route = "unmatched route"
		}
		logger.Printf(
			"panic recovered while handling %s %s (%T)",
			c.Request.Method,
			route,
			recovered,
		)
		c.AbortWithStatus(nethttp.StatusInternalServerError)
	})
}

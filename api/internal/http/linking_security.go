package http

import (
	"errors"
	"mime"
	"net"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	maxLinkBodyBytes  = 4 << 10
	linkRequestHeader = "X-Illarin-Request"
)

func readLinkJSON(c *gin.Context, destination any) bool {
	return readBoundedJSON(c, destination, maxLinkBodyBytes, "The link request is too large.")
}

// readBoundedJSON refuses anything past its limit before parsing it, so an
// oversized body costs the process the bytes it takes to notice rather than the
// memory to hold it.
func readBoundedJSON(c *gin.Context, destination any, limit int64, tooLargeMessage string) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "Send JSON with the application/json content type."})
		return false
	}
	body := nethttp.MaxBytesReader(c.Writer, c.Request.Body, limit)
	if err := decodeOneJSON(body, destination); err != nil {
		var tooLarge *nethttp.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(nethttp.StatusRequestEntityTooLarge, gin.H{"error": tooLargeMessage})
			return false
		}
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "Send one valid JSON object."})
		return false
	}
	return true
}

func (h *Handlers) allowLinkBrowserMutation(c *gin.Context) bool {
	if c.GetHeader(linkRequestHeader) != "1" || c.GetHeader("Origin") != h.links.BrowserOrigin() {
		c.JSON(nethttp.StatusForbidden, gin.H{"error": "Open this action from Illarin and try again."})
		return false
	}
	return true
}

func noStoreLink(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func noStoreLinkedInstanceResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if strings.HasPrefix(path, "/v1/link/") ||
			path == "/v1/instances" || strings.HasPrefix(path, "/v1/instances/") ||
			strings.HasPrefix(path, "/v1/deliveries") ||
			path == "/v1/library/sync" ||
			strings.HasSuffix(path, "/instances") ||
			strings.HasSuffix(path, "/deliveries") {
			noStoreLink(c)
		}
		c.Next()
	}
}

func generatedParameterError(c *gin.Context, err error, status int) {
	if strings.Contains(err.Error(), "X-Illarin-Request") {
		c.JSON(nethttp.StatusForbidden, gin.H{
			"error": "Open this action from Illarin and try again.",
		})
		return
	}
	c.JSON(status, gin.H{"msg": err.Error()})
}

func linkRequestSource(c *gin.Context) string {
	host := remoteHost(c.Request.RemoteAddr)
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return host
	}
	forwarded := strings.Split(c.GetHeader("X-Forwarded-For"), ",")
	if len(forwarded) == 0 {
		return host
	}
	candidate := strings.TrimSpace(forwarded[len(forwarded)-1])
	if net.ParseIP(candidate) == nil {
		return host
	}
	return candidate
}

func remoteHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return address
}

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
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		c.JSON(nethttp.StatusBadRequest, gin.H{"error": "Send JSON with the application/json content type."})
		return false
	}
	body := nethttp.MaxBytesReader(c.Writer, c.Request.Body, maxLinkBodyBytes)
	if err := decodeOneJSON(body, destination); err != nil {
		var tooLarge *nethttp.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(nethttp.StatusRequestEntityTooLarge, gin.H{"error": "The link request is too large."})
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

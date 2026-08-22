package http

import (
	"bytes"
	"log"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryLogsTheRouteWithoutASecretPathValue(t *testing.T) {
	var output bytes.Buffer
	router := gin.New()
	router.Use(Recovery(log.New(&output, "", 0)))
	router.GET("/v1/link/authorizations/:requestCode", func(*gin.Context) {
		panic("synthetic panic")
	})

	secret := strings.Repeat("s", 43)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(nethttp.MethodGet, "/v1/link/authorizations/"+secret, nil),
	)

	if response.Code != nethttp.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(output.String(), secret) {
		t.Fatalf("recovery log exposed the request secret: %s", output.String())
	}
	if !strings.Contains(output.String(), "/v1/link/authorizations/:requestCode") {
		t.Fatalf("recovery log did not identify the safe route: %s", output.String())
	}
}

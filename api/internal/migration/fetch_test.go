package migration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOnlyAnAllowlistedHostIsFetched(t *testing.T) {
	server := imageServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte("bytes"))
	})
	fetcher := fetcherFor(t, server, DefaultFetchLimits())
	if _, err := fetcher.Fetch(context.Background(), server.URL+"/art.png"); err != nil {
		t.Fatalf("fetch an allowlisted image: %v", err)
	}
	_, err := fetcher.Fetch(context.Background(), "https://elsewhere.example/art.png")
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("fetching an unlisted host gave %v", err)
	}
}

func TestARedirectOffTheAllowlistIsRefused(t *testing.T) {
	server := imageServer(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/moved.png" {
			http.Redirect(writer, request, "https://elsewhere.example/art.png", http.StatusFound)
			return
		}
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte("bytes"))
	})
	fetcher := fetcherFor(t, server, DefaultFetchLimits())
	if _, err := fetcher.Fetch(context.Background(), server.URL+"/moved.png"); err == nil {
		t.Error("a redirect off the allowlist was followed")
	}
}

func TestAResponseThatIsNotAnImageIsRefused(t *testing.T) {
	server := imageServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("<html>gone</html>"))
	})
	fetcher := fetcherFor(t, server, DefaultFetchLimits())
	if _, err := fetcher.Fetch(context.Background(), server.URL+"/art.png"); err == nil {
		t.Error("a page was accepted as an image")
	}
}

func TestAnOversizedImageIsRefused(t *testing.T) {
	server := imageServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write([]byte(strings.Repeat("x", 64)))
	})
	limits := FetchLimits{Bytes: 16, Timeout: 5 * time.Second, Redirects: 1}
	fetcher := fetcherFor(t, server, limits)
	if _, err := fetcher.Fetch(context.Background(), server.URL+"/art.png"); err == nil {
		t.Error("an oversized image was accepted")
	}
}

func TestTheHostsOfASetOfAddressesAreNamed(t *testing.T) {
	hosts := FetchHosts([]string{
		"https://one.example/a.png", "https://one.example/b.png",
		"https://two.example/c.png", "not a url at all",
	})
	if len(hosts) != 2 || hosts[0] != "one.example" || hosts[1] != "two.example" {
		t.Errorf("hosts = %v", hosts)
	}
}

func imageServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

// fetcherFor allows the test server's host and trusts its certificate, which a local server cannot otherwise supply.
func fetcherFor(t *testing.T, server *httptest.Server, limits FetchLimits) *AllowlistedFetcher {
	t.Helper()
	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("read the test server address: %v", err)
	}
	fetcher := NewAllowlistedFetcher([]string{address.Hostname()}, limits)
	fetcher.client.Transport = server.Client().Transport
	return fetcher
}

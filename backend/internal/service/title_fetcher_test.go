package service_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
)

// testFetcher allows loopback so httptest servers are reachable.
func testFetcher(timeout time.Duration) service.TitleFetcher {
	return service.NewTitleFetcher(service.TitleFetcherOptions{Timeout: timeout, AllowPrivate: true})
}

func TestFetchTitle_ParsesTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><TITLE lang=\"en\">\n  Hello   &amp;\n Welcome </TITLE></head><body>x</body></html>"))
	}))
	defer srv.Close()

	assert.Equal(t, "Hello & Welcome", testFetcher(3*time.Second)(context.Background(), srv.URL+"/page"))
}

func TestFetchTitle_TruncatesTo255Runes(t *testing.T) {
	long := strings.Repeat("é", 300)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<title>" + long + "</title>"))
	}))
	defer srv.Close()

	got := testFetcher(3*time.Second)(context.Background(), srv.URL)
	assert.Equal(t, 255, utf8.RuneCountInString(got))
}

func TestFetchTitle_FallbackHostnameOnTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	got := testFetcher(150*time.Millisecond)(context.Background(), srv.URL+"/slow")
	assert.Equal(t, "127.0.0.1", got)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestFetchTitle_FallbackHostnameOnMissingTitleOrError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/500" {
			http.Error(w, "<title>Oops</title>", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("<html><title>   </title></html>"))
	}))
	defer srv.Close()

	f := testFetcher(3 * time.Second)
	assert.Equal(t, "127.0.0.1", f(context.Background(), srv.URL+"/empty"))
	assert.Equal(t, "127.0.0.1", f(context.Background(), srv.URL+"/500"))
	assert.Equal(t, "", f(context.Background(), "::not a url"))
	assert.Equal(t, "example.com", f(context.Background(), "ftp://example.com/file"))
}

func TestFetchTitle_BlocksPrivateIP(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("<title>Internal</title>"))
	}))
	defer srv.Close()

	// The default fetcher refuses loopback targets.
	assert.Equal(t, "127.0.0.1", service.FetchTitle(context.Background(), srv.URL))

	// A hostname resolving to a private range is refused before any request.
	f := service.NewTitleFetcher(service.TitleFetcherOptions{
		LookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("10.1.2.3")}}, nil
		},
	})
	assert.Equal(t, "intranet.example", f(context.Background(), "http://intranet.example:8080/"))
	assert.Equal(t, "169.254.169.254", f(context.Background(), "http://169.254.169.254/latest/meta-data"))

	// A name whose pre-check resolves publicly but whose dial goes to a
	// private address (DNS rebinding, or a redirect) is refused at dial time.
	rebind := service.NewTitleFetcher(service.TitleFetcherOptions{
		LookupIPAddr: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	assert.Equal(t, "localhost", rebind(context.Background(), "http://localhost:"+port+"/"))

	assert.Zero(t, hits.Load(), "private server must never be contacted")
}

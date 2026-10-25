package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	repomocks "github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeResolver struct {
	res *service.ResolvedLink
	err error
}

func (f *fakeResolver) Resolve(_ context.Context, _, _ string) (*service.ResolvedLink, error) {
	return f.res, f.err
}

type fakeClickRecorder struct {
	events chan service.ClickEvent
}

func newFakeClickRecorder() *fakeClickRecorder {
	return &fakeClickRecorder{events: make(chan service.ClickEvent, 4)}
}

func (f *fakeClickRecorder) RecordClick(_ context.Context, ev service.ClickEvent) error {
	f.events <- ev
	return nil
}

func (f *fakeClickRecorder) next(t *testing.T) service.ClickEvent {
	t.Helper()
	select {
	case ev := <-f.events:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for click event")
		return service.ClickEvent{}
	}
}

func newRedirectTestRouter(t *testing.T, resolver LinkResolver, rec ClickRecorder) (*gin.Engine, *repomocks.MockDomainRepository) {
	gin.SetMode(gin.TestMode)
	domains := repomocks.NewMockDomainRepository(gomock.NewController(t))
	h := NewRedirectHandler(resolver, rec, domains, "salt", "https://app.example.com", "sho.rt")
	r := gin.New()
	r.GET("/", h.Root)
	r.GET("/:code", h.Redirect)
	r.GET("/m/:slug", h.PageHop)
	r.HEAD("/m/:slug", h.PageHop)
	return r, domains
}

func doGet(r *gin.Engine, host, path, ua string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRedirectHandler_SourceQR(t *testing.T) {
	rec := newFakeClickRecorder()
	r, _ := newRedirectTestRouter(t, &fakeResolver{res: &service.ResolvedLink{LinkID: 5, LongURL: "https://example.com/dest"}}, rec)

	w := doGet(r, "sho.rt", "/abc?r=qr&utm_source=news", "")
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://example.com/dest", w.Header().Get("Location"))

	ev := rec.next(t)
	assert.Equal(t, uint64(5), ev.LinkID)
	assert.Equal(t, "qr", ev.Source)
	assert.Equal(t, "news", ev.UTMSource)

	doGet(r, "sho.rt", "/abc?r=page", "")
	assert.Equal(t, "page", rec.next(t).Source)
	doGet(r, "sho.rt", "/abc?r=bogus", "")
	assert.Equal(t, "link", rec.next(t).Source)
}

func TestRedirectHandler_RoutingRuleApplied(t *testing.T) {
	rec := newFakeClickRecorder()
	res := &service.ResolvedLink{LinkID: 5, LongURL: "https://example.com/default", Rules: []model.RoutingRule{
		{LongURL: "https://apps.apple.com/app", OSMatch: model.StringList{"ios"}},
	}}
	r, _ := newRedirectTestRouter(t, &fakeResolver{res: res}, rec)

	iphone := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	w := doGet(r, "sho.rt", "/abc", iphone)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://apps.apple.com/app", w.Header().Get("Location"))
	rec.next(t)

	windows := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	w = doGet(r, "sho.rt", "/abc", windows)
	assert.Equal(t, "https://example.com/default", w.Header().Get("Location"))
	rec.next(t)
}

func TestRedirectHandler_ErrorPages(t *testing.T) {
	cases := []struct {
		name     string
		resolver *fakeResolver
		status   int
		heading  string
		location string
	}{
		{"not found", &fakeResolver{err: service.ErrLinkNotFound}, http.StatusNotFound, "Link not found", ""},
		{"expired", &fakeResolver{res: &service.ResolvedLink{LinkID: 1}, err: service.ErrLinkExpired}, http.StatusGone, "This link has expired", ""},
		{"expired with redirect", &fakeResolver{res: &service.ResolvedLink{LinkID: 1, ExpiredRedirect: "https://brand.example.com/404"}, err: service.ErrLinkExpired}, http.StatusFound, "", "https://brand.example.com/404"},
		{"inactive", &fakeResolver{err: service.ErrLinkInactive}, http.StatusGone, "This link is no longer available", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := newRedirectTestRouter(t, tc.resolver, newFakeClickRecorder())
			w := doGet(r, "sho.rt", "/abc", "")
			require.Equal(t, tc.status, w.Code)
			if tc.location != "" {
				assert.Equal(t, tc.location, w.Header().Get("Location"))
				return
			}
			assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
			body := w.Body.String()
			assert.Contains(t, body, ">"+tc.heading+"</h1>")
			assert.Contains(t, body, `href="https://app.example.com"`)
			assert.Contains(t, body, "#F4F6FA")
		})
	}
}

func TestRedirectHandler_RootRedirect(t *testing.T) {
	r, domains := newRedirectTestRouter(t, &fakeResolver{}, newFakeClickRecorder())
	root := "https://brand.example.com"
	domains.EXPECT().GetByDomain(gomock.Any(), "go.brand.com").Return(&model.Domain{ID: 1, Domain: "go.brand.com", RootRedirect: &root, VerificationStatus: model.DomainVerificationOK}, nil)
	// sho.rt is reserved: no domain lookup.

	w := doGet(r, "go.brand.com:8080", "/", "")
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, root, w.Header().Get("Location"))

	w = doGet(r, "sho.rt", "/", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Link not found")
}

func TestRedirectHandler_Root_NormalizesHost(t *testing.T) {
	r, domains := newRedirectTestRouter(t, &fakeResolver{}, newFakeClickRecorder())
	root := "https://brand.example.com"
	// Mixed case and a trailing dot are looked up as the stored lowercase name.
	domains.EXPECT().GetByDomain(gomock.Any(), "go.brand.com").Return(&model.Domain{ID: 1, Domain: "go.brand.com",
		RootRedirect: &root, VerificationStatus: model.DomainVerificationOK}, nil)

	w := doGet(r, "Go.Brand.COM.:8080", "/", "")
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, root, w.Header().Get("Location"))
}

func TestRedirectHandler_Root_UnverifiedDomainIgnored(t *testing.T) {
	r, domains := newRedirectTestRouter(t, &fakeResolver{}, newFakeClickRecorder())
	root := "https://evil.example.com"
	domains.EXPECT().GetByDomain(gomock.Any(), "go.brand.com").Return(&model.Domain{ID: 1, Domain: "go.brand.com",
		RootRedirect: &root, VerificationStatus: model.DomainVerificationPending}, nil)

	w := doGet(r, "go.brand.com", "/", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
}

func TestRedirectHandler_Root_ReservedHostNoRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// No GetByDomain expectation: reserved hosts are never looked up, so a
	// legacy "ok" row for them cannot hijack the root redirect.
	domains := repomocks.NewMockDomainRepository(gomock.NewController(t))
	h := NewRedirectHandler(&fakeResolver{}, newFakeClickRecorder(), domains, "salt", "https://App.Example.com:3000", "sho.rt", "r.sho.rt")
	r := gin.New()
	r.GET("/", h.Root)
	for _, host := range []string{"sho.rt", "R.SHO.RT:8080", "app.example.com"} {
		w := doGet(r, host, "/", "")
		assert.Equal(t, http.StatusNotFound, w.Code, host)
		assert.Empty(t, w.Header().Get("Location"), host)
	}
}

func TestRedirectHandler_PageHop(t *testing.T) {
	// The hop does no lookup: it only validates the slug format.
	r, _ := newRedirectTestRouter(t, &fakeResolver{err: service.ErrLinkNotFound}, newFakeClickRecorder())

	w := doGet(r, "sho.rt", "/m/jose", "")
	require.Equal(t, http.StatusFound, w.Code, w.Body.String())
	assert.Equal(t, "https://app.example.com/m/jose", w.Header().Get("Location"))

	req := httptest.NewRequest(http.MethodHead, "/m/jose", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://app.example.com/m/jose", w.Header().Get("Location"))

	w = doGet(r, "sho.rt", "/m/Jose-2?x=1", "")
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "https://app.example.com/m/jose-2", w.Header().Get("Location"))

	for _, bad := range []string{"/m/ab", "/m/-jose", "/m/jose-", "/m/jo_se", "/m/" + strings.Repeat("a", 41), "/m/api", "/m/%2e%2e"} {
		w = doGet(r, "sho.rt", bad, "")
		assert.Equal(t, http.StatusNotFound, w.Code, bad)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html", bad)
		assert.Contains(t, w.Body.String(), ">Page not found</h1>", bad)
	}

	// /m alone and other one-segment paths still reach the link redirect.
	w = doGet(r, "sho.rt", "/m", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), ">Link not found</h1>")

	// A trailing slash on the frontend URL is not doubled.
	gin.SetMode(gin.TestMode)
	h := NewRedirectHandler(&fakeResolver{}, newFakeClickRecorder(), repomocks.NewMockDomainRepository(gomock.NewController(t)),
		"salt", "http://localhost:3000/", "sho.rt")
	r2 := gin.New()
	r2.GET("/m/:slug", h.PageHop)
	w = doGet(r2, "sho.rt", "/m/jose", "")
	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "http://localhost:3000/m/jose", w.Header().Get("Location"))
}

// blockingClickRecorder holds RecordClick until release is closed.
type blockingClickRecorder struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingClickRecorder) RecordClick(_ context.Context, _ service.ClickEvent) error {
	close(b.started)
	<-b.release
	return nil
}

func TestRedirectHandler_WaitBlocksUntilRecordCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := &blockingClickRecorder{started: make(chan struct{}), release: make(chan struct{})}
	domains := repomocks.NewMockDomainRepository(gomock.NewController(t))
	h := NewRedirectHandler(&fakeResolver{res: &service.ResolvedLink{LinkID: 1, LongURL: "https://example.com"}},
		rec, domains, "salt", "https://app.example.com", "sho.rt")
	r := gin.New()
	r.GET("/:code", h.Redirect)

	w := doGet(r, "sho.rt", "/abc", "")
	require.Equal(t, http.StatusFound, w.Code)
	select {
	case <-rec.started:
	case <-time.After(2 * time.Second):
		t.Fatal("click was never recorded")
	}

	waited := make(chan struct{})
	go func() { h.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("Wait returned while a click record was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	close(rec.release)
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after the record completed")
	}
}

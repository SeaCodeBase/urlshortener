package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
)

func TestInvalidateCache(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	s := &RedirectService{rdb: rdb}
	ctx := context.Background()

	// Set a cache entry with host in key
	host := "example.com"
	code := "abc123"
	cacheKey := linkCacheKeyPrefix + host + ":" + code
	mr.Set(cacheKey, `{"url":"https://test.com","is_active":true}`)

	// Invalidate should remove the entry
	err = s.InvalidateCache(ctx, host, code)
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	// Verify key is deleted
	if mr.Exists(cacheKey) {
		t.Error("cache key should have been deleted")
	}
}

func TestInvalidateCache_CustomDomain(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	s := &RedirectService{rdb: rdb}
	ctx := context.Background()

	// Simulate cached link for custom domain
	host := "short.example.com"
	code := "xyz789"
	cacheKey := linkCacheKeyPrefix + host + ":" + code
	mr.Set(cacheKey, `{"url":"https://original.com","is_active":true,"link_id":123}`)

	// Verify it exists
	if !mr.Exists(cacheKey) {
		t.Fatal("test setup: cache key should exist")
	}

	// Invalidate
	err = s.InvalidateCache(ctx, host, code)
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	// Verify deleted
	if mr.Exists(cacheKey) {
		t.Error("cache key should have been deleted after invalidation")
	}
}

func TestInvalidateCache_NonExistentKey(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	s := &RedirectService{rdb: rdb}
	ctx := context.Background()

	// Invalidating non-existent key should not error
	err = s.InvalidateCache(ctx, "nonexistent.com", "missing")
	if err != nil {
		t.Errorf("InvalidateCache should not error for non-existent key: %v", err)
	}
}

type redirectFixture struct {
	svc     *RedirectService
	links   *mocks.MockLinkRepository
	domains *mocks.MockDomainRepository
	rules   *mocks.MockRoutingRuleRepository
	mr      *miniredis.Miniredis
}

func newRedirectFixture(t *testing.T) *redirectFixture {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctrl := gomock.NewController(t)
	f := &redirectFixture{
		links:   mocks.NewMockLinkRepository(ctrl),
		domains: mocks.NewMockDomainRepository(ctrl),
		rules:   mocks.NewMockRoutingRuleRepository(ctrl),
		mr:      mr,
	}
	f.svc = NewRedirectService(f.links, f.domains, f.rules, rdb, "sho.rt")
	return f
}

func TestRedirectService_ArchivedStillRedirects(t *testing.T) {
	f := newRedirectFixture(t)
	ctx := context.Background()

	rule := model.RoutingRule{ID: 1, LinkID: 7, LongURL: "https://ios.example.com", OSMatch: model.StringList{"ios"}}
	// sho.rt is the base host: reserved, so no domain lookup happens.
	f.links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "abc").Return(&model.Link{
		ID: 7, ShortCode: "abc", OriginalURL: "https://example.com/archived", IsActive: true, Archived: true,
	}, nil).Times(1)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(7)).Return([]model.RoutingRule{rule}, nil).Times(1)

	// First call: DB; second call: served from cache (mocks allow one call each).
	for i := 0; i < 2; i++ {
		res, err := f.svc.Resolve(ctx, "sho.rt:8080", "abc")
		if err != nil {
			t.Fatalf("call %d: archived link must still redirect, got %v", i, err)
		}
		if res.LinkID != 7 || res.LongURL != "https://example.com/archived" || res.DomainID != nil {
			t.Fatalf("call %d: unexpected resolved link %+v", i, res)
		}
		if len(res.Rules) != 1 || res.Rules[0].LongURL != rule.LongURL {
			t.Fatalf("call %d: expected routing rules to be returned, got %+v", i, res.Rules)
		}
	}

	cached, err := f.mr.Get(linkCacheKeyPrefix + "sho.rt:abc")
	if err != nil {
		t.Fatalf("expected cache entry under v2 key: %v", err)
	}
	for _, field := range []string{`"archived":true`, `"rules":[`} {
		if !strings.Contains(cached, field) {
			t.Errorf("cache JSON %s missing %s", cached, field)
		}
	}
}

func TestRedirectService_ExpiredUsesDomain404(t *testing.T) {
	f := newRedirectFixture(t)
	ctx := context.Background()

	notFound := "https://brand.example.com/oops"
	f.domains.EXPECT().GetByDomain(gomock.Any(), "go.brand.com").
		Return(&model.Domain{ID: 3, Domain: "go.brand.com", NotFoundRedirect: &notFound,
			VerificationStatus: model.DomainVerificationOK}, nil).Times(1)
	domainID := uint64(3)
	f.links.EXPECT().GetByDomainAndShortCode(gomock.Any(), &domainID, "old").Return(&model.Link{
		ID: 9, ShortCode: "old", OriginalURL: "https://example.com/old", IsActive: true, DomainID: &domainID,
		ExpiresAt: model.NullTime{NullTime: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}},
	}, nil).Times(1)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(9)).Return(nil, nil).Times(1)

	for i := 0; i < 2; i++ {
		res, err := f.svc.Resolve(ctx, "go.brand.com", "old")
		if !errors.Is(err, ErrLinkExpired) {
			t.Fatalf("call %d: expected ErrLinkExpired, got %v", i, err)
		}
		if res == nil || res.ExpiredRedirect != notFound {
			t.Fatalf("call %d: expected ExpiredRedirect %q, got %+v", i, notFound, res)
		}
	}

	cached, _ := f.mr.Get(linkCacheKeyPrefix + "go.brand.com:old")
	for _, field := range []string{`"expires_at":`, `"not_found_redirect":"` + notFound + `"`} {
		if !strings.Contains(cached, field) {
			t.Errorf("cache JSON %s missing %s", cached, field)
		}
	}
}

func TestRedirectService_NotFoundAndInactive(t *testing.T) {
	f := newRedirectFixture(t)
	ctx := context.Background()

	f.domains.EXPECT().GetByDomain(gomock.Any(), gomock.Any()).Return(nil, repository.ErrDomainNotFound).AnyTimes()
	f.links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "nope").Return(nil, repository.ErrLinkNotFound)
	f.links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "off").
		Return(&model.Link{ID: 2, OriginalURL: "https://example.com", IsActive: false}, nil)
	f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(2)).Return(nil, nil)

	if _, err := f.svc.Resolve(ctx, "sho.rt", "nope"); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("expected ErrLinkNotFound, got %v", err)
	}
	if _, err := f.svc.Resolve(ctx, "sho.rt", "off"); !errors.Is(err, ErrLinkInactive) {
		t.Fatalf("expected ErrLinkInactive, got %v", err)
	}
}

func TestRedirectService_InvalidateLink(t *testing.T) {
	f := newRedirectFixture(t)
	ctx := context.Background()

	f.mr.Set(linkCacheKeyPrefix+"sho.rt:abc", "{}")
	f.mr.Set(linkCacheKeyPrefix+"go.brand.com:abc", "{}")
	f.mr.Set(linkCacheKeyPrefix+"sho.rt:other", "{}")

	if err := f.svc.InvalidateLink(ctx, &model.Link{ShortCode: "abc"}, "go.brand.com"); err != nil {
		t.Fatalf("InvalidateLink: %v", err)
	}
	if f.mr.Exists(linkCacheKeyPrefix+"sho.rt:abc") || f.mr.Exists(linkCacheKeyPrefix+"go.brand.com:abc") {
		t.Error("expected default and custom domain keys to be deleted")
	}
	if !f.mr.Exists(linkCacheKeyPrefix + "sho.rt:other") {
		t.Error("unrelated key must survive")
	}

	f.mr.Set(linkCacheKeyPrefix+"sho.rt:abc", "{}")
	if err := f.svc.InvalidateLink(ctx, &model.Link{ShortCode: "abc"}, ""); err != nil {
		t.Fatalf("InvalidateLink default: %v", err)
	}
	if f.mr.Exists(linkCacheKeyPrefix + "sho.rt:abc") {
		t.Error("expected default-domain key to be deleted")
	}
}

func TestRedirectService_UnverifiedDomainNotResolved(t *testing.T) {
	for _, status := range []string{model.DomainVerificationPending, model.DomainVerificationError} {
		f := newRedirectFixture(t)
		ctx := context.Background()

		notFound := "https://evil.example.com/"
		f.domains.EXPECT().GetByDomain(gomock.Any(), "go.brand.com").
			Return(&model.Domain{ID: 3, Domain: "go.brand.com", NotFoundRedirect: &notFound, VerificationStatus: status}, nil).AnyTimes()
		// The host is treated as unknown: lookups go to the default domain
		// (nil), never to domain 3, and its not_found_redirect is ignored.
		f.links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "nope").Return(nil, repository.ErrLinkNotFound)
		res, err := f.svc.Resolve(ctx, "go.brand.com", "nope")
		if !errors.Is(err, ErrLinkNotFound) {
			t.Fatalf("%s: expected ErrLinkNotFound, got %v", status, err)
		}
		if res != nil && res.ExpiredRedirect != "" {
			t.Fatalf("%s: unverified domain's not_found_redirect must not be used, got %+v", status, res)
		}

		// A default-domain code resolves but is never cached under the
		// unverified host, so verifying the domain later has no stale entry.
		f.links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "abc").
			Return(&model.Link{ID: 7, ShortCode: "abc", OriginalURL: "https://example.com", IsActive: true}, nil)
		f.rules.EXPECT().ListByLinkID(gomock.Any(), uint64(7)).Return(nil, nil)
		if _, err := f.svc.Resolve(ctx, "go.brand.com", "abc"); err != nil {
			t.Fatalf("%s: unexpected error %v", status, err)
		}
		if f.mr.Exists(linkCacheKeyPrefix + "go.brand.com:abc") {
			t.Errorf("%s: nothing may be cached under an unverified domain host", status)
		}
	}
}

func TestRedirectService_ReservedHostIsDefaultDomain(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctrl := gomock.NewController(t)
	links := mocks.NewMockLinkRepository(ctrl)
	// No GetByDomain expectation: a reserved host must never be looked up,
	// even if a (legacy, "ok") domain row exists for it.
	domains := mocks.NewMockDomainRepository(ctrl)
	rules := mocks.NewMockRoutingRuleRepository(ctrl)
	svc := NewRedirectService(links, domains, rules, rdb, "sho.rt", "R.Sho.RT.", "app.example.com:3000")

	for _, host := range []string{"sho.rt:8080", "r.sho.rt", "APP.example.com"} {
		links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "nope").Return(nil, repository.ErrLinkNotFound)
		res, err := svc.Resolve(context.Background(), host, "nope")
		if !errors.Is(err, ErrLinkNotFound) {
			t.Fatalf("%s: expected ErrLinkNotFound, got %v", host, err)
		}
		if res != nil && res.ExpiredRedirect != "" {
			t.Fatalf("%s: reserved host must not use a domain's not_found_redirect: %+v", host, res)
		}
	}

	links.EXPECT().GetByDomainAndShortCode(gomock.Any(), nil, "abc").
		Return(&model.Link{ID: 7, ShortCode: "abc", OriginalURL: "https://example.com", IsActive: true}, nil)
	rules.EXPECT().ListByLinkID(gomock.Any(), uint64(7)).Return(nil, nil)
	res, err := svc.Resolve(context.Background(), "r.sho.rt", "abc")
	if err != nil || res.LinkID != 7 || res.DomainID != nil {
		t.Fatalf("reserved host resolves default-domain links: %+v %v", res, err)
	}
}

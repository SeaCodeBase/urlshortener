// backend/internal/service/redirect_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	ErrLinkExpired  = errors.New("link has expired")
	ErrLinkInactive = errors.New("link is not active")
)

const (
	linkCacheTTL = 1 * time.Hour
	// v2 entries carry archived/rules/not_found_redirect; the version bump makes
	// stale v1 entries ("link:{host}:{code}") invisible.
	linkCacheKeyPrefix = "link:v2:"
	// domainCacheKeyPrefix caches host -> custom-domain lookups so cache
	// hits stay DB-free; DomainService invalidates it on verify/update/delete.
	domainCacheKeyPrefix = "domain:v1:"
)

// ResolvedLink is what the redirect server needs to serve a short link.
// ExpiredRedirect is the domain's not_found_redirect; it is populated (with
// ErrLinkExpired or ErrLinkNotFound) when the visitor should be sent there.
type ResolvedLink struct {
	LinkID          uint64
	LongURL         string
	DomainID        *uint64
	Rules           []model.RoutingRule
	ExpiredRedirect string
}

type RedirectService struct {
	linkRepo   repository.LinkRepository
	domainRepo repository.DomainRepository
	ruleRepo   repository.RoutingRuleRepository
	rdb        *redis.Client
	baseHost   string
	reserved   map[string]bool
}

// NewRedirectService builds the service. baseHost is the default short domain
// host (from urls.base_url, without port) used to invalidate default-domain
// links. baseHost and reservedHosts (redirect host, frontend host) are always
// served as the default domain: no domain row is consulted for them, so a
// legacy row for one of the service's own hosts cannot hijack it.
func NewRedirectService(linkRepo repository.LinkRepository, domainRepo repository.DomainRepository,
	ruleRepo repository.RoutingRuleRepository, rdb *redis.Client, baseHost string, reservedHosts ...string) *RedirectService {
	return &RedirectService{
		linkRepo:   linkRepo,
		domainRepo: domainRepo,
		ruleRepo:   ruleRepo,
		rdb:        rdb,
		baseHost:   normalizeHost(stripPort(baseHost)),
		reserved:   ReservedHostSet(append([]string{baseHost}, reservedHosts...)...),
	}
}

type cachedLink struct {
	OriginalURL      string              `json:"url"`
	LinkID           uint64              `json:"link_id"`
	DomainID         *uint64             `json:"domain_id,omitempty"`
	IsActive         bool                `json:"is_active"`
	Archived         bool                `json:"archived"`
	ExpiresAt        *time.Time          `json:"expires_at,omitempty"`
	Rules            []model.RoutingRule `json:"rules"`
	NotFoundRedirect string              `json:"not_found_redirect,omitempty"`
}

// stripPort removes a trailing ":port" (e.g. "example.com:8080" -> "example.com").
func stripPort(host string) string {
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		return host[:colonIdx]
	}
	return host
}

func cacheKey(host, code string) string {
	return linkCacheKeyPrefix + host + ":" + code
}

// cachedDomain is the cached answer for a host: Verified false means the host
// is served as the default domain (unknown or unverified domain).
type cachedDomain struct {
	Verified         bool   `json:"verified"`
	ID               uint64 `json:"id,omitempty"`
	NotFoundRedirect string `json:"not_found_redirect,omitempty"`
}

// canonicalHost maps a request host to the host its links are cached under
// and the custom domain (nil = default domain) they belong to. Reserved,
// unknown and unverified hosts all map to the base host, so InvalidateLink's
// base-host key covers every default-domain entry.
func (s *RedirectService) canonicalHost(ctx context.Context, host string) (string, *uint64, string, error) {
	h := normalizeHost(stripPort(host))
	if h == "" || s.reserved[h] {
		return s.baseHost, nil, "", nil
	}
	var info cachedDomain
	cached := false
	if raw, err := s.rdb.Get(ctx, domainCacheKeyPrefix+h).Result(); err == nil {
		cached = json.Unmarshal([]byte(raw), &info) == nil
	}
	if !cached {
		domain, err := s.domainRepo.GetByDomain(ctx, h)
		switch {
		case errors.Is(err, repository.ErrDomainNotFound):
		case err != nil:
			return "", nil, "", err
		case domain.VerificationStatus == model.DomainVerificationOK:
			info = cachedDomain{Verified: true, ID: domain.ID}
			if domain.NotFoundRedirect != nil {
				info.NotFoundRedirect = *domain.NotFoundRedirect
			}
		}
		if data, err := json.Marshal(info); err == nil {
			if err := s.rdb.Set(ctx, domainCacheKeyPrefix+h, data, linkCacheTTL).Err(); err != nil {
				logger.Warn(ctx, "failed to cache domain", zap.String("host", h), zap.Error(err))
			}
		}
	}
	if !info.Verified {
		return s.baseHost, nil, "", nil
	}
	id := info.ID
	return h, &id, info.NotFoundRedirect, nil
}

// InvalidateDomain drops the cached host lookup for a custom domain.
func (s *RedirectService) InvalidateDomain(ctx context.Context, host string) error {
	return s.rdb.Del(ctx, domainCacheKeyPrefix+normalizeHost(stripPort(host))).Err()
}

// Resolve looks up the link for host+code (cache first). Archived links still
// resolve. It returns ErrLinkNotFound, ErrLinkInactive or ErrLinkExpired; for
// expired links (and unknown codes on a custom domain) the returned
// ResolvedLink carries ExpiredRedirect when the domain has not_found_redirect.
// Reserved hosts (the service's own) and unknown or unverified hosts are
// served as the default domain and share the base host's cache entries.
func (s *RedirectService) Resolve(ctx context.Context, host, code string) (*ResolvedLink, error) {
	canon, domainID, notFoundRedirect, err := s.canonicalHost(ctx, host)
	if err != nil {
		return nil, err
	}
	key := cacheKey(canon, code)

	if cached, err := s.rdb.Get(ctx, key).Result(); err == nil {
		var cl cachedLink
		if err := json.Unmarshal([]byte(cached), &cl); err == nil {
			// The domain's current not_found_redirect wins over the copy in
			// the link entry (domain settings invalidate only the domain key).
			cl.NotFoundRedirect = notFoundRedirect
			return validate(cl)
		}
	}

	link, err := s.linkRepo.GetByDomainAndShortCode(ctx, domainID, code)
	if errors.Is(err, repository.ErrLinkNotFound) {
		if notFoundRedirect != "" {
			return &ResolvedLink{DomainID: domainID, ExpiredRedirect: notFoundRedirect}, ErrLinkNotFound
		}
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, err
	}

	var rules []model.RoutingRule
	if s.ruleRepo != nil {
		rules, err = s.ruleRepo.ListByLinkID(ctx, link.ID)
		if err != nil {
			return nil, err
		}
	}

	cl := cachedLink{
		OriginalURL:      link.OriginalURL,
		LinkID:           link.ID,
		DomainID:         link.DomainID,
		IsActive:         link.IsActive,
		Archived:         link.Archived,
		Rules:            rules,
		NotFoundRedirect: notFoundRedirect,
	}
	if link.ExpiresAt.Valid {
		t := link.ExpiresAt.Time
		cl.ExpiresAt = &t
	}

	if data, err := json.Marshal(cl); err != nil {
		logger.Warn(ctx, "failed to marshal cached link",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
	} else if err := s.rdb.Set(ctx, key, data, linkCacheTTL).Err(); err != nil {
		logger.Warn(ctx, "failed to cache link",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
	}

	return validate(cl)
}

func validate(cl cachedLink) (*ResolvedLink, error) {
	if !cl.IsActive {
		return nil, ErrLinkInactive
	}
	if cl.ExpiresAt != nil && cl.ExpiresAt.Before(time.Now()) {
		return &ResolvedLink{LinkID: cl.LinkID, DomainID: cl.DomainID, ExpiredRedirect: cl.NotFoundRedirect}, ErrLinkExpired
	}
	return &ResolvedLink{
		LinkID:   cl.LinkID,
		LongURL:  cl.OriginalURL,
		DomainID: cl.DomainID,
		Rules:    cl.Rules,
	}, nil
}

func (s *RedirectService) InvalidateCache(ctx context.Context, host, code string) error {
	return s.rdb.Del(ctx, cacheKey(host, code)).Err()
}

// InvalidateLink drops the cached entries for link: the default-domain key
// (base host) and, when domainName is non-empty, the custom-domain key.
func (s *RedirectService) InvalidateLink(ctx context.Context, link *model.Link, domainName string) error {
	keys := []string{}
	if s.baseHost != "" {
		keys = append(keys, cacheKey(s.baseHost, link.ShortCode))
	}
	if domainName != "" {
		keys = append(keys, cacheKey(normalizeHost(stripPort(domainName)), link.ShortCode))
	}
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

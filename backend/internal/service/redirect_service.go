// backend/internal/service/redirect_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
	linkCacheTTL       = 1 * time.Hour
	linkCacheKeyPrefix = "link:"
)

type RedirectService struct {
	linkRepo   repository.LinkRepository
	domainRepo repository.DomainRepository
	rdb        *redis.Client
}

func NewRedirectService(linkRepo repository.LinkRepository, domainRepo repository.DomainRepository, rdb *redis.Client) *RedirectService {
	return &RedirectService{
		linkRepo:   linkRepo,
		domainRepo: domainRepo,
		rdb:        rdb,
	}
}

type cachedLink struct {
	OriginalURL string    `json:"url"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	IsActive    bool      `json:"is_active"`
	LinkID      uint64    `json:"link_id"`
}

func (s *RedirectService) Resolve(ctx context.Context, host, code string) (string, uint64, error) {
	// Strip port from host if present (e.g., "example.com:8080" -> "example.com")
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Determine domain ID from host
	var domainID *uint64
	domain, err := s.domainRepo.GetByDomain(ctx, host)
	if err == nil {
		domainID = &domain.ID
	}
	// If domain not found, domainID stays nil (default domain)

	// Try cache first (include domain in cache key)
	cacheKey := linkCacheKeyPrefix + host + ":" + code
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var cl cachedLink
		if err := json.Unmarshal([]byte(cached), &cl); err == nil {
			return s.validateAndReturn(cl)
		}
	}

	// Cache miss - query DB by domain and short code
	link, err := s.linkRepo.GetByDomainAndShortCode(ctx, domainID, code)
	if errors.Is(err, repository.ErrLinkNotFound) {
		return "", 0, ErrLinkNotFound
	}
	if err != nil {
		return "", 0, err
	}

	// Cache the result
	cl := cachedLink{
		OriginalURL: link.OriginalURL,
		IsActive:    link.IsActive,
		LinkID:      link.ID,
	}
	if link.ExpiresAt.Valid {
		cl.ExpiresAt = link.ExpiresAt.Time
	}

	data, err := json.Marshal(cl)
	if err != nil {
		logger.Warn(ctx, "failed to marshal cached link",
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
	} else {
		s.rdb.Set(ctx, cacheKey, data, linkCacheTTL)
	}

	return s.validateAndReturn(cl)
}

func (s *RedirectService) validateAndReturn(cl cachedLink) (string, uint64, error) {
	if !cl.IsActive {
		return "", 0, ErrLinkInactive
	}
	if !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Before(time.Now()) {
		return "", 0, ErrLinkExpired
	}
	return cl.OriginalURL, cl.LinkID, nil
}

func (s *RedirectService) InvalidateCache(ctx context.Context, code string) error {
	return s.rdb.Del(ctx, linkCacheKeyPrefix+code).Err()
}

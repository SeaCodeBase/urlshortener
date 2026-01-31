// backend/internal/service/redirect_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/redis/go-redis/v9"
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
	linkRepo repository.LinkRepository
	rdb      *redis.Client
}

func NewRedirectService(linkRepo repository.LinkRepository, rdb *redis.Client) *RedirectService {
	return &RedirectService{
		linkRepo: linkRepo,
		rdb:      rdb,
	}
}

type cachedLink struct {
	OriginalURL string    `json:"url"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	IsActive    bool      `json:"is_active"`
	LinkID      uint64    `json:"link_id"`
}

func (s *RedirectService) Resolve(ctx context.Context, code string) (string, uint64, error) {
	// Try cache first
	cacheKey := linkCacheKeyPrefix + code
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var cl cachedLink
		if err := json.Unmarshal([]byte(cached), &cl); err == nil {
			return s.validateAndReturn(cl)
		}
	}

	// Cache miss - query DB
	link, err := s.linkRepo.GetByShortCode(ctx, code)
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
		logger.Log.Warnf("failed to marshal cached link: %v", err)
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

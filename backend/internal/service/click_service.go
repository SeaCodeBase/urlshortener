// backend/internal/service/click_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type ClickEvent struct {
	LinkID      uint64    `json:"link_id"`
	ClickedAt   time.Time `json:"clicked_at"`
	IPHash      string    `json:"ip_hash"`
	UserAgent   string    `json:"user_agent"`
	Referrer    string    `json:"referrer"`
	UTMSource   string    `json:"utm_source,omitempty"`
	UTMMedium   string    `json:"utm_medium,omitempty"`
	UTMCampaign string    `json:"utm_campaign,omitempty"`
}

type ClickService struct {
	rdb *redis.Client
}

func NewClickService(rdb *redis.Client) *ClickService {
	return &ClickService{rdb: rdb}
}

func (s *ClickService) RecordClick(ctx context.Context, event ClickEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Buffer click event in Redis list
	bufferKey := "clicks:buffer"
	if err := s.rdb.LPush(ctx, bufferKey, data).Err(); err != nil {
		return err
	}

	// Increment real-time counter
	counterKey := fmt.Sprintf("clicks:count:%d", event.LinkID)
	if err := s.rdb.Incr(ctx, counterKey).Err(); err != nil {
		logger.Log.Warnf("failed to increment click counter: %v", err)
	}

	return nil
}

func (s *ClickService) GetRealtimeCount(ctx context.Context, linkID uint64) (int64, error) {
	counterKey := fmt.Sprintf("clicks:count:%d", linkID)
	return s.rdb.Get(ctx, counterKey).Int64()
}

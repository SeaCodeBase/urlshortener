// backend/internal/worker/click_flusher.go
package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/util"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ClickFlusher struct {
	rdb       *redis.Client
	clickRepo repository.ClickRepository
	interval  time.Duration
	batchSize int
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func NewClickFlusher(rdb *redis.Client, clickRepo repository.ClickRepository) *ClickFlusher {
	return &ClickFlusher{
		rdb:       rdb,
		clickRepo: clickRepo,
		interval:  30 * time.Second,
		batchSize: 100,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (f *ClickFlusher) Start() {
	go f.run()
}

func (f *ClickFlusher) Stop() {
	close(f.stopCh)
	<-f.doneCh // Wait for worker to finish
}

func (f *ClickFlusher) run() {
	defer close(f.doneCh) // Signal completion
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.flush()
		case <-f.stopCh:
			f.flush() // Final flush before stopping
			return
		}
	}
}

func (f *ClickFlusher) flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	bufferKey := "clicks:buffer"

	for {
		// Get batch of events
		events, err := f.rdb.LRange(ctx, bufferKey, 0, int64(f.batchSize-1)).Result()
		if err != nil {
			logger.Error(ctx, "failed to get click events from buffer",
			zap.Error(err),
		)
			return
		}

		if len(events) == 0 {
			return
		}

		// Parse events into clicks
		clicks := make([]model.Click, 0, len(events))
		for _, eventData := range events {
			var event service.ClickEvent
			if err := json.Unmarshal([]byte(eventData), &event); err != nil {
				logger.Warn(ctx, "failed to unmarshal click event",
				zap.Error(err),
			)
				continue
			}

			// Parse User-Agent
			uaResult := util.ParseUserAgent(event.UserAgent)

			// Lookup GeoIP
			geoResult := util.LookupIP(ctx, event.IPAddress)

			click := model.Click{
				LinkID:      event.LinkID,
				ClickedAt:   event.ClickedAt,
				IPHash:      event.IPHash,
				IPAddress:   event.IPAddress,
				UserAgent:   event.UserAgent,
				Referrer:    event.Referrer,
				Country:     geoResult.Country,
				City:        geoResult.City,
				DeviceType:  uaResult.DeviceType,
				Browser:     uaResult.Browser,
				UTMSource:   event.UTMSource,
				UTMMedium:   event.UTMMedium,
				UTMCampaign: event.UTMCampaign,
			}
			clicks = append(clicks, click)
		}

		// Insert into database
		if err := f.clickRepo.BatchInsert(ctx, clicks); err != nil {
			logger.Error(ctx, "failed to batch insert clicks",
			zap.Error(err),
		)
			return
		}

		// Remove processed events from buffer
		if err := f.rdb.LTrim(ctx, bufferKey, int64(len(events)), -1).Err(); err != nil {
			logger.Error(ctx, "failed to trim click buffer",
				zap.Error(err),
			)
		}

		logger.Info(ctx, "flushed click events to database",
			zap.Int("count", len(clicks)),
		)

		if len(events) < f.batchSize {
			return // No more events to process
		}
	}
}

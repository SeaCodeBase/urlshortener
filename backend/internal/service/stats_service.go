package service

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
)

// Compile-time check: StatsServiceImpl implements StatsService
var _ StatsService = (*StatsServiceImpl)(nil)

type StatsServiceImpl struct {
	clickRepo repository.ClickRepository
	linkRepo  repository.LinkRepository
}

func NewStatsService(clickRepo repository.ClickRepository, linkRepo repository.LinkRepository) *StatsServiceImpl {
	return &StatsServiceImpl{
		clickRepo: clickRepo,
		linkRepo:  linkRepo,
	}
}

type LinkStatsResponse struct {
	TotalClicks    int64                        `json:"total_clicks"`
	UniqueVisitors int64                        `json:"unique_visitors"`
	DailyStats     []repository.DailyClickStats `json:"daily_stats"`
	TopReferrers   []repository.ReferrerStats   `json:"top_referrers"`
	DeviceStats    []repository.DeviceStats     `json:"device_stats"`
	BrowserStats   []repository.BrowserStats    `json:"browser_stats"`
}

func (s *StatsServiceImpl) GetLinkStats(ctx context.Context, userID, linkID uint64) (*LinkStatsResponse, error) {
	// Verify ownership
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if err != nil {
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	// Get all stats
	stats, err := s.clickRepo.GetStatsByLinkID(ctx, linkID)
	if err != nil {
		return nil, err
	}

	daily, err := s.clickRepo.GetDailyStats(ctx, linkID, 30)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if daily == nil {
		daily = []repository.DailyClickStats{}
	}

	referrers, err := s.clickRepo.GetTopReferrers(ctx, linkID, 10)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if referrers == nil {
		referrers = []repository.ReferrerStats{}
	}

	devices, err := s.clickRepo.GetDeviceStats(ctx, linkID)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if devices == nil {
		devices = []repository.DeviceStats{}
	}

	browsers, err := s.clickRepo.GetBrowserStats(ctx, linkID)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if browsers == nil {
		browsers = []repository.BrowserStats{}
	}

	return &LinkStatsResponse{
		TotalClicks:    stats.TotalClicks,
		UniqueVisitors: stats.UniqueVisitors,
		DailyStats:     daily,
		TopReferrers:   referrers,
		DeviceStats:    devices,
		BrowserStats:   browsers,
	}, nil
}

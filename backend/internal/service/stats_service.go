package service

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
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

type LocationStats struct {
	Countries []repository.CountryStats `json:"countries"`
	Cities    []repository.CityStats    `json:"cities"`
}

type LinkStatsResponse struct {
	TotalClicks    int64                        `json:"total_clicks"`
	UniqueVisitors int64                        `json:"unique_visitors"`
	DailyStats     []repository.DailyClickStats `json:"daily_stats"`
	TopReferrers   []repository.ReferrerStats   `json:"top_referrers"`
	DeviceStats    []repository.DeviceStats     `json:"device_stats"`
	BrowserStats   []repository.BrowserStats    `json:"browser_stats"`
	Locations      LocationStats                `json:"locations"`
}

func (s *StatsServiceImpl) GetLinkStats(ctx context.Context, userID, linkID uint64) (*LinkStatsResponse, error) {
	// Verify ownership
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get link",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	// Get all stats
	stats, err := s.clickRepo.GetStatsByLinkID(ctx, linkID)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get click stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}

	daily, err := s.clickRepo.GetDailyStats(ctx, linkID, 30)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get daily stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if daily == nil {
		daily = []repository.DailyClickStats{}
	}

	referrers, err := s.clickRepo.GetTopReferrers(ctx, linkID, 10)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get top referrers",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if referrers == nil {
		referrers = []repository.ReferrerStats{}
	}

	devices, err := s.clickRepo.GetDeviceStats(ctx, linkID)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get device stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if devices == nil {
		devices = []repository.DeviceStats{}
	}

	browsers, err := s.clickRepo.GetBrowserStats(ctx, linkID)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get browser stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if browsers == nil {
		browsers = []repository.BrowserStats{}
	}

	countries, err := s.clickRepo.GetCountryStats(ctx, linkID, 10)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get country stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	if countries == nil {
		countries = []repository.CountryStats{}
	}
	// Calculate percentages and set country names
	for i := range countries {
		if stats.TotalClicks > 0 {
			countries[i].Percentage = float64(countries[i].Count) / float64(stats.TotalClicks) * 100
		}
		countries[i].CountryName = getCountryName(countries[i].Country)
	}

	cities, err := s.clickRepo.GetCityStats(ctx, linkID, 10)
	if err != nil {
		logger.Error(ctx, "stats-service: failed to get city stats",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	if cities == nil {
		cities = []repository.CityStats{}
	}
	// Calculate percentages
	for i := range cities {
		if stats.TotalClicks > 0 {
			cities[i].Percentage = float64(cities[i].Count) / float64(stats.TotalClicks) * 100
		}
	}

	return &LinkStatsResponse{
		TotalClicks:    stats.TotalClicks,
		UniqueVisitors: stats.UniqueVisitors,
		DailyStats:     daily,
		TopReferrers:   referrers,
		DeviceStats:    devices,
		BrowserStats:   browsers,
		Locations: LocationStats{
			Countries: countries,
			Cities:    cities,
		},
	}, nil
}

func getCountryName(code string) string {
	names := map[string]string{
		"CN": "China", "US": "United States", "JP": "Japan", "GB": "United Kingdom",
		"DE": "Germany", "FR": "France", "KR": "South Korea", "IN": "India",
		"BR": "Brazil", "RU": "Russia", "CA": "Canada", "AU": "Australia",
		"ES": "Spain", "IT": "Italy", "MX": "Mexico", "NL": "Netherlands",
		"Unknown": "Unknown",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}

package repository

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

//go:generate mockgen -destination=mocks/mock_user_repo.go -package=mocks . UserRepository
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, userID uint64, passwordHash string) error
	UpdateDisplayName(ctx context.Context, userID uint64, displayName string) error
}

//go:generate mockgen -destination=mocks/mock_link_repo.go -package=mocks . LinkRepository
type LinkRepository interface {
	Create(ctx context.Context, link *model.Link) error
	GetByID(ctx context.Context, id uint64) (*model.Link, error)
	GetByShortCode(ctx context.Context, shortCode string) (*model.Link, error)
	ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error)
	CountByUserID(ctx context.Context, userID uint64) (int64, error)
	Update(ctx context.Context, link *model.Link) error
	Delete(ctx context.Context, id uint64) error
	ShortCodeExists(ctx context.Context, shortCode string) (bool, error)
}

//go:generate mockgen -destination=mocks/mock_click_repo.go -package=mocks . ClickRepository
type ClickRepository interface {
	BatchInsert(ctx context.Context, clicks []model.Click) error
	GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error)
	GetStatsByLinkID(ctx context.Context, linkID uint64) (*ClickStats, error)
	GetDailyStats(ctx context.Context, linkID uint64, days int) ([]DailyClickStats, error)
	GetTopReferrers(ctx context.Context, linkID uint64, limit int) ([]ReferrerStats, error)
	GetDeviceStats(ctx context.Context, linkID uint64) ([]DeviceStats, error)
	GetBrowserStats(ctx context.Context, linkID uint64) ([]BrowserStats, error)
}

// Stats types used by ClickRepository
type ClickStats struct {
	TotalClicks    int64 `db:"total_clicks"`
	UniqueVisitors int64 `db:"unique_visitors"`
}

type DailyClickStats struct {
	Date   string `db:"date" json:"date"`
	Clicks int64  `db:"clicks" json:"clicks"`
}

type ReferrerStats struct {
	Referrer string `db:"referrer" json:"referrer"`
	Count    int64  `db:"count" json:"count"`
}

type DeviceStats struct {
	DeviceType string `db:"device_type" json:"device_type"`
	Count      int64  `db:"count" json:"count"`
}

type BrowserStats struct {
	Browser string `db:"browser" json:"browser"`
	Count   int64  `db:"count" json:"count"`
}

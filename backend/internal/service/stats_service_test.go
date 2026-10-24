package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func utcDate(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

type statsFixture struct {
	svc     *service.StatsServiceImpl
	clicks  *mocks.MockClickRepository
	links   *mocks.MockLinkRepository
	domains *mocks.MockDomainRepository
}

func newStatsSvc(t *testing.T) *statsFixture {
	ctrl := gomock.NewController(t)
	f := &statsFixture{clicks: mocks.NewMockClickRepository(ctrl), links: mocks.NewMockLinkRepository(ctrl),
		domains: mocks.NewMockDomainRepository(ctrl)}
	f.svc = service.NewStatsService(f.clicks, f.links, f.domains, "http://localhost:8080/")
	f.svc.SetClock(func() time.Time { return time.Date(2026, 10, 24, 15, 4, 5, 0, time.UTC) })
	return f
}

func TestStatsService_Compare_PreviousWindow(t *testing.T) {
	f := newStatsSvc(t)
	ctx := context.Background()
	ids := []uint64{5}
	from, to := utcDate(2026, 10, 11), utcDate(2026, 10, 20) // 10 days
	prevFrom, prevTo := utcDate(2026, 10, 1), utcDate(2026, 10, 10)

	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1}, nil)
	f.clicks.EXPECT().TotalAllTime(gomock.Any(), ids, "qr").Return(int64(100), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), ids, from, to, "qr").Return(int64(30), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), ids, prevFrom, prevTo, "qr").Return(int64(20), nil)
	f.clicks.EXPECT().Series(gomock.Any(), ids, from, to, "day", "qr").
		Return([]repository.Bucket{{Date: "2026-10-11", Count: 30}}, nil)
	f.clicks.EXPECT().Series(gomock.Any(), ids, prevFrom, prevTo, "day", "qr").
		Return([]repository.Bucket{{Date: "2026-10-01", Count: 20}}, nil)
	f.clicks.EXPECT().Breakdown(gomock.Any(), ids, gomock.Any(), gomock.Any(), gomock.Any(), 50, "qr").
		DoAndReturn(func(_ context.Context, _ []uint64, bf, bt time.Time, facet string, _ int, _ string) ([]repository.Row, error) {
			cur := bf.Equal(from) && bt.Equal(to)
			prev := bf.Equal(prevFrom) && bt.Equal(prevTo)
			require.True(t, cur || prev, "unexpected window %s..%s", bf, bt)
			switch facet {
			case "country":
				if cur {
					return []repository.Row{{Key: "US", Count: 15}, {Key: "FR", Count: 15}}, nil
				}
				return []repository.Row{{Key: "US", Count: 4}, {Key: "DE", Count: 16}}, nil
			case "city":
				if cur {
					return []repository.Row{{Key: "Paris", Key2: "FR", Count: 15}}, nil
				}
				return []repository.Row{{Key: "Paris", Key2: "US", Count: 9}, {Key: "Paris", Key2: "FR", Count: 3}}, nil
			case "referrer":
				if cur {
					return []repository.Row{{Key: "Direct", Count: 30}}, nil
				}
				return []repository.Row{{Key: "Direct", Count: 7}}, nil
			case "device", "browser", "os":
				if cur {
					return []repository.Row{{Key: "k-" + facet, Count: 6}}, nil
				}
				return nil, nil
			}
			return nil, errors.New("bad facet " + facet)
		}).Times(12)

	got, err := f.svc.GetLinkStats(ctx, 1, 5, service.StatsQuery{From: from, To: to, Unit: "day", Compare: true, Source: "qr"})
	require.NoError(t, err)

	assert.Equal(t, int64(100), got.TotalAllTime)
	assert.Equal(t, int64(30), got.TotalRange)
	assert.Equal(t, int64(20), got.PrevTotalRange)
	require.NotNil(t, got.ChangePct)
	assert.InDelta(t, 50.0, *got.ChangePct, 1e-9)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-11", Clicks: 30}}, got.Series)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-01", Clicks: 20}}, got.PrevSeries)
	assert.Equal(t, []service.CountryStat{
		{Code: "US", Name: "United States", Clicks: 15, Percentage: 50, PrevClicks: 4},
		{Code: "FR", Name: "France", Clicks: 15, Percentage: 50, PrevClicks: 0},
	}, got.Countries)
	assert.Equal(t, []service.CityStat{{Name: "Paris", Country: "FR", Clicks: 15, Percentage: 50, PrevClicks: 3}}, got.Cities)
	assert.Equal(t, []service.ReferrerStat{{Referrer: "Direct", Clicks: 30, Percentage: 100, PrevClicks: 7}}, got.Referrers)
	assert.Equal(t, []service.KeyStat{{Key: "k-device", Clicks: 6, Percentage: 20}}, got.Devices)
	assert.Equal(t, []service.KeyStat{{Key: "k-browser", Clicks: 6, Percentage: 20}}, got.Browsers)
	assert.Equal(t, []service.KeyStat{{Key: "k-os", Clicks: 6, Percentage: 20}}, got.OS)
	assert.Equal(t, "2026-10-11", got.From)
	assert.Equal(t, "2026-10-20", got.To)
}

func TestStatsService_Compare_HourlyPreviousWindow_NoPrevClicksMeansNilChange(t *testing.T) {
	f := newStatsSvc(t)
	ids := []uint64{5}
	d := utcDate(2026, 10, 20)
	prev := utcDate(2026, 10, 19)

	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1}, nil)
	f.clicks.EXPECT().TotalAllTime(gomock.Any(), ids, "").Return(int64(3), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), ids, d, d, "").Return(int64(3), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), ids, prev, prev, "").Return(int64(0), nil)
	f.clicks.EXPECT().Series(gomock.Any(), ids, d, d, "hour", "").Return([]repository.Bucket{{Date: "2026-10-20T00:00", Count: 3}}, nil)
	f.clicks.EXPECT().Series(gomock.Any(), ids, prev, prev, "hour", "").Return([]repository.Bucket{{Date: "2026-10-19T00:00"}}, nil)
	f.clicks.EXPECT().Breakdown(gomock.Any(), ids, gomock.Any(), gomock.Any(), gomock.Any(), 50, "").Return(nil, nil).Times(12)

	got, err := f.svc.GetLinkStats(context.Background(), 1, 5, service.StatsQuery{From: d, To: d, Unit: "hour", Compare: true, Source: "all"})
	require.NoError(t, err)
	assert.Nil(t, got.ChangePct)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-19T00:00"}}, got.PrevSeries)
	assert.Equal(t, []service.CountryStat{}, got.Countries)
	assert.Equal(t, []service.KeyStat{}, got.OS)
}

func TestStatsService_Defaults30Days(t *testing.T) {
	f := newStatsSvc(t)
	ids := []uint64{5}
	from, to := utcDate(2026, 9, 25), utcDate(2026, 10, 24)

	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1}, nil)
	f.clicks.EXPECT().TotalAllTime(gomock.Any(), ids, "").Return(int64(0), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), ids, from, to, "").Return(int64(0), nil)
	f.clicks.EXPECT().Series(gomock.Any(), ids, from, to, "day", "").Return([]repository.Bucket{}, nil)
	f.clicks.EXPECT().Breakdown(gomock.Any(), ids, from, to, gomock.Any(), 50, "").Return(nil, nil).Times(6)

	got, err := f.svc.GetLinkStats(context.Background(), 1, 5, service.StatsQuery{})
	require.NoError(t, err)
	assert.Equal(t, "2026-09-25", got.From)
	assert.Equal(t, "2026-10-24", got.To)
	assert.Equal(t, "day", got.Unit)
	assert.Nil(t, got.ChangePct)
	assert.Equal(t, int64(0), got.PrevTotalRange)
	assert.Equal(t, []service.SeriesPoint{}, got.Series)
	assert.Equal(t, []service.SeriesPoint{}, got.PrevSeries)
}

func TestStatsService_NotOwner(t *testing.T) {
	f := newStatsSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 2}, nil)
	_, err := f.svc.GetLinkStats(context.Background(), 1, 5, service.StatsQuery{})
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
}

func TestNormalizeStatsQuery(t *testing.T) {
	now := time.Date(2026, 10, 24, 23, 59, 0, 0, time.UTC)
	q, err := service.NormalizeStatsQuery(service.StatsQuery{Source: "all"}, now)
	require.NoError(t, err)
	assert.Equal(t, service.StatsQuery{From: utcDate(2026, 9, 25), To: utcDate(2026, 10, 24), Unit: "day", Source: ""}, q)

	q, err = service.NormalizeStatsQuery(service.StatsQuery{To: utcDate(2026, 10, 2), Unit: "hour"}, now)
	assert.Error(t, err, "default from = to-29d exceeds hourly limit")

	q, err = service.NormalizeStatsQuery(service.StatsQuery{From: utcDate(2026, 10, 1), To: utcDate(2026, 10, 2), Unit: "hour", Source: "page"}, now)
	require.NoError(t, err)
	assert.Equal(t, "page", q.Source)

	bad := []service.StatsQuery{
		{From: utcDate(2026, 10, 1), To: utcDate(2026, 10, 3), Unit: "hour"},
		{From: utcDate(2026, 10, 5), To: utcDate(2026, 10, 1)},
		{From: utcDate(2025, 10, 1), To: utcDate(2026, 10, 2)},
		{Unit: "week"},
		{Source: "email"},
	}
	for _, b := range bad {
		_, err := service.NormalizeStatsQuery(b, now)
		var qe *service.StatsQueryError
		assert.True(t, errors.As(err, &qe), "%+v", b)
	}
	_, err = service.NormalizeStatsQuery(bad[0], now)
	assert.EqualError(t, err, "hourly data is limited to 2 days")
	_, err = service.NormalizeStatsQuery(service.StatsQuery{From: utcDate(2025, 10, 23), To: utcDate(2026, 10, 23)}, now)
	assert.NoError(t, err, "366 days allowed")
}

func TestStatsService_GroupEngagements(t *testing.T) {
	f := newStatsSvc(t)
	from, to := utcDate(2026, 10, 18), utcDate(2026, 10, 24)
	f.clicks.EXPECT().CountRangeByGroup(gomock.Any(), uint64(7), from, to, "").Return(int64(42), nil)
	n, err := f.svc.GroupEngagements(context.Background(), 7, from, to)
	require.NoError(t, err)
	assert.Equal(t, int64(42), n)
}

func TestStatsService_CompareLink_AddsSeries(t *testing.T) {
	f := newStatsSvc(t)
	from, to := utcDate(2026, 10, 11), utcDate(2026, 10, 20)
	domainID := uint64(3)

	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1, GroupID: 1}, nil)
	// The comparison link may live in another group of the same user.
	f.links.EXPECT().GetByID(gomock.Any(), uint64(9)).
		Return(&model.Link{ID: 9, UserID: 1, GroupID: 2, ShortCode: "abc", DomainID: &domainID}, nil)
	f.domains.EXPECT().GetByID(gomock.Any(), domainID).Return(&model.Domain{ID: domainID, Domain: "go.example.com"}, nil)
	f.clicks.EXPECT().TotalAllTime(gomock.Any(), []uint64{5}, "qr").Return(int64(10), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), []uint64{5}, from, to, "qr").Return(int64(4), nil)
	f.clicks.EXPECT().Series(gomock.Any(), []uint64{5}, from, to, "day", "qr").Return([]repository.Bucket{{Date: "2026-10-11", Count: 4}}, nil)
	f.clicks.EXPECT().Breakdown(gomock.Any(), []uint64{5}, from, to, gomock.Any(), 50, "qr").Return(nil, nil).Times(6)
	// Same window, unit and source; no previous period for the comparison link.
	f.clicks.EXPECT().CountRange(gomock.Any(), []uint64{9}, from, to, "qr").Return(int64(7), nil)
	f.clicks.EXPECT().Series(gomock.Any(), []uint64{9}, from, to, "day", "qr").Return([]repository.Bucket{{Date: "2026-10-12", Count: 7}}, nil)

	got, err := f.svc.GetLinkStats(context.Background(), 1, 5,
		service.StatsQuery{From: from, To: to, Unit: "day", Source: "qr", CompareLinkID: 9})
	require.NoError(t, err)
	assert.Equal(t, &service.CompareLinkStats{
		ID: 9, Title: "abc", ShortURL: "http://go.example.com/abc", TotalRange: 7,
		Series: []service.SeriesPoint{{Date: "2026-10-12", Clicks: 7}},
	}, got.CompareLink)
	assert.Equal(t, int64(4), got.TotalRange)
}

func TestStatsService_CompareLink_TitleAndBaseShortURL(t *testing.T) {
	f := newStatsSvc(t)
	from, to := utcDate(2026, 10, 11), utcDate(2026, 10, 20)
	title := "Spring sale"

	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1}, nil)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Link{ID: 9, UserID: 1, ShortCode: "xyz", Title: &title}, nil)
	f.clicks.EXPECT().TotalAllTime(gomock.Any(), gomock.Any(), "").Return(int64(0), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), gomock.Any(), from, to, "").Return(int64(0), nil).Times(2)
	f.clicks.EXPECT().Series(gomock.Any(), gomock.Any(), from, to, "day", "").Return([]repository.Bucket{}, nil).Times(2)
	f.clicks.EXPECT().Breakdown(gomock.Any(), []uint64{5}, from, to, gomock.Any(), 50, "").Return(nil, nil).Times(6)

	got, err := f.svc.GetLinkStats(context.Background(), 1, 5, service.StatsQuery{From: from, To: to, CompareLinkID: 9})
	require.NoError(t, err)
	require.NotNil(t, got.CompareLink)
	assert.Equal(t, "Spring sale", got.CompareLink.Title)
	assert.Equal(t, "http://localhost:8080/xyz", got.CompareLink.ShortURL)
	assert.Equal(t, []service.SeriesPoint{}, got.CompareLink.Series)
}

func TestStatsService_CompareLink_OtherOwner404(t *testing.T) {
	f := newStatsSvc(t)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1}, nil)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Link{ID: 9, UserID: 2}, nil)

	_, err := f.svc.GetLinkStats(context.Background(), 1, 5, service.StatsQuery{CompareLinkID: 9})
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)

	f.links.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(&model.Link{ID: 5, UserID: 1}, nil)
	f.links.EXPECT().GetByID(gomock.Any(), uint64(8)).Return(nil, repository.ErrLinkNotFound)
	_, err = f.svc.GetLinkStats(context.Background(), 1, 5, service.StatsQuery{CompareLinkID: 8})
	assert.ErrorIs(t, err, repository.ErrLinkNotFound)
}

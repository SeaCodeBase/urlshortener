package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
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

type analyticsFixture struct {
	svc     *service.AnalyticsServiceImpl
	clicks  *mocks.MockClickRepository
	links   *mocks.MockLinkRepository
	qrs     *mocks.MockQRRepository
	domains *mocks.MockDomainRepository
}

func newAnalyticsSvc(t *testing.T) *analyticsFixture {
	ctrl := gomock.NewController(t)
	f := &analyticsFixture{
		clicks:  mocks.NewMockClickRepository(ctrl),
		links:   mocks.NewMockLinkRepository(ctrl),
		qrs:     mocks.NewMockQRRepository(ctrl),
		domains: mocks.NewMockDomainRepository(ctrl),
	}
	f.svc = service.NewAnalyticsService(f.clicks, f.links, f.qrs, f.domains, "http://localhost:8080/")
	f.svc.SetClock(func() time.Time { return time.Date(2026, 10, 24, 15, 4, 5, 0, time.UTC) })
	return f
}

func strPtr(s string) *string { return &s }

// win is an inclusive window used to route mock calls.
type win struct{ from, to time.Time }

func (w win) is(from, to time.Time) bool { return w.from.Equal(from) && w.to.Equal(to) }

// expectEmptyItems makes ItemsScope return no rows for any window.
func (f *analyticsFixture) expectEmptyItems() {
	f.clicks.EXPECT().ItemsScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 1000).
		Return([]repository.ItemCount{}, nil).AnyTimes()
}

// expectEmptyBreakdowns makes every facet but those in rows return nothing.
func (f *analyticsFixture) expectBreakdowns(rows map[string][]repository.Row) {
	f.clicks.EXPECT().BreakdownScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).
		DoAndReturn(func(_ context.Context, _ repository.ClickScope, _, _ time.Time, facet string, _ int) ([]repository.Row, error) {
			return rows[facet], nil
		}).AnyTimes()
}

func TestAnalyticsService_DashboardData_ComputesTopDayAndChange(t *testing.T) {
	ctx := context.Background()
	cur := win{utcDate(2026, 10, 18), utcDate(2026, 10, 20)}
	prev := win{utcDate(2026, 10, 15), utcDate(2026, 10, 17)}
	scope := repository.ClickScope{GroupID: 7}

	setup := func(t *testing.T, curTotal, prevTotal int64, curSeries, prevSeries []int64) *analyticsFixture {
		f := newAnalyticsSvc(t)
		f.clicks.EXPECT().CountScope(gomock.Any(), scope, gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ repository.ClickScope, from, to time.Time) (int64, error) {
				if cur.is(from, to) {
					return curTotal, nil
				}
				require.True(t, prev.is(from, to), "unexpected window")
				return prevTotal, nil
			}).AnyTimes()
		f.clicks.EXPECT().SeriesScope(gomock.Any(), scope, gomock.Any(), gomock.Any(), "day").
			DoAndReturn(func(_ context.Context, _ repository.ClickScope, from, to time.Time, _ string) ([]repository.Bucket, error) {
				w, counts := cur, curSeries
				if !cur.is(from, to) {
					w, counts = prev, prevSeries
				}
				out := []repository.Bucket{}
				for i, c := range counts {
					out = append(out, repository.Bucket{Date: w.from.AddDate(0, 0, i).Format("2006-01-02"), Count: c})
				}
				return out, nil
			}).AnyTimes()
		f.expectBreakdowns(map[string][]repository.Row{"country": {{Key: "US", Count: 55}}})
		f.expectEmptyItems()
		return f
	}

	t.Run("change against previous period", func(t *testing.T) {
		f := setup(t, 120, 100, []int64{10, 60, 50}, []int64{5, 30, 0})
		d, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "all"},
			service.StatsQuery{From: cur.from, To: cur.to, Compare: true})
		require.NoError(t, err)
		assert.Equal(t, "2026-10-18", d.From)
		assert.Equal(t, "2026-10-20", d.To)
		assert.Equal(t, "day", d.Unit)
		assert.True(t, d.Compare)
		assert.Equal(t, int64(120), d.Total)
		assert.Equal(t, int64(100), d.PrevTotal)
		require.NotNil(t, d.ChangePct)
		assert.InDelta(t, 20.0, *d.ChangePct, 0.001)
		require.NotNil(t, d.TopDay)
		assert.Equal(t, "2026-10-19", d.TopDay.Date)
		assert.Equal(t, int64(60), d.TopDay.Count)
		assert.Equal(t, int64(30), d.TopDay.PrevCount)
		require.NotNil(t, d.TopDay.ChangePct)
		assert.InDelta(t, 100.0, *d.TopDay.ChangePct, 0.001)
		require.NotNil(t, d.TopLocation)
		assert.Equal(t, service.TopLocation{Code: "US", Name: "United States", Count: 55}, *d.TopLocation)
		assert.Len(t, d.PrevSeries, 3)
		assert.Equal(t, []service.AnalyticsItem{}, d.Items)
	})

	t.Run("null change when previous is zero", func(t *testing.T) {
		f := setup(t, 120, 0, []int64{10, 60, 50}, []int64{0, 0, 0})
		d, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "all"},
			service.StatsQuery{From: cur.from, To: cur.to, Compare: true})
		require.NoError(t, err)
		assert.Nil(t, d.ChangePct)
		require.NotNil(t, d.TopDay)
		assert.Nil(t, d.TopDay.ChangePct)
	})

	t.Run("ties pick the first bucket; no comparison", func(t *testing.T) {
		f := setup(t, 120, 0, []int64{60, 60, 0}, nil)
		d, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "all"},
			service.StatsQuery{From: cur.from, To: cur.to})
		require.NoError(t, err)
		assert.Equal(t, "2026-10-18", d.TopDay.Date)
		assert.Nil(t, d.ChangePct)
		assert.Equal(t, []service.SeriesPoint{}, d.PrevSeries)
	})

	t.Run("no top day when total is zero", func(t *testing.T) {
		f := setup(t, 0, 0, []int64{0, 0, 0}, nil)
		d, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "all"},
			service.StatsQuery{From: cur.from, To: cur.to})
		require.NoError(t, err)
		assert.Nil(t, d.TopDay)
	})

	t.Run("invalid query", func(t *testing.T) {
		f := newAnalyticsSvc(t)
		_, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "all"},
			service.StatsQuery{From: cur.to, To: cur.from})
		var qe *service.StatsQueryError
		assert.ErrorAs(t, err, &qe)
	})
}

func TestAnalyticsService_DashboardData_HourlyForShortRanges(t *testing.T) {
	f := newAnalyticsSvc(t)
	day := utcDate(2026, 10, 24)
	f.clicks.EXPECT().CountScope(gomock.Any(), gomock.Any(), day, day).Return(int64(0), nil)
	f.clicks.EXPECT().SeriesScope(gomock.Any(), gomock.Any(), day, day, "hour").Return([]repository.Bucket{}, nil)
	f.expectBreakdowns(nil)
	f.expectEmptyItems()
	d, err := f.svc.DashboardData(context.Background(), 7, service.DashboardFilter{Type: "all"},
		service.StatsQuery{From: day, To: day})
	require.NoError(t, err)
	assert.Equal(t, "hour", d.Unit)
}

// Review Focus 4: QR scans of a link that owns a QR code go to the QR item,
// its other clicks stay on the link item; a link without a QR keeps every
// source; a QR-owned link is always the QR item; deleted links are dropped.
func TestAnalyticsService_Items_SplitsQrScans(t *testing.T) {
	f := newAnalyticsSvc(t)
	ctx := context.Background()
	cur := win{utcDate(2026, 10, 18), utcDate(2026, 10, 24)}
	prev := win{utcDate(2026, 10, 11), utcDate(2026, 10, 17)}
	domainID := uint64(3)

	f.clicks.EXPECT().CountScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ repository.ClickScope, from, to time.Time) (int64, error) {
			if cur.is(from, to) {
				return 100, nil
			}
			return 50, nil
		}).Times(2)
	f.clicks.EXPECT().SeriesScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "day").
		Return([]repository.Bucket{}, nil).Times(2)
	f.expectBreakdowns(nil)
	f.clicks.EXPECT().ItemsScope(gomock.Any(), repository.ClickScope{GroupID: 7}, gomock.Any(), gomock.Any(), 1000).
		DoAndReturn(func(_ context.Context, _ repository.ClickScope, from, to time.Time, _ int) ([]repository.ItemCount, error) {
			if cur.is(from, to) {
				return []repository.ItemCount{
					{LinkID: 1, Source: "link", Count: 30}, // link 1 owns QR 11
					{LinkID: 2, Source: "qr", Count: 25},   // link 2 has no QR: stays on link
					{LinkID: 1, Source: "qr", Count: 20},   // → QR 11
					{LinkID: 2, Source: "link", Count: 15}, // merged into link 2 → 40
					{LinkID: 3, Source: "link", Count: 6},  // QR-owned link 3 (QR 12)
					{LinkID: 3, Source: "qr", Count: 4},    // → QR 12 total 10
					{LinkID: 9, Source: "link", Count: 5},  // deleted link: dropped
				}, nil
			}
			require.True(t, prev.is(from, to))
			return []repository.ItemCount{
				{LinkID: 1, Source: "qr", Count: 8},
				{LinkID: 2, Source: "link", Count: 3},
			}, nil
		}).Times(2)
	f.links.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uint64) ([]model.Link, error) {
			got := append([]uint64(nil), ids...)
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			assert.Equal(t, []uint64{1, 2, 3, 9}, got)
			return []model.Link{
				{ID: 1, ShortCode: "one", Title: strPtr("Link one"), Source: model.LinkSourceLink},
				{ID: 2, ShortCode: "two", Title: strPtr("Link two"), Source: model.LinkSourceLink, DomainID: &domainID},
				{ID: 3, ShortCode: "thr", Title: strPtr("QR link"), Source: model.LinkSourceQR},
			}, nil
		})
	f.qrs.EXPECT().QRCodeIDsForLinks(gomock.Any(), gomock.Any()).
		Return(map[uint64]uint64{1: 11, 3: 12}, nil)
	f.qrs.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, ids []uint64) ([]repository.QRCodeRow, error) {
			got := append([]uint64(nil), ids...)
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			assert.Equal(t, []uint64{11, 12}, got)
			return []repository.QRCodeRow{
				{QRCode: model.QRCode{ID: 11, LinkID: 1, Title: strPtr("Menu")}},
				{QRCode: model.QRCode{ID: 12, LinkID: 3}, LinkTitle: strPtr("QR link")},
			}, nil
		})
	f.domains.EXPECT().GetByID(gomock.Any(), domainID).Return(&model.Domain{ID: domainID, Domain: "go.example"}, nil)

	d, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "all"},
		service.StatsQuery{From: cur.from, To: cur.to, Compare: true})
	require.NoError(t, err)
	assert.Equal(t, []service.AnalyticsItem{
		{Type: "link", ID: 2, LinkID: 2, Title: "Link two", ShortURL: "http://go.example/two", Clicks: 40, Percentage: 40, PrevClicks: 3},
		{Type: "link", ID: 1, LinkID: 1, Title: "Link one", ShortURL: "http://localhost:8080/one", Clicks: 30, Percentage: 30, PrevClicks: 0},
		{Type: "qr", ID: 11, LinkID: 1, Title: "Menu", ShortURL: "http://localhost:8080/one", Clicks: 20, Percentage: 20, PrevClicks: 8},
		{Type: "qr", ID: 12, LinkID: 3, Title: "QR link", ShortURL: "http://localhost:8080/thr", Clicks: 10, Percentage: 10, PrevClicks: 0},
	}, d.Items)
}

func TestAnalyticsService_ScopeFor_Filters(t *testing.T) {
	ctx := context.Background()
	day := utcDate(2026, 10, 20)
	run := func(t *testing.T, f *analyticsFixture, filter service.DashboardFilter, want repository.ClickScope) {
		f.clicks.EXPECT().CountScope(gomock.Any(), want, day, day).Return(int64(0), nil)
		f.clicks.EXPECT().SeriesScope(gomock.Any(), want, day, day, "hour").Return([]repository.Bucket{}, nil)
		f.clicks.EXPECT().BreakdownScope(gomock.Any(), want, day, day, gomock.Any(), 200).Return(nil, nil).AnyTimes()
		f.clicks.EXPECT().ItemsScope(gomock.Any(), want, day, day, 1000).Return(nil, nil)
		_, err := f.svc.DashboardData(ctx, 7, filter, service.StatsQuery{From: day, To: day})
		require.NoError(t, err)
	}

	t.Run("all", func(t *testing.T) {
		run(t, newAnalyticsSvc(t), service.DashboardFilter{Type: "all"}, repository.ClickScope{GroupID: 7})
	})
	t.Run("empty type means all", func(t *testing.T) {
		run(t, newAnalyticsSvc(t), service.DashboardFilter{}, repository.ClickScope{GroupID: 7})
	})
	t.Run("links", func(t *testing.T) {
		run(t, newAnalyticsSvc(t), service.DashboardFilter{Type: "links", IDs: []uint64{4, 5}},
			repository.ClickScope{GroupID: 7, LinkIDs: []uint64{4, 5}, Source: "not_qr"})
	})
	t.Run("qr maps to link ids", func(t *testing.T) {
		f := newAnalyticsSvc(t)
		f.qrs.EXPECT().GetByIDs(gomock.Any(), []uint64{11, 12, 13}).Return([]repository.QRCodeRow{
			{QRCode: model.QRCode{ID: 11, GroupID: 7, LinkID: 1}},
			{QRCode: model.QRCode{ID: 12, GroupID: 7, LinkID: 3}},
			{QRCode: model.QRCode{ID: 13, GroupID: 8, LinkID: 4}}, // other group: ignored
		}, nil)
		run(t, f, service.DashboardFilter{Type: "qr", IDs: []uint64{11, 12, 13}},
			repository.ClickScope{GroupID: 7, LinkIDs: []uint64{1, 3}, Source: "qr"})
	})
	t.Run("unknown type", func(t *testing.T) {
		f := newAnalyticsSvc(t)
		_, err := f.svc.DashboardData(ctx, 7, service.DashboardFilter{Type: "tags"}, service.StatsQuery{From: day, To: day})
		assert.Error(t, err)
	})
}

func TestAnalyticsService_Overview_Windows(t *testing.T) {
	f := newAnalyticsSvc(t)
	ctx := context.Background()
	scope := repository.ClickScope{GroupID: 7}
	this := win{utcDate(2026, 10, 18), utcDate(2026, 10, 24)}
	last := win{utcDate(2026, 10, 11), utcDate(2026, 10, 17)}
	month := win{utcDate(2026, 9, 25), utcDate(2026, 10, 24)}

	f.clicks.EXPECT().CountScope(gomock.Any(), scope, this.from, this.to).Return(int64(45), nil)
	f.clicks.EXPECT().CountScope(gomock.Any(), scope, last.from, last.to).Return(int64(40), nil)
	f.clicks.EXPECT().CountScope(gomock.Any(), scope, month.from, month.to).Return(int64(200), nil)
	f.clicks.EXPECT().SeriesScope(gomock.Any(), scope, this.from, this.to, "day").
		Return([]repository.Bucket{{Date: "2026-10-18", Count: 45}}, nil)
	f.clicks.EXPECT().SeriesScope(gomock.Any(), scope, last.from, last.to, "day").
		Return([]repository.Bucket{{Date: "2026-10-11", Count: 40}}, nil)
	rows := []repository.ItemCount{}
	links := []model.Link{}
	for i := uint64(1); i <= 6; i++ {
		rows = append(rows, repository.ItemCount{LinkID: i, Source: "link", Count: int64(70 - i*10)})
		links = append(links, model.Link{ID: i, ShortCode: "c" + string(rune('0'+i)), Source: model.LinkSourceLink})
	}
	f.clicks.EXPECT().ItemsScope(gomock.Any(), scope, month.from, month.to, 1000).Return(rows, nil)
	f.links.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).Return(links, nil)
	f.qrs.EXPECT().QRCodeIDsForLinks(gomock.Any(), gomock.Any()).Return(map[uint64]uint64{}, nil)

	o, err := f.svc.Overview(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, service.OverviewWeek{From: "2026-10-18", To: "2026-10-24", Total: 45,
		Series: []service.SeriesPoint{{Date: "2026-10-18", Clicks: 45}}}, o.ThisWeek)
	assert.Equal(t, "2026-10-11", o.LastWeek.From)
	assert.Equal(t, "2026-10-17", o.LastWeek.To)
	assert.Equal(t, int64(40), o.LastWeek.Total)
	require.NotNil(t, o.ChangePct)
	assert.InDelta(t, 12.5, *o.ChangePct, 0.001)
	require.Len(t, o.TopPerforming, 5)
	assert.Equal(t, uint64(1), o.TopPerforming[0].ID)
	assert.InDelta(t, 30.0, o.TopPerforming[0].Percentage, 0.001)
	assert.Equal(t, uint64(5), o.TopPerforming[4].ID)
}

func TestAnalyticsService_ExportZip(t *testing.T) {
	f := newAnalyticsSvc(t)
	day := utcDate(2026, 10, 20)
	f.clicks.EXPECT().CountScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(10), nil).AnyTimes()
	f.clicks.EXPECT().SeriesScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]repository.Bucket{{Date: "2026-10-20T00:00", Count: 10}}, nil).AnyTimes()
	f.expectBreakdowns(map[string][]repository.Row{
		"country":  {{Key: "US", Count: 10}},
		"city":     {{Key: "Austin", Key2: "US", Count: 10}},
		"referrer": {{Key: "@evil", Count: 10}},
		"device":   {{Key: "mobile", Count: 10}},
	})
	f.clicks.EXPECT().ItemsScope(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 1000).
		Return([]repository.ItemCount{{LinkID: 1, Source: "link", Count: 10}}, nil).AnyTimes()
	f.links.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).
		Return([]model.Link{{ID: 1, ShortCode: "abc", Title: strPtr("=HYPERLINK(1)"), Source: model.LinkSourceLink}}, nil)
	f.qrs.EXPECT().QRCodeIDsForLinks(gomock.Any(), gomock.Any()).Return(map[uint64]uint64{}, nil)

	data, err := f.svc.ExportZip(context.Background(), 7, "+Team", service.DashboardFilter{Type: "all"},
		service.StatsQuery{From: day, To: day, Compare: true})
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	files := map[string]string{}
	names := []string{}
	for _, zf := range zr.File {
		rc, err := zf.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		files[zf.Name] = string(b)
		names = append(names, zf.Name)
	}
	assert.ElementsMatch(t, []string{"summary.csv", "engagements_over_time.csv", "devices.csv", "referrers.csv",
		"locations.csv", "links_and_qr_codes.csv", "top.csv"}, names)

	assert.Contains(t, files["links_and_qr_codes.csv"], "'=HYPERLINK(1)")
	assert.Contains(t, files["links_and_qr_codes.csv"], "link,'=HYPERLINK(1),http://localhost:8080/abc,10,100.0,10\n")
	assert.Contains(t, files["referrers.csv"], "'@evil,10,100.0,10")
	assert.Contains(t, files["summary.csv"], "Dashboard,'+Team\n")
	assert.Contains(t, files["summary.csv"], "Range,2026-10-20 to 2026-10-20\n")
	assert.Contains(t, files["summary.csv"], "Total,10\n")
	assert.Contains(t, files["summary.csv"], "Previous total,10\n")
	assert.Contains(t, files["summary.csv"], "Change %,0.0\n")
	assert.Contains(t, files["engagements_over_time.csv"], "2026-10-20T00:00,10,10\n")
	assert.Contains(t, files["devices.csv"], "mobile,10,100.0,10\n")
	loc := files["locations.csv"]
	assert.Contains(t, loc, "US,United States,10,100.0,10\n\nCity,")
	assert.Contains(t, loc, "Austin,US,10,100.0,10\n")
	assert.Contains(t, files["top.csv"], "Top day,2026-10-20T00:00,10\n")
	assert.Contains(t, files["top.csv"], "Top location,United States,10\n")
	assert.True(t, strings.HasPrefix(files["top.csv"], "Metric,Value,Engagements\n"))
}

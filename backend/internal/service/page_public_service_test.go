package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const pagePublicSalt = "test-salt"

type pagePublicFixture struct {
	svc     *service.PagePublicServiceImpl
	pages   *mocks.MockPageRepository
	views   *mocks.MockPageViewRepository
	clicks  *mocks.MockClickRepository
	links   *mocks.MockLinkRepository
	domains *mocks.MockDomainRepository
	now     time.Time
}

func newPagePublicSvc(t *testing.T) *pagePublicFixture {
	ctrl := gomock.NewController(t)
	f := &pagePublicFixture{
		pages:   mocks.NewMockPageRepository(ctrl),
		views:   mocks.NewMockPageViewRepository(ctrl),
		clicks:  mocks.NewMockClickRepository(ctrl),
		links:   mocks.NewMockLinkRepository(ctrl),
		domains: mocks.NewMockDomainRepository(ctrl),
		now:     time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
	}
	f.svc = service.NewPagePublicService(f.pages, f.views, f.clicks, f.links, f.domains, pageBaseURL+"/", pagePublicSalt)
	f.svc.SetClock(func() time.Time { return f.now })
	return f
}

func publishedPage(id, groupID uint64, slug string) *model.Page {
	p := samplePage(id, groupID, slug)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	p.Status = model.PageStatusPublished
	p.PublishedAt = &at
	p.PublishedContent = model.JSONMap{
		"slug": slug, "title": "Snapshot title", "description": "", "avatar_url": "",
		"design": map[string]any(p.Design), "published_at": at.Format(time.RFC3339),
		"blocks": []any{},
	}
	return p
}

func TestPagePublicService_GetPublished_DraftIs404(t *testing.T) {
	ctx := context.Background()
	t.Run("never published", func(t *testing.T) {
		f := newPagePublicSvc(t)
		f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(samplePage(1, 7, "jose"), nil)
		_, err := f.svc.GetPublished(ctx, "jose")
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
	t.Run("status draft with a stale snapshot", func(t *testing.T) {
		f := newPagePublicSvc(t)
		p := publishedPage(1, 7, "jose")
		p.Status = model.PageStatusDraft
		f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(p, nil)
		_, err := f.svc.GetPublished(ctx, "jose")
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
	t.Run("published without content", func(t *testing.T) {
		f := newPagePublicSvc(t)
		p := publishedPage(1, 7, "jose")
		p.PublishedContent = nil
		f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(p, nil)
		_, err := f.svc.GetPublished(ctx, "jose")
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
	t.Run("missing", func(t *testing.T) {
		f := newPagePublicSvc(t)
		f.pages.EXPECT().GetBySlug(gomock.Any(), "nobody").Return(nil, repository.ErrPageNotFound)
		_, err := f.svc.GetPublished(ctx, "nobody")
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
	t.Run("invalid slug never hits the repo", func(t *testing.T) {
		f := newPagePublicSvc(t)
		_, err := f.svc.GetPublished(ctx, "../x")
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
}

func TestPagePublicService_GetPublished_ArchivedIs404(t *testing.T) {
	f := newPagePublicSvc(t)
	p := publishedPage(1, 7, "jose")
	p.Archived = true
	f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(p, nil)
	_, err := f.svc.GetPublished(context.Background(), "jose")
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
}

func TestPagePublicService_GetPublished_ServesSnapshotNotLive(t *testing.T) {
	f := newPagePublicSvc(t)
	p := publishedPage(1, 7, "jose")
	p.Title = "Live title edited after publish"
	p.PublishedContent["blocks"] = []any{map[string]any{"id": 5, "type": "text"}}
	f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(p, nil)

	got, err := f.svc.GetPublished(context.Background(), " JOSE ")
	require.NoError(t, err)
	assert.Equal(t, "Snapshot title", got["title"])
	assert.Equal(t, "jose", got["slug"])
	assert.Len(t, got["blocks"], 1)
	_, hasID := got["id"]
	assert.False(t, hasID, "no page id in the public JSON")
	_, hasGroup := got["group_id"]
	assert.False(t, hasGroup)

	// The returned map is a copy: mutating it leaves the page's snapshot alone.
	got["title"] = "x"
	assert.Equal(t, "Snapshot title", p.PublishedContent["title"])
}

func TestPagePublicService_RecordView_Truncates(t *testing.T) {
	ctx := context.Background()
	f := newPagePublicSvc(t)
	f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(publishedPage(1, 7, "jose"), nil)

	longRef := strings.Repeat("é", 600) // multi-byte: truncation counts runes
	ua := "Mozilla/5.0 (" + strings.Repeat("x", 300) + ")"
	var got *model.PageView
	f.views.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *model.PageView) error {
		got = v
		return nil
	})
	require.NoError(t, f.svc.RecordView(ctx, "jose", service.ViewInput{IP: "not-an-ip", UserAgent: ua, Referrer: longRef}))
	require.NotNil(t, got)
	assert.Equal(t, uint64(1), got.PageID)
	assert.Equal(t, 512, len([]rune(got.Referrer)))
	assert.Equal(t, strings.Repeat("é", 512), got.Referrer)
	assert.LessOrEqual(t, len([]rune(got.Browser)), 50)
	assert.LessOrEqual(t, len([]rune(got.DeviceType)), 20)
	assert.LessOrEqual(t, len([]rune(got.City)), 100)
	assert.LessOrEqual(t, len([]rune(got.Country)), 2)
	// GeoIP cannot resolve a malformed IP: empty location, no error.
	assert.Equal(t, "", got.Country)
	assert.Equal(t, "", got.City)

	t.Run("unpublished is 404 and records nothing", func(t *testing.T) {
		f := newPagePublicSvc(t)
		f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(samplePage(1, 7, "jose"), nil)
		err := f.svc.RecordView(ctx, "jose", service.ViewInput{IP: "1.2.3.4"})
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
	t.Run("insert failure is returned", func(t *testing.T) {
		f := newPagePublicSvc(t)
		f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(publishedPage(1, 7, "jose"), nil)
		f.views.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(errors.New("db down"))
		assert.Error(t, f.svc.RecordView(ctx, "jose", service.ViewInput{}))
	})
}

func TestPagePublicService_RecordView_HashesIP(t *testing.T) {
	ctx := context.Background()
	f := newPagePublicSvc(t)
	f.pages.EXPECT().GetBySlug(gomock.Any(), "jose").Return(publishedPage(1, 7, "jose"), nil).Times(2)
	var views []*model.PageView
	f.views.EXPECT().Insert(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, v *model.PageView) error {
		views = append(views, v)
		return nil
	}).Times(2)

	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	require.NoError(t, f.svc.RecordView(ctx, "jose", service.ViewInput{IP: "10.0.0.1", UserAgent: ua, Referrer: "https://t.co/x"}))
	require.NoError(t, f.svc.RecordView(ctx, "jose", service.ViewInput{}))
	require.Len(t, views, 2)

	v := views[0]
	assert.Equal(t, util.HashIP(pagePublicSalt, "10.0.0.1"), v.IPHash)
	assert.Len(t, v.IPHash, 64)
	assert.NotContains(t, v.IPHash, "10.0.0.1")
	assert.Equal(t, "https://t.co/x", v.Referrer)
	assert.Equal(t, "mobile", v.DeviceType)
	assert.Equal(t, "Safari", v.Browser)
	assert.Equal(t, "XX", v.Country) // private IP
	assert.Equal(t, "Local", v.City)

	assert.Equal(t, "", views[1].IPHash, "no IP, no hash")
}

func TestPagePublicService_Stats_BlocksClicks(t *testing.T) {
	ctx := context.Background()
	f := newPagePublicSvc(t)
	p := publishedPage(3, 7, "jose")
	f.pages.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(p, nil)

	from := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	prevFrom := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	prevTo := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

	l9, l11 := uint64(9), uint64(11)
	f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(3)).Return([]model.PageBlock{
		{ID: 5, PageID: 3, Type: model.PageBlockLink, Position: 0, IsActive: true, LinkID: &l9,
			Content: model.JSONMap{"title": "Menu"}},
		{ID: 6, PageID: 3, Type: model.PageBlockText, Position: 1, IsActive: true,
			Content: model.JSONMap{"heading": "Hi"}},
		{ID: 7, PageID: 3, Type: model.PageBlockLink, Position: 2, IsActive: false, LinkID: &l11,
			Content: model.JSONMap{"title": "Hidden"}},
	}, nil)
	dom := uint64(4)
	f.links.EXPECT().GetByIDs(gomock.Any(), gomock.InAnyOrder([]uint64{9, 11})).Return([]model.Link{
		{ID: 9, ShortCode: "abc", OriginalURL: "https://a"},
		{ID: 11, ShortCode: "def", OriginalURL: "https://b", DomainID: &dom},
	}, nil)
	f.domains.EXPECT().GetByID(gomock.Any(), dom).Return(&model.Domain{ID: dom, Domain: "go.example.com"}, nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), []uint64{9}, from, to, "page").Return(int64(4), nil)
	f.clicks.EXPECT().CountRange(gomock.Any(), []uint64{11}, from, to, "page").Return(int64(1), nil)

	f.views.EXPECT().CountAllTime(gomock.Any(), uint64(3)).Return(int64(100), nil)
	f.views.EXPECT().CountRange(gomock.Any(), uint64(3), from, to).Return(int64(20), nil)
	f.views.EXPECT().CountRange(gomock.Any(), uint64(3), prevFrom, prevTo).Return(int64(10), nil)
	f.views.EXPECT().Series(gomock.Any(), uint64(3), from, to, "day").
		Return([]repository.Bucket{{Date: "2026-10-19", Count: 20}}, nil)
	f.views.EXPECT().Series(gomock.Any(), uint64(3), prevFrom, prevTo, "day").
		Return([]repository.Bucket{{Date: "2026-10-12", Count: 10}}, nil)
	for _, facet := range []string{"referrer", "device", "country", "city"} {
		var rows []repository.Row
		switch facet {
		case "referrer":
			rows = []repository.Row{{Key: "Direct", Count: 15}}
		case "device":
			rows = []repository.Row{{Key: "mobile", Count: 20}}
		case "country":
			rows = []repository.Row{{Key: "US", Count: 5}}
		case "city":
			rows = []repository.Row{{Key: "Austin", Key2: "US", Count: 5}}
		}
		f.views.EXPECT().Breakdown(gomock.Any(), uint64(3), from, to, facet, 50).Return(rows, nil)
		f.views.EXPECT().Breakdown(gomock.Any(), uint64(3), prevFrom, prevTo, facet, 50).
			Return([]repository.Row{{Key: rows[0].Key, Key2: rows[0].Key2, Count: 2}}, nil)
	}

	st, err := f.svc.Stats(ctx, 7, 3, service.StatsQuery{From: from, To: to, Compare: true, Source: "qr"})
	require.NoError(t, err)
	assert.Equal(t, "2026-10-19", st.From)
	assert.Equal(t, "2026-10-25", st.To)
	assert.Equal(t, "day", st.Unit)
	assert.Equal(t, int64(100), st.TotalViewsAllTime)
	assert.Equal(t, int64(20), st.TotalViews)
	assert.Equal(t, int64(10), st.PrevTotalViews)
	require.NotNil(t, st.ChangePct)
	assert.InDelta(t, 100.0, *st.ChangePct, 0.001)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-19", Clicks: 20}}, st.Series)
	assert.Equal(t, []service.SeriesPoint{{Date: "2026-10-12", Clicks: 10}}, st.PrevSeries)
	require.Len(t, st.Referrers, 1)
	assert.Equal(t, "Direct", st.Referrers[0].Referrer)
	assert.InDelta(t, 75.0, st.Referrers[0].Percentage, 0.001)
	assert.Equal(t, int64(2), st.Referrers[0].PrevClicks)
	assert.Equal(t, "mobile", st.Devices[0].Key)
	assert.Equal(t, "United States", st.Countries[0].Name)
	assert.Equal(t, "Austin", st.Cities[0].Name)
	assert.Equal(t, "US", st.Cities[0].Country)

	assert.Equal(t, []service.PageBlockStat{
		{BlockID: 5, Title: "Menu", LinkID: 9, ShortURL: pageBaseURL + "/abc", Clicks: 4},
		{BlockID: 7, Title: "Hidden", LinkID: 11, ShortURL: "http://go.example.com/def", Clicks: 1},
	}, st.Blocks)
}

func TestPagePublicService_Stats_NoCompareAndErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("wrong group", func(t *testing.T) {
		f := newPagePublicSvc(t)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(publishedPage(3, 8, "jose"), nil)
		_, err := f.svc.Stats(ctx, 7, 3, service.StatsQuery{})
		assert.ErrorIs(t, err, repository.ErrPageNotFound)
	})
	t.Run("invalid query", func(t *testing.T) {
		f := newPagePublicSvc(t)
		_, err := f.svc.Stats(ctx, 7, 3, service.StatsQuery{Unit: "week"})
		var qe *service.StatsQueryError
		assert.True(t, errors.As(err, &qe))
	})
	t.Run("default window without compare, draft page, no blocks", func(t *testing.T) {
		f := newPagePublicSvc(t)
		from := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
		f.pages.EXPECT().GetByID(gomock.Any(), uint64(3)).Return(samplePage(3, 7, "jose"), nil)
		f.pages.EXPECT().ListBlocks(gomock.Any(), uint64(3)).Return(nil, nil)
		f.views.EXPECT().CountAllTime(gomock.Any(), uint64(3)).Return(int64(0), nil)
		f.views.EXPECT().CountRange(gomock.Any(), uint64(3), from, to).Return(int64(0), nil)
		f.views.EXPECT().Series(gomock.Any(), uint64(3), from, to, "day").Return([]repository.Bucket{}, nil)
		f.views.EXPECT().Breakdown(gomock.Any(), uint64(3), from, to, gomock.Any(), 50).Return(nil, nil).Times(4)
		st, err := f.svc.Stats(ctx, 7, 3, service.StatsQuery{})
		require.NoError(t, err)
		assert.Nil(t, st.ChangePct)
		assert.Equal(t, int64(0), st.PrevTotalViews)
		assert.NotNil(t, st.PrevSeries)
		assert.NotNil(t, st.Series)
		assert.NotNil(t, st.Blocks)
		assert.NotNil(t, st.Referrers)
		assert.NotNil(t, st.Cities)
	})
}

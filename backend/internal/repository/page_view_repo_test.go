package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pvDay(d, h int) time.Time {
	return time.Date(2026, 3, d, h, 0, 0, 0, time.UTC)
}

func insertView(t *testing.T, repo *repository.PageViewRepositoryImpl, v model.PageView) {
	t.Helper()
	require.NoError(t, repo.Insert(context.Background(), &v))
	require.NotZero(t, v.ID)
}

func TestPageViewRepo_SeriesZeroFilled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "pv.series@example.com")
	pages := repository.NewPageRepository(db)
	p := createPage(t, pages, group, "series")
	other := createPage(t, pages, group, "series-other")
	repo := repository.NewPageViewRepository(db)

	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: pvDay(10, 1)})
	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: pvDay(10, 23)})
	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: pvDay(12, 5)})
	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: pvDay(13, 0)}) // outside range
	insertView(t, repo, model.PageView{PageID: other.ID, ViewedAt: pvDay(11, 5)})

	got, err := repo.Series(ctx, p.ID, pvDay(10, 0), pvDay(12, 0), "day")
	require.NoError(t, err)
	assert.Equal(t, []repository.Bucket{
		{Date: "2026-03-10", Count: 2},
		{Date: "2026-03-11", Count: 0},
		{Date: "2026-03-12", Count: 1},
	}, got)

	hourly, err := repo.Series(ctx, p.ID, pvDay(10, 0), pvDay(10, 0), "hour")
	require.NoError(t, err)
	require.Len(t, hourly, 24)
	assert.Equal(t, repository.Bucket{Date: "2026-03-10T01:00", Count: 1}, hourly[1])
	assert.Equal(t, repository.Bucket{Date: "2026-03-10T23:00", Count: 1}, hourly[23])
	assert.Equal(t, int64(0), hourly[0].Count)

	_, err = repo.Series(ctx, p.ID, pvDay(10, 0), pvDay(12, 0), "week")
	assert.Error(t, err)

	n, err := repo.CountRange(ctx, p.ID, pvDay(10, 0), pvDay(12, 0))
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	all, err := repo.CountAllTime(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(4), all)
}

func TestPageViewRepo_BreakdownUnknownBuckets(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "pv.breakdown@example.com")
	p := createPage(t, repository.NewPageRepository(db), group, "breakdown")
	repo := repository.NewPageViewRepository(db)

	at := pvDay(10, 12)
	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: at, Referrer: "", Country: "", City: "", DeviceType: "", Browser: ""})
	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: at, Referrer: "", Country: "US", City: "Austin", DeviceType: "mobile", Browser: "Chrome"})
	insertView(t, repo, model.PageView{PageID: p.ID, ViewedAt: at, Referrer: "instagram.com", Country: "US", City: "Austin", DeviceType: "unknown", Browser: "Chrome"})

	rows, err := repo.Breakdown(ctx, p.ID, at, at, "referrer", 50)
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Direct", Count: 2}, {Key: "instagram.com", Count: 1}}, rows)

	rows, err = repo.Breakdown(ctx, p.ID, at, at, "country", 50)
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "US", Count: 2}, {Key: "Unknown", Count: 1}}, rows)

	rows, err = repo.Breakdown(ctx, p.ID, at, at, "device", 50)
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Unknown", Count: 2}, {Key: "mobile", Count: 1}}, rows)

	rows, err = repo.Breakdown(ctx, p.ID, at, at, "city", 50)
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Austin", Key2: "US", Count: 2}, {Key: "Unknown", Key2: "Unknown", Count: 1}}, rows)

	rows, err = repo.Breakdown(ctx, p.ID, at, at, "browser", 1)
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Chrome", Count: 2}}, rows)

	empty, err := repo.Breakdown(ctx, p.ID, pvDay(11, 0), pvDay(11, 0), "browser", 50)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	_, err = repo.Breakdown(ctx, p.ID, at, at, "os", 50)
	assert.Error(t, err, "page views have no os facet")
	_, err = repo.Breakdown(ctx, p.ID, at, at, "nope", 50)
	assert.Error(t, err)
}

func TestPageViewRepo_CountsForPages(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "pv.counts@example.com")
	pages := repository.NewPageRepository(db)
	a := createPage(t, pages, group, "counts-a")
	b := createPage(t, pages, group, "counts-b")
	c := createPage(t, pages, group, "counts-c")
	repo := repository.NewPageViewRepository(db)

	insertView(t, repo, model.PageView{PageID: a.ID, ViewedAt: pvDay(1, 0)})
	insertView(t, repo, model.PageView{PageID: a.ID, ViewedAt: pvDay(2, 0)})
	insertView(t, repo, model.PageView{PageID: b.ID, ViewedAt: pvDay(3, 0)})

	got, err := repo.CountsForPages(ctx, []uint64{a.ID, b.ID, c.ID})
	require.NoError(t, err)
	assert.Equal(t, map[uint64]int64{a.ID: 2, b.ID: 1}, got)

	only, err := repo.CountsForPages(ctx, []uint64{b.ID})
	require.NoError(t, err)
	assert.Equal(t, map[uint64]int64{b.ID: 1}, only)

	none, err := repo.CountsForPages(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func TestPageViewRepo_InsertDefaultsViewedAt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "pv.insert@example.com")
	p := createPage(t, repository.NewPageRepository(db), group, "insert")
	repo := repository.NewPageViewRepository(db)

	v := &model.PageView{PageID: p.ID, Referrer: "x.com"}
	require.NoError(t, repo.Insert(ctx, v))
	require.NotZero(t, v.ID)
	assert.False(t, v.ViewedAt.IsZero(), "zero ViewedAt is set to now")
	n, err := repo.CountRange(ctx, p.ID, time.Now().UTC(), time.Now().UTC())
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

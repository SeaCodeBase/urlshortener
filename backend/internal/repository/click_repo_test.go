package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clickFixture creates a user, a group and two links in it, plus a link in a
// second group, and returns their IDs.
type clickFixture struct {
	repo        *repository.ClickRepositoryImpl
	groupID     uint64
	otherGroup  uint64
	linkA       uint64
	linkB       uint64
	foreignLink uint64
}

func newClickFixture(t *testing.T, db *sqlx.DB) *clickFixture {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, db, "clicks@example.com")
	groups := repository.NewGroupRepository(db)
	g1 := &model.Group{UserID: user.ID, Name: "g1"}
	g2 := &model.Group{UserID: user.ID, Name: "g2"}
	require.NoError(t, groups.Create(ctx, g1))
	require.NoError(t, groups.Create(ctx, g2))
	links := repository.NewLinkRepository(db)
	mk := func(code string, gid uint64) uint64 {
		l := &model.Link{UserID: user.ID, GroupID: gid, ShortCode: code, OriginalURL: "https://example.com/" + code, IsActive: true}
		require.NoError(t, links.Create(ctx, l))
		return l.ID
	}
	return &clickFixture{
		repo:        repository.NewClickRepository(db),
		groupID:     g1.ID,
		otherGroup:  g2.ID,
		linkA:       mk("cla", g1.ID),
		linkB:       mk("clb", g1.ID),
		foreignLink: mk("clf", g2.ID),
	}
}

func day(d int, h int) time.Time { return time.Date(2026, 10, d, h, 30, 0, 0, time.UTC) }

func date(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

func (f *clickFixture) insert(t *testing.T, clicks ...model.Click) {
	t.Helper()
	for i := range clicks {
		if clicks[i].Source == "" {
			clicks[i].Source = "link"
		}
	}
	require.NoError(t, f.repo.BatchInsert(context.Background(), clicks))
}

func TestClickRepository_Series_ZeroFills(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	f := newClickFixture(t, db)
	ctx := context.Background()

	f.insert(t,
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 1)},
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 23)},
		model.Click{LinkID: f.linkB, ClickedAt: day(12, 5), Source: "qr"},
		model.Click{LinkID: f.linkA, ClickedAt: day(13, 0)},       // after range
		model.Click{LinkID: f.linkA, ClickedAt: day(9, 23)},       // before range
		model.Click{LinkID: f.foreignLink, ClickedAt: day(11, 2)}, // not requested
	)

	got, err := f.repo.Series(ctx, []uint64{f.linkA, f.linkB}, date(10), date(12), "day", "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Bucket{
		{Date: "2026-10-10", Count: 2},
		{Date: "2026-10-11", Count: 0},
		{Date: "2026-10-12", Count: 1},
	}, got)

	got, err = f.repo.Series(ctx, []uint64{f.linkA, f.linkB}, date(10), date(12), "day", "link")
	require.NoError(t, err)
	assert.Equal(t, int64(0), got[2].Count, "qr click excluded by source=link")

	hourly, err := f.repo.Series(ctx, []uint64{f.linkA}, date(10), date(10), "hour", "all")
	require.NoError(t, err)
	require.Len(t, hourly, 24)
	assert.Equal(t, repository.Bucket{Date: "2026-10-10T00:00", Count: 0}, hourly[0])
	assert.Equal(t, repository.Bucket{Date: "2026-10-10T01:00", Count: 1}, hourly[1])
	assert.Equal(t, repository.Bucket{Date: "2026-10-10T23:00", Count: 1}, hourly[23])

	empty, err := f.repo.Series(ctx, nil, date(10), date(11), "day", "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Bucket{{Date: "2026-10-10"}, {Date: "2026-10-11"}}, empty)
}

func TestClickRepository_CountRange_TotalAllTime_ByGroup(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	f := newClickFixture(t, db)
	ctx := context.Background()

	f.insert(t,
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 1)},
		model.Click{LinkID: f.linkB, ClickedAt: day(11, 1), Source: "qr"},
		model.Click{LinkID: f.linkA, ClickedAt: day(20, 1)},
		model.Click{LinkID: f.foreignLink, ClickedAt: day(10, 1)},
	)

	n, err := f.repo.CountRange(ctx, []uint64{f.linkA, f.linkB}, date(10), date(11), "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	n, err = f.repo.CountRange(ctx, []uint64{f.linkA, f.linkB}, date(10), date(11), "qr")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	n, err = f.repo.TotalAllTime(ctx, []uint64{f.linkA}, "all")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	n, err = f.repo.TotalAllTime(ctx, nil, "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)

	n, err = f.repo.CountRangeByGroup(ctx, f.groupID, date(10), date(20), "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	n, err = f.repo.CountRangeByGroup(ctx, f.otherGroup, date(10), date(20), "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestClickRepository_Breakdown_Device(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	f := newClickFixture(t, db)
	ctx := context.Background()

	f.insert(t,
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 1), DeviceType: "mobile", Country: "US", City: "Austin", OS: "iOS", Browser: "Safari", Referrer: "https://t.co/x"},
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 2), DeviceType: "mobile", Country: "US", City: "Austin", OS: "iOS", Browser: "Safari"},
		model.Click{LinkID: f.linkB, ClickedAt: day(10, 3), DeviceType: "desktop", Country: "FR", City: "Paris", OS: "Windows", Browser: "Chrome", Source: "qr"},
		model.Click{LinkID: f.linkB, ClickedAt: day(10, 4), DeviceType: "unknown", Country: "", City: ""},
		model.Click{LinkID: f.linkA, ClickedAt: day(15, 4), DeviceType: "tablet"}, // out of range
	)
	ids := []uint64{f.linkA, f.linkB}

	rows, err := f.repo.Breakdown(ctx, ids, date(10), date(10), "device", 50, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{
		{Key: "mobile", Count: 2},
		{Key: "desktop", Count: 1},
		{Key: "Unknown", Count: 1},
	}, rows, "ties ordered by key; empty and 'unknown' grouped as Unknown")

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "device", 1, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "mobile", Count: 2}}, rows, "limit applies")

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "device", 50, "qr")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "desktop", Count: 1}}, rows, "source filter applies")

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "city", 50, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{
		{Key: "Austin", Key2: "US", Count: 2},
		{Key: "Paris", Key2: "FR", Count: 1},
		{Key: "Unknown", Key2: "Unknown", Count: 1},
	}, rows)

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "referrer", 50, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Direct", Count: 3}, {Key: "https://t.co/x", Count: 1}}, rows)

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "os", 50, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "iOS", Count: 2}, {Key: "Unknown", Count: 1}, {Key: "Windows", Count: 1}}, rows)

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "browser", 50, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Safari", Count: 2}, {Key: "Chrome", Count: 1}, {Key: "Unknown", Count: 1}}, rows)

	rows, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "country", 50, "")
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "US", Count: 2}, {Key: "FR", Count: 1}, {Key: "Unknown", Count: 1}}, rows)

	_, err = f.repo.Breakdown(ctx, ids, date(10), date(10), "nope", 50, "")
	assert.Error(t, err)

	rows, err = f.repo.Breakdown(ctx, nil, date(10), date(10), "device", 50, "")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestClickRepo_Scope_GroupVsIDs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	f := newClickFixture(t, db)
	ctx := context.Background()

	f.insert(t,
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 1)},
		model.Click{LinkID: f.linkA, ClickedAt: day(11, 2), Source: "qr"},
		model.Click{LinkID: f.linkB, ClickedAt: day(11, 3)},
		model.Click{LinkID: f.linkB, ClickedAt: day(12, 4), Source: "page"},
		model.Click{LinkID: f.foreignLink, ClickedAt: day(11, 5)},
		model.Click{LinkID: f.foreignLink, ClickedAt: day(11, 6)},
		model.Click{LinkID: f.linkA, ClickedAt: day(13, 0)}, // after range
	)

	count := func(scope repository.ClickScope) int64 {
		t.Helper()
		n, err := f.repo.CountScope(ctx, scope, date(10), date(12))
		require.NoError(t, err)
		return n
	}

	// Group scope counts only the group's links.
	assert.Equal(t, int64(4), count(repository.ClickScope{GroupID: f.groupID}))
	assert.Equal(t, int64(2), count(repository.ClickScope{GroupID: f.otherGroup}))
	// Id scope counts exactly the listed links.
	assert.Equal(t, int64(4), count(repository.ClickScope{LinkIDs: []uint64{f.linkB, f.foreignLink}}))
	assert.Equal(t, int64(2), count(repository.ClickScope{LinkIDs: []uint64{f.linkA}}))
	// Source filters.
	assert.Equal(t, int64(3), count(repository.ClickScope{GroupID: f.groupID, Source: "not_qr"}))
	assert.Equal(t, int64(1), count(repository.ClickScope{GroupID: f.groupID, Source: "qr"}))
	assert.Equal(t, int64(1), count(repository.ClickScope{LinkIDs: []uint64{f.linkA}, Source: "not_qr"}))
	assert.Equal(t, int64(1), count(repository.ClickScope{GroupID: f.groupID, Source: "page"}))
	// Empty non-nil LinkIDs means no links, even with a group set.
	assert.Equal(t, int64(0), count(repository.ClickScope{GroupID: f.groupID, LinkIDs: []uint64{}}))

	series, err := f.repo.SeriesScope(ctx, repository.ClickScope{GroupID: f.groupID, Source: "not_qr"}, date(10), date(12), "day")
	require.NoError(t, err)
	assert.Equal(t, []repository.Bucket{
		{Date: "2026-10-10", Count: 1},
		{Date: "2026-10-11", Count: 1},
		{Date: "2026-10-12", Count: 1},
	}, series)

	empty, err := f.repo.SeriesScope(ctx, repository.ClickScope{LinkIDs: []uint64{}}, date(10), date(12), "day")
	require.NoError(t, err)
	assert.Equal(t, []repository.Bucket{
		{Date: "2026-10-10", Count: 0},
		{Date: "2026-10-11", Count: 0},
		{Date: "2026-10-12", Count: 0},
	}, empty)

	_, err = f.repo.SeriesScope(ctx, repository.ClickScope{GroupID: f.groupID}, date(10), date(12), "week")
	assert.Error(t, err)

	rows, err := f.repo.BreakdownScope(ctx, repository.ClickScope{GroupID: f.groupID}, date(10), date(12), "referrer", 10)
	require.NoError(t, err)
	assert.Equal(t, []repository.Row{{Key: "Direct", Key2: "", Count: 4}}, rows)

	noRows, err := f.repo.BreakdownScope(ctx, repository.ClickScope{LinkIDs: []uint64{}}, date(10), date(12), "referrer", 10)
	require.NoError(t, err)
	assert.Empty(t, noRows)

	items, err := f.repo.ItemsScope(ctx, repository.ClickScope{LinkIDs: []uint64{}}, date(10), date(12), 10)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestClickRepo_ItemsScope_GroupsByLinkAndSource(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	f := newClickFixture(t, db)
	ctx := context.Background()

	f.insert(t,
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 1)},
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 2)},
		model.Click{LinkID: f.linkA, ClickedAt: day(10, 3), Source: "qr"},
		model.Click{LinkID: f.linkA, ClickedAt: day(11, 4), Source: "qr"},
		model.Click{LinkID: f.linkA, ClickedAt: day(12, 5), Source: "qr"},
		model.Click{LinkID: f.foreignLink, ClickedAt: day(11, 6)}, // other group
	)

	items, err := f.repo.ItemsScope(ctx, repository.ClickScope{GroupID: f.groupID}, date(10), date(12), 10)
	require.NoError(t, err)
	assert.Equal(t, []repository.ItemCount{
		{LinkID: f.linkA, Source: "qr", Count: 3},
		{LinkID: f.linkA, Source: "link", Count: 2},
	}, items)

	limited, err := f.repo.ItemsScope(ctx, repository.ClickScope{GroupID: f.groupID}, date(10), date(12), 1)
	require.NoError(t, err)
	assert.Equal(t, []repository.ItemCount{{LinkID: f.linkA, Source: "qr", Count: 3}}, limited)
}

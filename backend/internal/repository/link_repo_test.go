package repository_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkRepository_Create_WritesGroupID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "linker@example.com")
	group := &model.Group{UserID: user.ID, Name: "linker"}
	if err := repository.NewGroupRepository(db).Create(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}

	repo := repository.NewLinkRepository(db)
	link := &model.Link{
		UserID:      user.ID,
		GroupID:     group.ID,
		ShortCode:   "grp1",
		OriginalURL: "https://example.com/grouped",
		IsActive:    true,
	}
	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.GetByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.GroupID != group.ID {
		t.Errorf("GetByID: expected group_id %d, got %d", group.ID, got.GroupID)
	}

	byCode, err := repo.GetByDomainAndShortCode(ctx, nil, "grp1")
	if err != nil {
		t.Fatalf("GetByDomainAndShortCode failed: %v", err)
	}
	if byCode.GroupID != group.ID {
		t.Errorf("GetByDomainAndShortCode: expected group_id %d, got %d", group.ID, byCode.GroupID)
	}

	list, err := repo.ListByUserID(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListByUserID failed: %v", err)
	}
	if len(list) != 1 || list[0].GroupID != group.ID {
		t.Errorf("ListByUserID: expected one link with group_id %d, got %+v", group.ID, list)
	}
}

func TestDomainRepository_GetByDomain_ReturnsRedirects(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "domainer@example.com")
	repo := repository.NewDomainRepository(db)
	d := &model.Domain{UserID: user.ID, Domain: "go.redirects.test"}
	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE domains SET root_redirect = ?, not_found_redirect = ? WHERE id = ?`,
		"https://root.example.com", "https://404.example.com", d.ID); err != nil {
		t.Fatalf("set redirects: %v", err)
	}

	got, err := repo.GetByDomain(ctx, "go.redirects.test")
	if err != nil {
		t.Fatalf("GetByDomain: %v", err)
	}
	if got.RootRedirect == nil || *got.RootRedirect != "https://root.example.com" ||
		got.NotFoundRedirect == nil || *got.NotFoundRedirect != "https://404.example.com" {
		t.Fatalf("expected redirects populated, got %+v", got)
	}
}

func TestLinkRepository_GetByDomainAndShortCode_ReturnsArchived(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	user := createTestUser(t, db, "archiver@example.com")
	group := &model.Group{UserID: user.ID, Name: "archiver"}
	if err := repository.NewGroupRepository(db).Create(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	repo := repository.NewLinkRepository(db)
	link := &model.Link{UserID: user.ID, GroupID: group.ID, ShortCode: "arch1", OriginalURL: "https://example.com", IsActive: true}
	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("create link: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE links SET archived = TRUE WHERE id = ?`, link.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, err := repo.GetByDomainAndShortCode(ctx, nil, "arch1")
	if err != nil {
		t.Fatalf("GetByDomainAndShortCode: %v", err)
	}
	if !got.Archived {
		t.Fatal("expected Archived=true")
	}
}

// linkFixture creates a user, a group and links in that group. mutate may
// adjust each link before insert.
func linkFixture(t *testing.T, db *sqlx.DB, email string, links ...*model.Link) (*model.Group, []*model.Link) {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, db, email)
	group := &model.Group{UserID: user.ID, Name: email}
	require.NoError(t, repository.NewGroupRepository(db).Create(ctx, group))
	repo := repository.NewLinkRepository(db)
	for _, l := range links {
		l.UserID = user.ID
		l.GroupID = group.ID
		if l.OriginalURL == "" {
			l.OriginalURL = "https://example.com/" + l.ShortCode
		}
		l.IsActive = true
		require.NoError(t, repo.Create(ctx, l))
	}
	return group, links
}

func codesOf(rows []model.LinkWithStats) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ShortCode
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

func TestLinkRepository_Create_WritesV2Columns(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	host := "example.com"
	_, links := linkFixture(t, db, "v2cols@example.com",
		&model.Link{ShortCode: "parent1"})
	parentID := links[0].ID
	repo := repository.NewLinkRepository(db)
	child := &model.Link{UserID: links[0].UserID, GroupID: links[0].GroupID, ShortCode: "My-Custom_Half",
		OriginalURL: "https://example.com/x", LongURLHost: &host, IsActive: true, HasCustomBackHalf: true,
		ParentLinkID: &parentID, Source: model.LinkSourceLink}
	require.NoError(t, repo.Create(ctx, child))

	got, err := repo.GetByID(ctx, child.ID)
	require.NoError(t, err)
	assert.True(t, got.HasCustomBackHalf)
	require.NotNil(t, got.ParentLinkID)
	assert.Equal(t, parentID, *got.ParentLinkID)
	require.NotNil(t, got.LongURLHost)
	assert.Equal(t, "example.com", *got.LongURLHost)
	assert.Equal(t, model.LinkSourceLink, got.Source)
	assert.Equal(t, 0, got.RedirectCount)
}

func TestLinkRepository_ShortCode_CaseSensitiveAndLong(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	long := strings.Repeat("a", 255)
	_, _ = linkFixture(t, db, "case@example.com",
		&model.Link{ShortCode: "AbC"}, &model.Link{ShortCode: "abc"}, &model.Link{ShortCode: long})
	repo := repository.NewLinkRepository(db)
	got, err := repo.GetByDomainAndShortCode(ctx, nil, "abc")
	require.NoError(t, err)
	assert.Equal(t, "abc", got.ShortCode)
	exists, err := repo.ShortCodeExistsInDomain(ctx, nil, "ABC")
	require.NoError(t, err)
	assert.False(t, exists)
	exists, err = repo.ShortCodeExistsInDomain(ctx, nil, long)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLinkRepository_ListByGroup_FilterArchivedDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "arch@example.com",
		&model.Link{ShortCode: "act1"}, &model.Link{ShortCode: "hid1"}, &model.Link{ShortCode: "qr1"})
	repo := repository.NewLinkRepository(db)
	require.NoError(t, repo.SetArchived(ctx, []uint64{links[1].ID}, true))
	_, err := db.ExecContext(ctx, `UPDATE links SET source = 'qr' WHERE id = ?`, links[2].ID)
	require.NoError(t, err)
	// Another group's link never shows up.
	_, _ = linkFixture(t, db, "other@example.com", &model.Link{ShortCode: "oth1"})

	rows, total, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Archived: boolPtr(false)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, []string{"act1"}, codesOf(rows))

	rows, total, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Archived: boolPtr(true)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, []string{"hid1"}, codesOf(rows))
	assert.True(t, rows[0].Archived)

	// nil Archived returns both; source<>'link' excluded unless IncludeAllSources.
	rows, total, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.ElementsMatch(t, []string{"act1", "hid1"}, codesOf(rows))

	rows, total, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{IncludeAllSources: true}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, rows, 3)

	count, err := repo.CountByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestLinkRepository_ListByGroup_FilterTags(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "ftags@example.com",
		&model.Link{ShortCode: "t1"}, &model.Link{ShortCode: "t2"}, &model.Link{ShortCode: "t3"})
	tags := repository.NewTagRepository(db)
	require.NoError(t, tags.SetLinkTags(ctx, links[0].ID, []string{"news"}))
	require.NoError(t, tags.SetLinkTags(ctx, links[1].ID, []string{"promo", "news"}))
	require.NoError(t, tags.SetLinkTags(ctx, links[2].ID, []string{"other"}))
	repo := repository.NewLinkRepository(db)

	rows, total, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Tags: []string{"news"}}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.ElementsMatch(t, []string{"t1", "t2"}, codesOf(rows))

	// Several tags match any of them; each link appears once.
	rows, total, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Tags: []string{"promo", "other", "news"}}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, rows, 3)
}

func TestLinkRepository_ListByGroup_FilterCustomBackHalf(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "cbh@example.com",
		&model.Link{ShortCode: "rand123"}, &model.Link{ShortCode: "my-brand", HasCustomBackHalf: true})
	repo := repository.NewLinkRepository(db)

	rows, _, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{CustomBackHalf: boolPtr(true)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"my-brand"}, codesOf(rows))

	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{CustomBackHalf: boolPtr(false)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"rand123"}, codesOf(rows))
}

func TestLinkRepository_ListByGroup_FilterExpiration(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	past := model.NullTime{NullTime: sql.NullTime{Time: time.Now().Add(-48 * time.Hour), Valid: true}}
	future := model.NullTime{NullTime: sql.NullTime{Time: time.Now().Add(48 * time.Hour), Valid: true}}
	group, _ := linkFixture(t, db, "exp@example.com",
		&model.Link{ShortCode: "expired", ExpiresAt: past},
		&model.Link{ShortCode: "expiring", ExpiresAt: future},
		&model.Link{ShortCode: "forever"})
	repo := repository.NewLinkRepository(db)

	for filter, want := range map[string]string{"expired": "expired", "expiring": "expiring", "none": "forever"} {
		rows, total, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Expiration: filter}, 20, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total, filter)
		assert.Equal(t, []string{want}, codesOf(rows), filter)
	}
}

func TestLinkRepository_ListByGroup_Query(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	title := "Spring Sale"
	group, links := linkFixture(t, db, "query@example.com",
		&model.Link{ShortCode: "q1", Title: &title},
		&model.Link{ShortCode: "q2", OriginalURL: "https://docs.golang.org/ref"},
		&model.Link{ShortCode: "findme"},
		&model.Link{ShortCode: "q4"},
		&model.Link{ShortCode: "q5"})
	require.NoError(t, repository.NewTagRepository(db).SetLinkTags(ctx, links[3].ID, []string{"campaign-x"}))
	repo := repository.NewLinkRepository(db)

	for q, want := range map[string]string{"spring": "q1", "golang": "q2", "findm": "findme", "campaign-x": "q4"} {
		rows, total, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Query: q}, 20, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total, q)
		assert.Equal(t, []string{want}, codesOf(rows), q)
	}
	// LIKE wildcards in the query are matched literally.
	rows, _, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Query: "%"}, 20, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestLinkRepository_ListByGroup_SortClicks(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "sort@example.com",
		&model.Link{ShortCode: "few"}, &model.Link{ShortCode: "many"}, &model.Link{ShortCode: "none"})
	for i := 0; i < 3; i++ {
		_, err := db.ExecContext(ctx, `INSERT INTO clicks (link_id) VALUES (?)`, links[1].ID)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO clicks (link_id) VALUES (?)`, links[0].ID)
	require.NoError(t, err)
	repo := repository.NewLinkRepository(db)

	rows, total, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{Sort: "clicks"}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, []string{"many", "few", "none"}, codesOf(rows))
	assert.Equal(t, int64(3), rows[0].TotalClicks)
	assert.Equal(t, int64(1), rows[1].TotalClicks)

	// Default sort: newest first; pagination applies.
	rows, total, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{}, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, []string{"none", "many"}, codesOf(rows))
	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{}, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"few"}, codesOf(rows))
}

func TestLinkRepository_ListByGroup_CreatedRange(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := linkFixture(t, db, "range@example.com",
		&model.Link{ShortCode: "old"}, &model.Link{ShortCode: "new"})
	_, err := db.ExecContext(ctx, `UPDATE links SET created_at = '2026-01-10 12:00:00' WHERE id = ?`, links[0].ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE links SET created_at = '2026-03-10 12:00:00' WHERE id = ?`, links[1].ID)
	require.NoError(t, err)
	repo := repository.NewLinkRepository(db)
	after := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	rows, _, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{CreatedAfter: &after}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"new"}, codesOf(rows))
	rows, _, err = repo.ListByGroup(ctx, group.ID, repository.LinkFilter{CreatedBefore: &before}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"old"}, codesOf(rows))
	// HasQR=on matches nothing until QR codes exist.
	rows, total, err := repo.ListByGroup(ctx, group.ID, repository.LinkFilter{HasQR: boolPtr(true)}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows)
}

func TestLinkRepository_SetArchived(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	_, links := linkFixture(t, db, "setarch@example.com",
		&model.Link{ShortCode: "sa1"}, &model.Link{ShortCode: "sa2"}, &model.Link{ShortCode: "sa3"})
	repo := repository.NewLinkRepository(db)

	require.NoError(t, repo.SetArchived(ctx, []uint64{links[0].ID, links[1].ID}, true))
	got, err := repo.GetByIDs(ctx, []uint64{links[0].ID, links[1].ID, links[2].ID})
	require.NoError(t, err)
	require.Len(t, got, 3)
	archived := map[string]bool{}
	for _, l := range got {
		archived[l.ShortCode] = l.Archived
	}
	assert.Equal(t, map[string]bool{"sa1": true, "sa2": true, "sa3": false}, archived)

	require.NoError(t, repo.SetArchived(ctx, []uint64{links[0].ID}, false))
	l, err := repo.GetByID(ctx, links[0].ID)
	require.NoError(t, err)
	assert.False(t, l.Archived)

	// Empty id list is a no-op.
	require.NoError(t, repo.SetArchived(ctx, nil, true))
}

func TestLinkRepository_IncrementRedirectCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	_, links := linkFixture(t, db, "redir@example.com", &model.Link{ShortCode: "rc1"})
	repo := repository.NewLinkRepository(db)
	require.NoError(t, repo.IncrementRedirectCount(ctx, links[0].ID))
	require.NoError(t, repo.IncrementRedirectCount(ctx, links[0].ID))
	l, err := repo.GetByID(ctx, links[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 2, l.RedirectCount)
	assert.ErrorIs(t, repo.IncrementRedirectCount(ctx, 999999999), repository.ErrLinkNotFound)
}

func TestLinkRepository_Update_WritesLongURLHost(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	_, links := linkFixture(t, db, "upd@example.com", &model.Link{ShortCode: "up1"})
	repo := repository.NewLinkRepository(db)
	host := "new.example.org"
	l := links[0]
	l.OriginalURL = "https://new.example.org/p"
	l.LongURLHost = &host
	require.NoError(t, repo.Update(ctx, l))
	got, err := repo.GetByID(ctx, l.ID)
	require.NoError(t, err)
	require.NotNil(t, got.LongURLHost)
	assert.Equal(t, host, *got.LongURLHost)
}

func TestLinkRepository_BackHalfUniquePerDomain(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	_, links := linkFixture(t, db, "perdomain@example.com", &model.Link{ShortCode: "promo"})
	d := &model.Domain{UserID: links[0].UserID, Domain: "go.perdomain.test"}
	require.NoError(t, repository.NewDomainRepository(db).Create(ctx, d))
	repo := repository.NewLinkRepository(db)

	// Same back-half on another domain is allowed.
	onDomain := &model.Link{UserID: links[0].UserID, GroupID: links[0].GroupID, ShortCode: "promo",
		OriginalURL: "https://example.com", IsActive: true, DomainID: &d.ID}
	require.NoError(t, repo.Create(ctx, onDomain))
	exists, err := repo.ShortCodeExistsInDomain(ctx, &d.ID, "promo")
	require.NoError(t, err)
	assert.True(t, exists)

	// Same back-half on the same custom domain is rejected.
	dup := &model.Link{UserID: links[0].UserID, GroupID: links[0].GroupID, ShortCode: "promo",
		OriginalURL: "https://example.com", IsActive: true, DomainID: &d.ID}
	assert.ErrorIs(t, repo.Create(ctx, dup), repository.ErrShortCodeExists)
}

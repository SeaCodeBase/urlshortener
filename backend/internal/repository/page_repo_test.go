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

func newPage(group *model.Group, slug string) *model.Page {
	return &model.Page{
		UserID:  group.UserID,
		GroupID: group.ID,
		Slug:    slug,
		Title:   "Title " + slug,
		Design:  model.JSONMap{"theme": "classic"},
	}
}

func createPage(t *testing.T, repo *repository.PageRepositoryImpl, group *model.Group, slug string) *model.Page {
	t.Helper()
	p := newPage(group, slug)
	require.NoError(t, repo.Create(context.Background(), p))
	return p
}

func createBlock(t *testing.T, repo *repository.PageRepositoryImpl, pageID uint64, typ string, linkID *uint64) *model.PageBlock {
	t.Helper()
	b := &model.PageBlock{PageID: pageID, Type: typ, Content: model.JSONMap{"heading": "h"}, IsActive: true, LinkID: linkID}
	require.NoError(t, repo.CreateBlock(context.Background(), b))
	return b
}

func blockIDs(rows []model.PageBlock) []uint64 {
	out := make([]uint64, len(rows))
	for i, b := range rows {
		out[i] = b.ID
	}
	return out
}

func pageFixture(t *testing.T, db *sqlx.DB, email string, links ...*model.Link) (*model.Group, []*model.Link) {
	t.Helper()
	return linkFixture(t, db, email, links...)
}

func TestPageRepo_CreateGetSlug(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "page.crud@example.com")
	repo := repository.NewPageRepository(db)

	p := createPage(t, repo, group, "jose")
	require.NotZero(t, p.ID)
	assert.Equal(t, model.PageStatusDraft, p.Status)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "jose", got.Slug)
	assert.Equal(t, "Title jose", got.Title)
	assert.Equal(t, group.ID, got.GroupID)
	assert.Equal(t, group.UserID, got.UserID)
	assert.Equal(t, "classic", got.Design["theme"])
	assert.Equal(t, model.PageStatusDraft, got.Status)
	assert.Nil(t, got.PublishedContent, "NULL published_content scans to nil")
	assert.Nil(t, got.PublishedAt)
	assert.False(t, got.Archived)

	bySlug, err := repo.GetBySlug(ctx, "JOSE")
	require.NoError(t, err)
	assert.Equal(t, p.ID, bySlug.ID)
	_, err = repo.GetBySlug(ctx, "nobody")
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
	_, err = repo.GetByID(ctx, p.ID+999)
	assert.ErrorIs(t, err, repository.ErrPageNotFound)

	exists, err := repo.SlugExists(ctx, "jose")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = repo.SlugExists(ctx, "Jose")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = repo.SlugExists(ctx, "other")
	require.NoError(t, err)
	assert.False(t, exists)

	got.Title, got.Description, got.AvatarURL = "New", "Desc", "https://example.com/a.png"
	got.Design = model.JSONMap{"theme": "bold"}
	require.NoError(t, repo.Update(ctx, got))
	got2, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", got2.Title)
	assert.Equal(t, "Desc", got2.Description)
	assert.Equal(t, "https://example.com/a.png", got2.AvatarURL)
	assert.Equal(t, "bold", got2.Design["theme"])
	got2.ID = p.ID + 999
	assert.ErrorIs(t, repo.Update(ctx, got2), repository.ErrPageNotFound)

	require.NoError(t, repo.SetArchived(ctx, p.ID, true))
	got3, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.True(t, got3.Archived)
	require.NoError(t, repo.SetArchived(ctx, p.ID, true), "no-op archive is not an error")
	assert.ErrorIs(t, repo.SetArchived(ctx, p.ID+999, true), repository.ErrPageNotFound)

	require.NoError(t, repo.Delete(ctx, p.ID))
	assert.ErrorIs(t, repo.Delete(ctx, p.ID), repository.ErrPageNotFound)
	_, err = repo.GetByID(ctx, p.ID)
	assert.ErrorIs(t, err, repository.ErrPageNotFound)
}

func TestPageRepo_SlugUniqueCaseInsensitive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "page.uniq@example.com")
	other, _ := pageFixture(t, db, "page.uniq2@example.com")
	repo := repository.NewPageRepository(db)

	createPage(t, repo, group, "jose")
	err := repo.Create(ctx, newPage(other, "Jose"))
	assert.ErrorIs(t, err, repository.ErrPageSlugTaken)
	err = repo.Create(ctx, newPage(group, "jose"))
	assert.ErrorIs(t, err, repository.ErrPageSlugTaken)
}

func TestPageRepo_Blocks_CRUD_Reorder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := pageFixture(t, db, "page.blocks@example.com", &model.Link{ShortCode: "pgblk1"})
	repo := repository.NewPageRepository(db)
	p := createPage(t, repo, group, "blocks")
	other := createPage(t, repo, group, "otherpage")

	none, err := repo.MaxBlockUpdatedAt(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, none)

	a := createBlock(t, repo, p.ID, model.PageBlockText, nil)
	b := createBlock(t, repo, p.ID, model.PageBlockLink, &links[0].ID)
	c := createBlock(t, repo, p.ID, model.PageBlockImage, nil)
	foreign := createBlock(t, repo, other.ID, model.PageBlockText, nil)
	assert.Equal(t, 0, a.Position)
	assert.Equal(t, 1, b.Position)
	assert.Equal(t, 2, c.Position)
	assert.Equal(t, 0, foreign.Position)
	assert.False(t, a.CreatedAt.IsZero())

	gotB, err := repo.GetBlock(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PageBlockLink, gotB.Type)
	require.NotNil(t, gotB.LinkID)
	assert.Equal(t, links[0].ID, *gotB.LinkID)
	assert.True(t, gotB.IsActive)
	assert.False(t, gotB.IsPinned)
	assert.Equal(t, "h", gotB.Content["heading"])
	_, err = repo.GetBlock(ctx, c.ID+999)
	assert.ErrorIs(t, err, repository.ErrBlockNotFound)

	gotB.Content = model.JSONMap{"title": "Menu"}
	gotB.IsActive, gotB.IsPinned = false, true
	require.NoError(t, repo.UpdateBlock(ctx, gotB))
	gotB2, err := repo.GetBlock(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "Menu", gotB2.Content["title"])
	assert.False(t, gotB2.IsActive)
	assert.True(t, gotB2.IsPinned)
	gotB2.ID = c.ID + 999
	assert.ErrorIs(t, repo.UpdateBlock(ctx, gotB2), repository.ErrBlockNotFound)

	maxAt, err := repo.MaxBlockUpdatedAt(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, maxAt)

	rows, err := repo.ListBlocks(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{a.ID, b.ID, c.ID}, blockIDs(rows))

	// Positions follow the order of ids.
	require.NoError(t, repo.ReorderBlocks(ctx, p.ID, []uint64{c.ID, a.ID, b.ID}))
	rows, err = repo.ListBlocks(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{c.ID, a.ID, b.ID}, blockIDs(rows))
	for i, r := range rows {
		assert.Equal(t, i, r.Position)
	}

	// Foreign, missing, duplicate and extra ids are rejected; nothing changes.
	for name, ids := range map[string][]uint64{
		"foreign":   {c.ID, a.ID, foreign.ID},
		"missing":   {c.ID, a.ID},
		"duplicate": {c.ID, a.ID, a.ID},
		"extra":     {c.ID, a.ID, b.ID, foreign.ID},
		"unknown":   {c.ID, a.ID, b.ID + 999},
	} {
		err := repo.ReorderBlocks(ctx, p.ID, ids)
		assert.ErrorIs(t, err, repository.ErrInvalidBlockOrder, name)
	}
	rows, err = repo.ListBlocks(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{c.ID, a.ID, b.ID}, blockIDs(rows))
	otherRows, err := repo.ListBlocks(ctx, other.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, otherRows[0].Position)

	// A new block goes after the current max.
	d := createBlock(t, repo, p.ID, model.PageBlockText, nil)
	assert.Equal(t, 3, d.Position)

	require.NoError(t, repo.DeleteBlock(ctx, a.ID))
	assert.ErrorIs(t, repo.DeleteBlock(ctx, a.ID), repository.ErrBlockNotFound)
	rows, err = repo.ListBlocks(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{c.ID, b.ID, d.ID}, blockIDs(rows))

	// Deleting the page removes its blocks.
	require.NoError(t, repo.Delete(ctx, p.ID))
	_, err = repo.GetBlock(ctx, c.ID)
	assert.ErrorIs(t, err, repository.ErrBlockNotFound)
}

func TestPageRepo_BlockCascadesOnLinkDelete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := pageFixture(t, db, "page.cascade@example.com",
		&model.Link{ShortCode: "pgcas1"}, &model.Link{ShortCode: "pgcas2"})
	repo := repository.NewPageRepository(db)
	p := createPage(t, repo, group, "cascade")
	gone := createBlock(t, repo, p.ID, model.PageBlockLink, &links[0].ID)
	kept := createBlock(t, repo, p.ID, model.PageBlockLink, &links[1].ID)
	text := createBlock(t, repo, p.ID, model.PageBlockText, nil)

	_, err := db.ExecContext(ctx, "DELETE FROM links WHERE id = ?", links[0].ID)
	require.NoError(t, err)

	_, err = repo.GetBlock(ctx, gone.ID)
	assert.ErrorIs(t, err, repository.ErrBlockNotFound)
	rows, err := repo.ListBlocks(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{kept.ID, text.ID}, blockIDs(rows))
}

func TestPageRepo_PagesForLinks_ExcludesArchived(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, links := pageFixture(t, db, "page.forlinks@example.com",
		&model.Link{ShortCode: "pgfl1"}, &model.Link{ShortCode: "pgfl2"}, &model.Link{ShortCode: "pgfl3"})
	repo := repository.NewPageRepository(db)
	p1 := createPage(t, repo, group, "first")
	p2 := createPage(t, repo, group, "second")
	archived := createPage(t, repo, group, "archived")
	require.NoError(t, repo.SetArchived(ctx, archived.ID, true))

	l1, l2, l3 := links[0].ID, links[1].ID, links[2].ID
	createBlock(t, repo, p2.ID, model.PageBlockLink, &l1)
	createBlock(t, repo, p1.ID, model.PageBlockLink, &l1)
	createBlock(t, repo, p1.ID, model.PageBlockLink, &l1) // duplicate on the same page
	createBlock(t, repo, archived.ID, model.PageBlockLink, &l1)
	createBlock(t, repo, archived.ID, model.PageBlockLink, &l2)
	createBlock(t, repo, p2.ID, model.PageBlockLink, &l3)
	createBlock(t, repo, p1.ID, model.PageBlockText, nil)

	got, err := repo.PagesForLinks(ctx, []uint64{l1, l2, l3})
	require.NoError(t, err)
	assert.Equal(t, map[uint64][]uint64{
		l1: {p1.ID, p2.ID},
		l3: {p2.ID},
	}, got)

	empty, err := repo.PagesForLinks(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.NotNil(t, empty)
}

func TestPageRepo_Publish_SetsSnapshot(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "page.publish@example.com")
	repo := repository.NewPageRepository(db)
	p := createPage(t, repo, group, "publish")

	at := time.Date(2026, 10, 25, 12, 30, 0, 0, time.UTC)
	snapshot := model.JSONMap{"slug": "publish", "blocks": []any{map[string]any{"type": "text"}}}
	require.NoError(t, repo.Publish(ctx, p.ID, snapshot, at))

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PageStatusPublished, got.Status)
	require.NotNil(t, got.PublishedAt)
	assert.True(t, at.Equal(*got.PublishedAt), "published_at %v", got.PublishedAt)
	require.NotNil(t, got.PublishedContent)
	assert.Equal(t, "publish", got.PublishedContent["slug"])
	assert.Len(t, got.PublishedContent["blocks"], 1)

	bySlug, err := repo.GetBySlug(ctx, "publish")
	require.NoError(t, err)
	assert.Equal(t, "publish", bySlug.PublishedContent["slug"])

	// Later edits do not touch the snapshot.
	got.Title = "Edited"
	require.NoError(t, repo.Update(ctx, got))
	again, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Edited", again.Title)
	assert.Equal(t, "publish", again.PublishedContent["slug"])

	assert.ErrorIs(t, repo.Publish(ctx, p.ID+999, snapshot, at), repository.ErrPageNotFound)
}

func TestPageRepo_ListByGroup_FilterAndViews(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := pageFixture(t, db, "page.list@example.com")
	other, _ := pageFixture(t, db, "page.list2@example.com")
	repo := repository.NewPageRepository(db)

	old := createPage(t, repo, group, "old-page")
	old.Title = "Coffee menu"
	require.NoError(t, repo.Update(ctx, old))
	mid := createPage(t, repo, group, "under_score")
	newest := createPage(t, repo, group, "newest")
	arch := createPage(t, repo, group, "archived-coffee")
	require.NoError(t, repo.SetArchived(ctx, arch.ID, true))
	createPage(t, repo, other, "other-coffee")

	now := time.Now().UTC()
	for id, offset := range map[uint64]time.Duration{old.ID: 3 * time.Hour, mid.ID: 2 * time.Hour, newest.ID: time.Hour} {
		_, err := db.ExecContext(ctx, "UPDATE pages SET created_at = ? WHERE id = ?", now.Add(-offset), id)
		require.NoError(t, err)
	}
	for i := 0; i < 3; i++ {
		_, err := db.ExecContext(ctx, "INSERT INTO page_views (page_id) VALUES (?)", old.ID)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, "INSERT INTO page_views (page_id) VALUES (?)", newest.ID)
	require.NoError(t, err)

	slugs := func(rows []repository.PageWithViews) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = r.Slug
		}
		return out
	}

	all, err := repo.ListByGroup(ctx, group.ID, repository.PageFilter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"newest", "under_score", "old-page"}, slugs(all))
	views := map[string]int64{}
	for _, r := range all {
		views[r.Slug] = r.Views
	}
	assert.Equal(t, map[string]int64{"newest": 1, "under_score": 0, "old-page": 3}, views)
	assert.Equal(t, "classic", all[0].Design["theme"])

	byTitle, err := repo.ListByGroup(ctx, group.ID, repository.PageFilter{Query: "coffee"})
	require.NoError(t, err)
	assert.Equal(t, []string{"old-page"}, slugs(byTitle))
	bySlug, err := repo.ListByGroup(ctx, group.ID, repository.PageFilter{Query: "NEW"})
	require.NoError(t, err)
	assert.Equal(t, []string{"newest"}, slugs(bySlug))
	// "_" is matched literally, not as a single-char wildcard.
	literal, err := repo.ListByGroup(ctx, group.ID, repository.PageFilter{Query: "r_s"})
	require.NoError(t, err)
	assert.Equal(t, []string{"under_score"}, slugs(literal))
	wild, err := repo.ListByGroup(ctx, group.ID, repository.PageFilter{Query: "d_p"})
	require.NoError(t, err)
	assert.Empty(t, wild)

	archived, err := repo.ListByGroup(ctx, group.ID, repository.PageFilter{Archived: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"archived-coffee"}, slugs(archived))
	assert.NotNil(t, archived)

	none, err := repo.ListByGroup(ctx, group.ID+999, repository.PageFilter{})
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

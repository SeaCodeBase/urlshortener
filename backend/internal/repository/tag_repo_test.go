package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestGroupAndLinks(t *testing.T, db *sqlx.DB, email string, codes ...string) (*model.Group, []*model.Link) {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, db, email)
	group := &model.Group{UserID: user.ID, Name: email}
	require.NoError(t, repository.NewGroupRepository(db).Create(ctx, group))
	links := make([]*model.Link, 0, len(codes))
	for _, code := range codes {
		l := &model.Link{UserID: user.ID, GroupID: group.ID, ShortCode: code, OriginalURL: "https://example.com/" + code, IsActive: true}
		require.NoError(t, repository.NewLinkRepository(db).Create(ctx, l))
		links = append(links, l)
	}
	return group, links
}

func TestTagRepository_SetAndGet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := repository.NewTagRepository(db)
	_, links := createTestGroupAndLinks(t, db, "tags@example.com", "tg1", "tg2")

	long := strings.Repeat("x", 120)
	require.NoError(t, repo.SetLinkTags(ctx, links[0].ID, []string{" news ", "promo", "news", "", "   ", long}))

	got, err := repo.GetLinkTags(ctx, links[0].ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"news", "promo", strings.Repeat("x", 100)}, got)

	// Replacing the set removes old tags.
	require.NoError(t, repo.SetLinkTags(ctx, links[0].ID, []string{"spring"}))
	got, err = repo.GetLinkTags(ctx, links[0].ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"spring"}, got)

	// A link without tags returns an empty, non-nil slice.
	got, err = repo.GetLinkTags(ctx, links[1].ID)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)

	// Clearing works with nil.
	require.NoError(t, repo.SetLinkTags(ctx, links[0].ID, nil))
	got, err = repo.GetLinkTags(ctx, links[0].ID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTagRepository_ListByGroup_Distinct(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := repository.NewTagRepository(db)
	group, links := createTestGroupAndLinks(t, db, "tags-a@example.com", "ta1", "ta2")
	_, others := createTestGroupAndLinks(t, db, "tags-b@example.com", "tb1")

	require.NoError(t, repo.SetLinkTags(ctx, links[0].ID, []string{"beta", "alpha"}))
	require.NoError(t, repo.SetLinkTags(ctx, links[1].ID, []string{"alpha", "gamma"}))
	require.NoError(t, repo.SetLinkTags(ctx, others[0].ID, []string{"other"}))

	tags, err := repo.ListByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, tags)

	empty, err := repo.ListByGroup(ctx, 999999)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestTagRepository_BulkAddRemoveAndBatchGet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := repository.NewTagRepository(db)
	_, links := createTestGroupAndLinks(t, db, "tags-bulk@example.com", "tk1", "tk2", "tk3")
	ids := []uint64{links[0].ID, links[1].ID}

	require.NoError(t, repo.SetLinkTags(ctx, links[0].ID, []string{"keep"}))
	// Adding a tag a link already has is not an error.
	require.NoError(t, repo.AddTagsToLinks(ctx, ids, []string{"keep", " sale "}))

	byLink, err := repo.GetTagsForLinks(ctx, []uint64{links[0].ID, links[1].ID, links[2].ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"keep", "sale"}, byLink[links[0].ID])
	assert.ElementsMatch(t, []string{"keep", "sale"}, byLink[links[1].ID])
	assert.Empty(t, byLink[links[2].ID])

	require.NoError(t, repo.RemoveTagsFromLinks(ctx, ids, []string{"keep"}))
	byLink, err = repo.GetTagsForLinks(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, []string{"sale"}, byLink[links[0].ID])
	assert.Equal(t, []string{"sale"}, byLink[links[1].ID])

	// Empty inputs are no-ops.
	require.NoError(t, repo.AddTagsToLinks(ctx, nil, []string{"x"}))
	require.NoError(t, repo.AddTagsToLinks(ctx, ids, nil))
	require.NoError(t, repo.RemoveTagsFromLinks(ctx, nil, []string{"x"}))
	m, err := repo.GetTagsForLinks(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, m)
}

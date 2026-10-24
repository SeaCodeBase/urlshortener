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

func TestActivityRepository_InsertAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := repository.NewActivityRepository(db)
	group, _ := createTestGroupAndLinks(t, db, "act@example.com")
	other, _ := createTestGroupAndLinks(t, db, "act-other@example.com")

	e1 := &model.ActivityEntry{UserID: group.UserID, GroupID: group.ID, Action: "link.create", EntityType: "link", EntityID: 1, Metadata: model.JSONMap{"short_code": "abc"}}
	require.NoError(t, repo.Insert(ctx, e1))
	assert.NotZero(t, e1.ID)
	assert.False(t, e1.CreatedAt.IsZero())

	e2 := &model.ActivityEntry{UserID: group.UserID, GroupID: group.ID, Action: "link.delete", EntityType: "link", EntityID: 1}
	require.NoError(t, repo.Insert(ctx, e2))
	require.NoError(t, repo.Insert(ctx, &model.ActivityEntry{UserID: other.UserID, GroupID: other.ID, Action: "link.create", EntityType: "link", EntityID: 2}))

	// Backdate e1 to make the date filter observable.
	old := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	_, err := db.Exec("UPDATE activity_log SET created_at = ? WHERE id = ?", old, e1.ID)
	require.NoError(t, err)

	entries, total, err := repo.List(ctx, group.ID, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, entries, 2)
	assert.Equal(t, e2.ID, entries[0].ID, "newest first")
	assert.Equal(t, "abc", entries[1].Metadata["short_code"])
	assert.NotNil(t, entries[0].Metadata, "NULL metadata scans to empty map")

	// Pagination.
	entries, total, err = repo.List(ctx, group.ID, nil, nil, 1, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, entries, 1)
	assert.Equal(t, e1.ID, entries[0].ID)

	// from inclusive, to exclusive.
	from := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)
	entries, total, err = repo.List(ctx, group.ID, &from, &to, 10, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, e1.ID, entries[0].ID)

	entries, total, err = repo.List(ctx, group.ID, &to, nil, 10, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, e2.ID, entries[0].ID)

	entries, total, err = repo.List(ctx, 999999, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, entries)
}

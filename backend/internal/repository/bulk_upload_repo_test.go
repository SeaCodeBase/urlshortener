package repository_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createBulk(t *testing.T, repo *repository.BulkUploadRepositoryImpl, group *model.Group, filename string) *model.BulkUpload {
	t.Helper()
	b := &model.BulkUpload{UserID: group.UserID, GroupID: group.ID, Filename: filename, Total: 2,
		SourceCSV: "long_url,title,custom_back_half,tags,domain\nhttps://a.example,,,,\nhttps://b.example,,,,\n"}
	require.NoError(t, repo.Create(context.Background(), b))
	require.NotZero(t, b.ID)
	return b
}

func TestBulkRepo_CreateGetProgressFinish(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "bulk.crud@example.com")
	repo := repository.NewBulkUploadRepository(db)

	b := createBulk(t, repo, group, "a.csv")
	assert.Equal(t, model.BulkStatusQueued, b.Status)
	assert.False(t, b.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "a.csv", got.Filename)
	assert.Equal(t, 2, got.Total)
	assert.Equal(t, b.SourceCSV, got.SourceCSV)
	assert.Nil(t, got.ResultCSV)
	_, err = repo.GetByID(ctx, b.ID+999)
	assert.ErrorIs(t, err, repository.ErrBulkUploadNotFound)

	require.NoError(t, repo.UpdateProgress(ctx, b.ID, 2, 1, 1))
	require.NoError(t, repo.UpdateProgress(ctx, b.ID, 2, 1, 1), "no-op progress is not an error")
	assert.ErrorIs(t, repo.UpdateProgress(ctx, b.ID+999, 1, 1, 0), repository.ErrBulkUploadNotFound)
	got, err = repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.Processed)
	assert.Equal(t, 1, got.Succeeded)
	assert.Equal(t, 1, got.Failed)

	require.NoError(t, repo.Finish(ctx, b.ID, model.BulkStatusDone, "row,long_url\n", ""))
	got, err = repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, model.BulkStatusDone, got.Status)
	require.NotNil(t, got.ResultCSV)
	assert.Equal(t, "row,long_url\n", *got.ResultCSV)
	assert.Equal(t, "", got.Error)

	f := createBulk(t, repo, group, "f.csv")
	require.NoError(t, repo.Finish(ctx, f.ID, model.BulkStatusFailed, "", "db down"))
	got, err = repo.GetByID(ctx, f.ID)
	require.NoError(t, err)
	assert.Equal(t, model.BulkStatusFailed, got.Status)
	assert.Nil(t, got.ResultCSV, "empty result stays NULL")
	assert.Equal(t, "db down", got.Error)
	assert.ErrorIs(t, repo.Finish(ctx, f.ID+999, model.BulkStatusDone, "", ""), repository.ErrBulkUploadNotFound)
}

func TestBulkRepo_ClaimNextIsAtomic(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "bulk.claim@example.com")
	repo := repository.NewBulkUploadRepository(db)

	first := createBulk(t, repo, group, "1.csv")
	second := createBulk(t, repo, group, "2.csv")
	done := createBulk(t, repo, group, "done.csv")
	require.NoError(t, repo.Finish(ctx, done.ID, model.BulkStatusDone, "x", ""))

	a, err := repo.ClaimNext(ctx)
	require.NoError(t, err)
	require.NotNil(t, a)
	b, err := repo.ClaimNext(ctx)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, first.ID, a.ID, "oldest first")
	assert.Equal(t, second.ID, b.ID)
	assert.Equal(t, model.BulkStatusProcessing, a.Status)
	assert.Equal(t, model.BulkStatusProcessing, b.Status)
	assert.Equal(t, first.SourceCSV, a.SourceCSV, "claimed job carries its CSV")

	c, err := repo.ClaimNext(ctx)
	require.NoError(t, err)
	assert.Nil(t, c)

	got, err := repo.GetByID(ctx, first.ID)
	require.NoError(t, err)
	assert.Equal(t, model.BulkStatusProcessing, got.Status)
}

func TestBulkRepo_ListOmitsCSVBodies(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "bulk.list@example.com")
	other, _ := linkFixture(t, db, "bulk.list2@example.com")
	repo := repository.NewBulkUploadRepository(db)

	a := createBulk(t, repo, group, "a.csv")
	b := createBulk(t, repo, group, "b.csv")
	require.NoError(t, repo.Finish(ctx, a.ID, model.BulkStatusDone, "result", ""))
	createBulk(t, repo, other, "other.csv")

	list, err := repo.ListByGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, b.ID, list[0].ID, "newest first")
	assert.Equal(t, a.ID, list[1].ID)
	for _, u := range list {
		assert.Empty(t, u.SourceCSV)
		assert.Nil(t, u.ResultCSV)
		assert.NotEmpty(t, u.Filename)
		assert.Equal(t, 2, u.Total)
	}
	assert.Equal(t, model.BulkStatusDone, list[1].Status)
}

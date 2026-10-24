package repository_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dashboardFixture(t *testing.T, db *sqlx.DB, email string) *model.Group {
	t.Helper()
	group, _ := linkFixture(t, db, email)
	return group
}

func newDashboard(group *model.Group, name string) *model.Dashboard {
	return &model.Dashboard{
		GroupID:   group.ID,
		CreatedBy: group.UserID,
		Name:      name,
		DateRange: "last_7_days",
		Filter:    model.JSONMap{"type": "all"},
		Layout:    model.JSONMap{"modules": []any{}},
	}
}

func dashboardNames(rows []model.Dashboard) []string {
	out := make([]string, len(rows))
	for i, d := range rows {
		out[i] = d.Name
	}
	return out
}

func TestDashboardRepo_CreateGetListDelete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group := dashboardFixture(t, db, "dash.crud@example.com")
	other := dashboardFixture(t, db, "dash.other@example.com")
	repo := repository.NewDashboardRepository(db)

	first := newDashboard(group, "Zeta")
	require.NoError(t, repo.Create(ctx, first))
	require.NotZero(t, first.ID)
	assert.False(t, first.CreatedAt.IsZero())
	assert.False(t, first.UpdatedAt.IsZero())
	second := newDashboard(group, "Alpha")
	require.NoError(t, repo.Create(ctx, second))
	require.NoError(t, repo.Create(ctx, newDashboard(other, "Other group")))
	// Make the creation order observable at second granularity.
	_, err := db.ExecContext(ctx, "UPDATE dashboards SET created_at = ?, updated_at = ? WHERE id = ?",
		time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-time.Minute), first.ID)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alpha", got.Name)
	assert.Equal(t, group.ID, got.GroupID)
	assert.Equal(t, group.UserID, got.CreatedBy)
	assert.False(t, got.IsDefault)
	assert.Equal(t, "last_7_days", got.DateRange)
	assert.Equal(t, "all", got.Filter["type"])

	byCreated, err := repo.ListByGroup(ctx, group.ID, "created")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alpha", "Zeta"}, dashboardNames(byCreated))
	byDefault, err := repo.ListByGroup(ctx, group.ID, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alpha", "Zeta"}, dashboardNames(byDefault))
	byName, err := repo.ListByGroup(ctx, group.ID, "name")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alpha", "Zeta"}, dashboardNames(byName))
	_, err = db.ExecContext(ctx, "UPDATE dashboards SET updated_at = ? WHERE id = ?",
		time.Now().UTC().Add(time.Minute), first.ID)
	require.NoError(t, err)
	byModified, err := repo.ListByGroup(ctx, group.ID, "modified")
	require.NoError(t, err)
	assert.Equal(t, []string{"Zeta", "Alpha"}, dashboardNames(byModified))

	count, err := repo.CountByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	require.NoError(t, repo.Delete(ctx, second.ID))
	_, err = repo.GetByID(ctx, second.ID)
	assert.ErrorIs(t, err, repository.ErrDashboardNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, second.ID), repository.ErrDashboardNotFound)
	count, err = repo.CountByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	empty, err := repo.ListByGroup(ctx, group.ID+1000, "created")
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestDashboardRepo_EnsureDefault_Idempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group := dashboardFixture(t, db, "dash.default@example.com")
	repo := repository.NewDashboardRepository(db)
	layout := model.JSONMap{"modules": []any{map[string]any{"key": "top_day", "viz": "scorecard"}}}

	// A non-default dashboard does not count as the default one.
	require.NoError(t, repo.Create(ctx, newDashboard(group, "Mine")))

	d1, err := repo.EnsureDefault(ctx, group.ID, group.UserID, layout)
	require.NoError(t, err)
	assert.True(t, d1.IsDefault)
	assert.Equal(t, "Default", d1.Name)
	assert.Equal(t, "last_7_days", d1.DateRange)
	assert.Equal(t, "all", d1.Filter["type"])
	assert.Equal(t, group.UserID, d1.CreatedBy)
	require.Len(t, d1.Layout["modules"], 1)

	d2, err := repo.EnsureDefault(ctx, group.ID, group.UserID, layout)
	require.NoError(t, err)
	assert.Equal(t, d1.ID, d2.ID)

	// Concurrent first calls still leave exactly one default row.
	other := dashboardFixture(t, db, "dash.default2@example.com")
	var wg sync.WaitGroup
	ids := make([]uint64, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d, err := repo.EnsureDefault(ctx, other.ID, other.UserID, layout)
			if assert.NoError(t, err) {
				ids[i] = d.ID
			}
		}(i)
	}
	wg.Wait()
	for _, id := range ids {
		assert.Equal(t, ids[0], id)
	}

	count, err := repo.CountByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	count, err = repo.CountByGroup(ctx, other.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestDashboardRepo_Update_PersistsJSON(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group := dashboardFixture(t, db, "dash.update@example.com")
	repo := repository.NewDashboardRepository(db)
	d := newDashboard(group, "Before")
	require.NoError(t, repo.Create(ctx, d))

	d.Name = "After"
	d.DateRange = "last_30_days"
	d.Filter = model.JSONMap{"type": "links", "ids": []any{1, 2}}
	d.Layout = model.JSONMap{"modules": []any{map[string]any{"key": "over_time", "viz": "bar"}}}
	require.NoError(t, repo.Update(ctx, d))
	// Writing identical values again is not "not found".
	require.NoError(t, repo.Update(ctx, d))

	got, err := repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "After", got.Name)
	assert.Equal(t, "last_30_days", got.DateRange)
	assert.Equal(t, model.JSONMap{"type": "links", "ids": []any{float64(1), float64(2)}}, got.Filter)
	assert.Equal(t, model.JSONMap{"modules": []any{map[string]any{"key": "over_time", "viz": "bar"}}}, got.Layout)

	missing := *d
	missing.ID = d.ID + 1000
	assert.ErrorIs(t, repo.Update(ctx, &missing), repository.ErrDashboardNotFound)
}

func TestDashboardRepo_CreatorNames(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	named := createTestUser(t, db, "named@example.com")
	plain := createTestUser(t, db, "plain@example.com")
	require.NoError(t, repository.NewUserRepository(db).UpdateDisplayName(ctx, named.ID, "Ada"))
	repo := repository.NewDashboardRepository(db)

	names, err := repo.CreatorNames(ctx, []uint64{named.ID, plain.ID, plain.ID + 1000})
	require.NoError(t, err)
	assert.Equal(t, map[uint64]string{named.ID: "Ada", plain.ID: "plain@example.com"}, names)

	none, err := repo.CreatorNames(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

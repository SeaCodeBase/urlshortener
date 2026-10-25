package repository_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createWebhook(t *testing.T, repo *repository.WebhookRepositoryImpl, group *model.Group, name string) *model.Webhook {
	t.Helper()
	w := &model.Webhook{UserID: group.UserID, GroupID: group.ID, Name: name,
		URL: "https://hooks.example.com/" + name, Event: "engagement", Secret: "s3cr3t-" + name}
	require.NoError(t, repo.Create(context.Background(), w))
	require.NotZero(t, w.ID)
	return w
}

func TestWebhookRepo_CRUD(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "wh.crud@example.com")
	repo := repository.NewWebhookRepository(db)

	w := createWebhook(t, repo, group, "one")
	assert.True(t, w.IsActive)
	assert.Equal(t, model.WebhookStatusActive, w.Status)
	assert.False(t, w.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://hooks.example.com/one", got.URL)
	assert.Equal(t, "s3cr3t-one", got.Secret)
	assert.Equal(t, "engagement", got.Event)
	assert.Equal(t, 0, got.ConsecutiveFailures)
	assert.False(t, got.FailingSince.Valid)
	assert.False(t, got.LastDeliveryAt.Valid)
	_, err = repo.GetByID(ctx, w.ID+999)
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)

	now := got.CreatedAt
	got.Name, got.URL = "renamed", "https://hooks.example.com/new"
	got.Status, got.ConsecutiveFailures = model.WebhookStatusAlert, 2
	got.FailingSince.Valid, got.FailingSince.Time = true, now
	got.LastDeliveryAt.Valid, got.LastDeliveryAt.Time = true, now
	require.NoError(t, repo.Update(ctx, got))
	got2, err := repo.GetByID(ctx, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got2.Name)
	assert.Equal(t, "https://hooks.example.com/new", got2.URL)
	assert.Equal(t, model.WebhookStatusAlert, got2.Status)
	assert.Equal(t, 2, got2.ConsecutiveFailures)
	assert.True(t, got2.FailingSince.Valid)
	assert.True(t, got2.LastDeliveryAt.Valid)
	require.NoError(t, repo.Update(ctx, got2), "no-op update is not an error")
	got2.ID = w.ID + 999
	assert.ErrorIs(t, repo.Update(ctx, got2), repository.ErrWebhookNotFound)

	list, err := repo.ListByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Delete(ctx, w.ID))
	assert.ErrorIs(t, repo.Delete(ctx, w.ID), repository.ErrWebhookNotFound)
}

func TestWebhookRepo_ListActiveExcludesDeactivated(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "wh.active@example.com")
	other, _ := linkFixture(t, db, "wh.active2@example.com")
	repo := repository.NewWebhookRepository(db)

	active := createWebhook(t, repo, group, "active")
	alert := createWebhook(t, repo, group, "alert")
	alert.Status = model.WebhookStatusAlert
	require.NoError(t, repo.Update(ctx, alert))
	deact := createWebhook(t, repo, group, "deact")
	deact.Status, deact.IsActive = model.WebhookStatusDeactivated, false
	require.NoError(t, repo.Update(ctx, deact))
	off := createWebhook(t, repo, group, "off")
	off.IsActive = false
	require.NoError(t, repo.Update(ctx, off))
	createWebhook(t, repo, other, "foreign")

	list, err := repo.ListActiveByGroup(ctx, group.ID)
	require.NoError(t, err)
	ids := []uint64{}
	for _, w := range list {
		ids = append(ids, w.ID)
	}
	assert.ElementsMatch(t, []uint64{active.ID, alert.ID}, ids)

	all, err := repo.ListByGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Len(t, all, 4)
}

func TestWebhookRepo_DeliveriesKeepNewest50(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "wh.deliv@example.com")
	repo := repository.NewWebhookRepository(db)
	w := createWebhook(t, repo, group, "d")
	w2 := createWebhook(t, repo, group, "d2")
	require.NoError(t, repo.AddDelivery(ctx, &model.WebhookDelivery{WebhookID: w2.ID, EventID: "other", StatusCode: 200}))

	for i := 1; i <= 55; i++ {
		d := &model.WebhookDelivery{WebhookID: w.ID, EventID: fmt.Sprintf("ev-%d", i), StatusCode: 200, Attempt: 1}
		if i%2 == 0 {
			d.StatusCode, d.Error = 500, "boom"
		}
		require.NoError(t, repo.AddDelivery(ctx, d))
		require.NotZero(t, d.ID)
	}
	var n int
	require.NoError(t, db.Get(&n, `SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = ?`, w.ID))
	assert.Equal(t, 50, n)

	got, err := repo.ListDeliveries(ctx, w.ID, 20)
	require.NoError(t, err)
	require.Len(t, got, 20)
	assert.Equal(t, "ev-55", got[0].EventID, "newest first")
	assert.Equal(t, "ev-36", got[19].EventID)
	assert.Equal(t, 1, got[0].Attempt)
	assert.Equal(t, 200, got[0].StatusCode)
	assert.Equal(t, 500, got[1].StatusCode)
	assert.Equal(t, "boom", got[1].Error)
	assert.False(t, got[0].DeliveredAt.IsZero())

	all, err := repo.ListDeliveries(ctx, w.ID, 100)
	require.NoError(t, err)
	require.Len(t, all, 50)
	assert.Equal(t, "ev-6", all[49].EventID, "oldest five pruned")

	other, err := repo.ListDeliveries(ctx, w2.ID, 20)
	require.NoError(t, err)
	assert.Len(t, other, 1, "pruning is per webhook")

	// Deleting the webhook removes its deliveries.
	require.NoError(t, repo.Delete(ctx, w.ID))
	require.NoError(t, db.Get(&n, `SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = ?`, w.ID))
	assert.Zero(t, n)
}

// failDelivery is a stand-in for service.ApplyDeliveryResult(w, false, now).
func failDelivery(now time.Time) func(w *model.Webhook) {
	return func(w *model.Webhook) {
		w.ConsecutiveFailures++
		if !w.FailingSince.Valid {
			w.FailingSince.Time, w.FailingSince.Valid = now, true
		}
		if w.ConsecutiveFailures >= 2 {
			w.Status = model.WebhookStatusAlert
		}
		w.IsActive = true
		w.LastDeliveryAt.Time, w.LastDeliveryAt.Valid = now, true
	}
}

func TestWebhookRepo_UpdateDeliveryState(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	group, _ := linkFixture(t, db, "wh.state@example.com")
	repo := repository.NewWebhookRepository(db)
	now := time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC)

	t.Run("keeps a concurrent name/url change", func(t *testing.T) {
		w := createWebhook(t, repo, group, "edit")
		edited := *w
		edited.Name, edited.URL = "renamed", "https://hooks.example.com/moved"
		require.NoError(t, repo.Update(ctx, &edited)) // lands while a delivery is in flight

		got, err := repo.UpdateDeliveryState(ctx, w.ID, failDelivery(now))
		require.NoError(t, err)
		assert.Equal(t, "https://hooks.example.com/moved", got.URL)
		stored, err := repo.GetByID(ctx, w.ID)
		require.NoError(t, err)
		assert.Equal(t, "renamed", stored.Name)
		assert.Equal(t, "https://hooks.example.com/moved", stored.URL)
		assert.Equal(t, 1, stored.ConsecutiveFailures)
		assert.True(t, stored.FailingSince.Time.Equal(now))
		assert.True(t, stored.LastDeliveryAt.Time.Equal(now))
	})

	t.Run("never reactivates a deactivated webhook", func(t *testing.T) {
		w := createWebhook(t, repo, group, "off")
		w.IsActive, w.Status = false, model.WebhookStatusDeactivated
		require.NoError(t, repo.Update(ctx, w))

		got, err := repo.UpdateDeliveryState(ctx, w.ID, func(x *model.Webhook) {
			x.IsActive, x.Status, x.ConsecutiveFailures = true, model.WebhookStatusActive, 0
			x.LastDeliveryAt.Time, x.LastDeliveryAt.Valid = now, true
		})
		require.NoError(t, err)
		assert.False(t, got.IsActive)
		stored, err := repo.GetByID(ctx, w.ID)
		require.NoError(t, err)
		assert.False(t, stored.IsActive)
		assert.Equal(t, model.WebhookStatusDeactivated, stored.Status)
		assert.True(t, stored.LastDeliveryAt.Valid, "the attempt is still recorded")
	})

	t.Run("parallel failures all count", func(t *testing.T) {
		w := createWebhook(t, repo, group, "par")
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := repo.UpdateDeliveryState(ctx, w.ID, failDelivery(now))
				assert.NoError(t, err)
			}()
		}
		wg.Wait()
		stored, err := repo.GetByID(ctx, w.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, stored.ConsecutiveFailures)
		assert.Equal(t, model.WebhookStatusAlert, stored.Status)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := repo.UpdateDeliveryState(ctx, 999999999, failDelivery(now))
		assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
	})
}

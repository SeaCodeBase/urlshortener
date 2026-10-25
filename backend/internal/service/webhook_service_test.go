package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	svcmocks "github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type webhookSvcFixture struct {
	svc      *service.WebhookServiceImpl
	hooks    *mocks.MockWebhookRepository
	activity *svcmocks.MockActivityService
	resolver *fakeIPResolver
	now      time.Time
}

func newWebhookSvc(t *testing.T, client *http.Client) *webhookSvcFixture {
	ctrl := gomock.NewController(t)
	f := &webhookSvcFixture{
		hooks:    mocks.NewMockWebhookRepository(ctrl),
		activity: svcmocks.NewMockActivityService(ctrl),
		resolver: &fakeIPResolver{ips: map[string][]string{"hooks.example.test": {"93.184.216.34"}}},
		now:      time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
	}
	f.svc = service.NewWebhookService(service.WebhookServiceDeps{
		Webhooks: f.hooks,
		Activity: f.activity,
		Client:   client,
		Resolver: f.resolver,
	})
	f.svc.SetClock(func() time.Time { return f.now })
	return f
}

func sampleWebhook(id, groupID uint64, url string) *model.Webhook {
	return &model.Webhook{
		ID: id, UserID: 1, GroupID: groupID, Name: "hook", URL: url, Event: "engagement",
		Secret: "abc123secret", IsActive: true, Status: model.WebhookStatusActive,
	}
}

func TestWebhookService_Create_GeneratesSecret(t *testing.T) {
	f := newWebhookSvc(t, nil)
	ctx := context.Background()

	t.Run("event must be engagement", func(t *testing.T) {
		_, err := f.svc.Create(ctx, 1, 7, service.WebhookInput{Name: "n", URL: "https://hooks.example.test/x", Event: "click"})
		assertValidation(t, err)
	})
	t.Run("name required", func(t *testing.T) {
		_, err := f.svc.Create(ctx, 1, 7, service.WebhookInput{Name: "  ", URL: "https://hooks.example.test/x", Event: "engagement"})
		assertValidation(t, err)
	})
	t.Run("private url rejected", func(t *testing.T) {
		_, err := f.svc.Create(ctx, 1, 7, service.WebhookInput{Name: "n", URL: "http://10.0.0.5/h", Event: "engagement"})
		assertValidation(t, err)
	})
	t.Run("ok", func(t *testing.T) {
		var stored *model.Webhook
		f.hooks.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, w *model.Webhook) error {
			stored = w
			w.ID = 5
			w.Status = model.WebhookStatusActive
			w.IsActive = true
			return nil
		})
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookCreate, service.EntityWebhook, uint64(5),
			map[string]any{"name": "My hook", "url_host": "https://hooks.example.test"})
		w, err := f.svc.Create(ctx, 1, 7, service.WebhookInput{Name: " My hook ", URL: "https://hooks.example.test/x", Event: "engagement"})
		require.NoError(t, err)
		require.NotNil(t, stored)
		assert.Equal(t, uint64(5), w.ID)
		assert.Equal(t, uint64(1), stored.UserID)
		assert.Equal(t, uint64(7), stored.GroupID)
		assert.Equal(t, "My hook", stored.Name)
		assert.Equal(t, "engagement", stored.Event)
		assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), stored.Secret)
	})
}

func TestWebhookService_WrongGroup(t *testing.T) {
	f := newWebhookSvc(t, nil)
	ctx := context.Background()
	f.hooks.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(sampleWebhook(5, 8, "https://hooks.example.test/x"), nil).AnyTimes()

	_, err := f.svc.Get(ctx, 7, 5)
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
	_, err = f.svc.Update(ctx, 1, 7, 5, service.WebhookInput{Name: "n", URL: "https://hooks.example.test/x", Event: "engagement"})
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
	assert.ErrorIs(t, f.svc.Delete(ctx, 1, 7, 5), repository.ErrWebhookNotFound)
	_, err = f.svc.Deactivate(ctx, 1, 7, 5)
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
	_, err = f.svc.Activate(ctx, 1, 7, 5)
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
	_, err = f.svc.Verify(ctx, 1, 7, 5)
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
	_, err = f.svc.Deliveries(ctx, 7, 5)
	assert.ErrorIs(t, err, repository.ErrWebhookNotFound)
}

func TestWebhookService_DeactivateActivate(t *testing.T) {
	f := newWebhookSvc(t, nil)
	ctx := context.Background()
	w := sampleWebhook(5, 7, "https://hooks.example.test/x")
	f.hooks.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(w, nil).AnyTimes()
	f.hooks.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookDeactivate, service.EntityWebhook, uint64(5), gomock.Any())
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookUpdate, service.EntityWebhook, uint64(5), map[string]any{"activated": true})

	got, err := f.svc.Deactivate(ctx, 1, 7, 5)
	require.NoError(t, err)
	assert.False(t, got.IsActive)
	assert.Equal(t, model.WebhookStatusDeactivated, got.Status)

	got.ConsecutiveFailures = 4
	got.FailingSince.Valid = true
	got, err = f.svc.Activate(ctx, 1, 7, 5)
	require.NoError(t, err)
	assert.True(t, got.IsActive)
	assert.Equal(t, model.WebhookStatusActive, got.Status)
	assert.Equal(t, 0, got.ConsecutiveFailures)
	assert.False(t, got.FailingSince.Valid)
}

func TestWebhookService_Verify_RecordsDelivery(t *testing.T) {
	ctx := context.Background()
	run := func(t *testing.T, status int) (*model.WebhookDelivery, *model.Webhook, *http.Request, []byte) {
		var gotReq *http.Request
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			gotReq = r
			gotBody, _ = io.ReadAll(r.Body)
			rw.WriteHeader(status)
		}))
		t.Cleanup(srv.Close)

		f := newWebhookSvc(t, &http.Client{Timeout: 2 * time.Second})
		w := sampleWebhook(5, 7, srv.URL+"/hook")
		var updated *model.Webhook
		f.hooks.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(w, nil)
		f.hooks.EXPECT().AddDelivery(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, d *model.WebhookDelivery) error {
			d.ID = 99
			return nil
		})
		f.hooks.EXPECT().UpdateDeliveryState(gomock.Any(), uint64(5), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ uint64, apply func(*model.Webhook)) (*model.Webhook, error) {
				cur := *w // the row as locked by the repository
				apply(&cur)
				updated = &cur
				return updated, nil
			})
		f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookVerify, service.EntityWebhook, uint64(5),
			map[string]any{"status_code": status})

		d, err := f.svc.Verify(ctx, 1, 7, 5)
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.NotNil(t, gotReq)
		return d, updated, gotReq, gotBody
	}

	t.Run("204 ok", func(t *testing.T) {
		d, w, req, body := run(t, http.StatusNoContent)
		assert.Equal(t, uint64(99), d.ID)
		assert.Equal(t, uint64(5), d.WebhookID)
		assert.Equal(t, http.StatusNoContent, d.StatusCode)
		assert.Equal(t, 1, d.Attempt)
		assert.Empty(t, d.Error)
		assert.Equal(t, "ping-"+strconv.FormatInt(time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC).Unix(), 10), d.EventID)
		assert.Equal(t, model.WebhookStatusActive, w.Status)
		assert.True(t, w.IsActive)
		assert.Equal(t, 0, w.ConsecutiveFailures)
		assert.True(t, w.LastDeliveryAt.Valid)

		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "/hook", req.URL.Path)
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, service.SignBody("abc123secret", body), req.Header.Get("X-Signature"))
		assert.Equal(t, "5", req.Header.Get("X-Webhook-Id"))
		assert.Equal(t, d.EventID, req.Header.Get("X-Event-Id"))

		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "ping", payload["event_type"])
		assert.Equal(t, float64(5), payload["webhook_id"])
		assert.Equal(t, "2026-10-25T12:00:00Z", payload["datetime"])
	})

	t.Run("500 fails", func(t *testing.T) {
		d, w, _, _ := run(t, http.StatusInternalServerError)
		assert.Equal(t, http.StatusInternalServerError, d.StatusCode)
		assert.Equal(t, 1, w.ConsecutiveFailures)
		assert.True(t, w.FailingSince.Valid)
		assert.Equal(t, model.WebhookStatusActive, w.Status)
	})
}

func TestWebhookService_Verify_Timeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	f := newWebhookSvc(t, &http.Client{Timeout: 50 * time.Millisecond})
	w := sampleWebhook(5, 7, srv.URL)
	var updated *model.Webhook
	f.hooks.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(w, nil)
	f.hooks.EXPECT().AddDelivery(gomock.Any(), gomock.Any()).Return(nil)
	f.hooks.EXPECT().UpdateDeliveryState(gomock.Any(), uint64(5), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uint64, apply func(*model.Webhook)) (*model.Webhook, error) {
			cur := *w // the row as locked by the repository
			apply(&cur)
			updated = &cur
			return updated, nil
		})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookVerify, service.EntityWebhook, uint64(5),
		map[string]any{"status_code": 0})

	d, err := f.svc.Verify(context.Background(), 1, 7, 5)
	require.NoError(t, err)
	assert.Equal(t, 0, d.StatusCode)
	assert.NotEmpty(t, d.Error)
	require.NotNil(t, updated)
	assert.Equal(t, 1, updated.ConsecutiveFailures)
}

func TestWebhookService_Verify_DeactivatedStaysDeactivated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) { rw.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	f := newWebhookSvc(t, &http.Client{Timeout: 2 * time.Second})
	w := sampleWebhook(5, 7, srv.URL)
	w.IsActive = false
	w.Status = model.WebhookStatusDeactivated
	var updated *model.Webhook
	f.hooks.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(w, nil)
	f.hooks.EXPECT().AddDelivery(gomock.Any(), gomock.Any()).Return(nil)
	f.hooks.EXPECT().UpdateDeliveryState(gomock.Any(), uint64(5), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ uint64, apply func(*model.Webhook)) (*model.Webhook, error) {
			cur := *w // the row as locked by the repository
			apply(&cur)
			updated = &cur
			return updated, nil
		})
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookVerify, service.EntityWebhook, uint64(5), gomock.Any())

	d, err := f.svc.Verify(context.Background(), 1, 7, 5)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, d.StatusCode)
	require.NotNil(t, updated)
	assert.False(t, updated.IsActive)
	assert.Equal(t, model.WebhookStatusDeactivated, updated.Status)
	assert.True(t, updated.LastDeliveryAt.Valid)
}

func TestWebhookService_UpdateDeleteDeliveries(t *testing.T) {
	f := newWebhookSvc(t, nil)
	ctx := context.Background()
	w := sampleWebhook(5, 7, "https://hooks.example.test/x")
	f.hooks.EXPECT().GetByID(gomock.Any(), uint64(5)).Return(w, nil).AnyTimes()

	_, err := f.svc.Update(ctx, 1, 7, 5, service.WebhookInput{Name: "n", URL: "http://localhost/x", Event: "engagement"})
	assertValidation(t, err)

	f.hooks.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookUpdate, service.EntityWebhook, uint64(5), gomock.Any())
	got, err := f.svc.Update(ctx, 1, 7, 5, service.WebhookInput{Name: "Renamed", URL: "https://hooks.example.test/y"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", got.Name)
	assert.Equal(t, "https://hooks.example.test/y", got.URL)

	f.hooks.EXPECT().ListDeliveries(gomock.Any(), uint64(5), 20).Return([]model.WebhookDelivery{{ID: 1}}, nil)
	ds, err := f.svc.Deliveries(ctx, 7, 5)
	require.NoError(t, err)
	assert.Len(t, ds, 1)

	f.hooks.EXPECT().Delete(gomock.Any(), uint64(5)).Return(nil)
	f.activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionWebhookDelete, service.EntityWebhook, uint64(5), gomock.Any())
	require.NoError(t, f.svc.Delete(ctx, 1, 7, 5))
}

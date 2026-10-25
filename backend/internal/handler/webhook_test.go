package handler

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var webhookTS = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func newWebhookRouter(t *testing.T) (*gin.Engine, *mocks.MockWebhookService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockWebhookService(gomock.NewController(t))
	r := gin.New()
	RegisterWebhookRoutes(r.Group("/api", withGroup), NewWebhookHandler(svc))
	return r, svc
}

func sampleHook(id uint64) *model.Webhook {
	return &model.Webhook{ID: id, UserID: 1, GroupID: 7, Name: "Zapier", URL: "https://hooks.example.com/x",
		Event: "engagement", Secret: "abc123", IsActive: true, Status: model.WebhookStatusActive,
		CreatedAt: webhookTS, UpdatedAt: webhookTS}
}

func TestWebhookHandler_Create(t *testing.T) {
	r, svc := newWebhookRouter(t)
	in := service.WebhookInput{Name: "Zapier", URL: "https://hooks.example.com/x", Event: "engagement"}
	svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), in).Return(sampleHook(3), nil)
	w := doJSON(r, http.MethodPost, "/api/webhooks", `{"name":"Zapier","url":"https://hooks.example.com/x","event":"engagement"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"secret":"abc123"`)
	assert.Contains(t, w.Body.String(), `"status":"active"`)

	w = doJSON(r, http.MethodPost, "/api/webhooks", `not json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestWebhookHandler_CreatePrivateURL400(t *testing.T) {
	r, svc := newWebhookRouter(t)
	svc.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), gomock.Any()).
		Return(nil, &service.ValidationError{Msg: "URL must be a public http(s) address"})
	w := doJSON(r, http.MethodPost, "/api/webhooks", `{"name":"x","url":"http://10.0.0.1/"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"URL must be a public http(s) address"}`, w.Body.String())
}

func TestWebhookHandler_List(t *testing.T) {
	r, svc := newWebhookRouter(t)
	svc.EXPECT().List(gomock.Any(), uint64(7)).Return([]model.Webhook{*sampleHook(3)}, nil)
	w := doJSON(r, http.MethodGet, "/api/webhooks", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"webhooks":[{`)
	assert.Contains(t, w.Body.String(), `"secret":"abc123"`)

	svc.EXPECT().List(gomock.Any(), uint64(7)).Return(nil, nil)
	w = doJSON(r, http.MethodGet, "/api/webhooks", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"webhooks":[]}`, w.Body.String())

	svc.EXPECT().List(gomock.Any(), uint64(7)).Return(nil, errors.New("db down"))
	w = doJSON(r, http.MethodGet, "/api/webhooks", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"failed to list webhooks"}`, w.Body.String())
}

func TestWebhookHandler_GetUpdateDelete(t *testing.T) {
	r, svc := newWebhookRouter(t)
	svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(3)).Return(sampleHook(3), nil)
	w := doJSON(r, http.MethodGet, "/api/webhooks/3", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"id":3`)

	svc.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(3), service.WebhookInput{Name: "New", URL: "https://h.example.com"}).
		Return(sampleHook(3), nil)
	w = doJSON(r, http.MethodPut, "/api/webhooks/3", `{"name":"New","url":"https://h.example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(3)).Return(nil)
	w = doJSON(r, http.MethodDelete, "/api/webhooks/3", "")
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = doJSON(r, http.MethodGet, "/api/webhooks/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"invalid webhook ID"}`, w.Body.String())
}

func TestWebhookHandler_WrongGroup404(t *testing.T) {
	r, svc := newWebhookRouter(t)
	svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(9)).Return(nil, repository.ErrWebhookNotFound)
	w := doJSON(r, http.MethodGet, "/api/webhooks/9", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"webhook not found"}`, w.Body.String())

	svc.EXPECT().Verify(gomock.Any(), uint64(1), uint64(7), uint64(9)).Return(nil, repository.ErrWebhookNotFound)
	w = doJSON(r, http.MethodPost, "/api/webhooks/9/verify", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	svc.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(9)).Return(repository.ErrWebhookNotFound)
	w = doJSON(r, http.MethodDelete, "/api/webhooks/9", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	svc.EXPECT().Deliveries(gomock.Any(), uint64(7), uint64(9)).Return(nil, repository.ErrWebhookNotFound)
	w = doJSON(r, http.MethodGet, "/api/webhooks/9/deliveries", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_Verify(t *testing.T) {
	r, svc := newWebhookRouter(t)
	d := &model.WebhookDelivery{ID: 11, WebhookID: 3, EventID: "ping-1", StatusCode: 200, Attempt: 1, DeliveredAt: webhookTS}
	gomock.InOrder(
		svc.EXPECT().Verify(gomock.Any(), uint64(1), uint64(7), uint64(3)).Return(d, nil),
		svc.EXPECT().Get(gomock.Any(), uint64(7), uint64(3)).Return(sampleHook(3), nil),
	)
	w := doJSON(r, http.MethodPost, "/api/webhooks/3/verify", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"delivery":{"id":11`)
	assert.Contains(t, w.Body.String(), `"status_code":200`)
	assert.Contains(t, w.Body.String(), `"webhook":{"id":3`)
}

func TestWebhookHandler_DeactivateActivate(t *testing.T) {
	r, svc := newWebhookRouter(t)
	off := sampleHook(3)
	off.IsActive, off.Status = false, model.WebhookStatusDeactivated
	svc.EXPECT().Deactivate(gomock.Any(), uint64(1), uint64(7), uint64(3)).Return(off, nil)
	w := doJSON(r, http.MethodPost, "/api/webhooks/3/deactivate", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"deactivated"`)
	assert.Contains(t, w.Body.String(), `"is_active":false`)

	svc.EXPECT().Activate(gomock.Any(), uint64(1), uint64(7), uint64(3)).Return(sampleHook(3), nil)
	w = doJSON(r, http.MethodPost, "/api/webhooks/3/activate", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"status":"active"`)
}

func TestWebhookHandler_Deliveries(t *testing.T) {
	r, svc := newWebhookRouter(t)
	svc.EXPECT().Deliveries(gomock.Any(), uint64(7), uint64(3)).Return([]model.WebhookDelivery{
		{ID: 2, WebhookID: 3, EventID: "e2", StatusCode: 500, Attempt: 2, DeliveredAt: webhookTS, Error: ""},
	}, nil)
	w := doJSON(r, http.MethodGet, "/api/webhooks/3/deliveries", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"deliveries":[{"id":2`)

	svc.EXPECT().Deliveries(gomock.Any(), uint64(7), uint64(3)).Return(nil, nil)
	w = doJSON(r, http.MethodGet, "/api/webhooks/3/deliveries", "")
	assert.JSONEq(t, `{"deliveries":[]}`, w.Body.String())
}

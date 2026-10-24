package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newTokenRouter(t *testing.T) (*gin.Engine, *mocks.MockTokenService) {
	r, svc, activity := newTokenRouterWithActivity(t)
	activity.EXPECT().Log(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	return r, svc
}

func newTokenRouterWithActivity(t *testing.T) (*gin.Engine, *mocks.MockTokenService, *mocks.MockActivityService) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockTokenService(ctrl)
	activity := mocks.NewMockActivityService(ctrl)
	h := NewTokenHandler(svc, activity)
	r := gin.New()
	g := r.Group("/api/api-tokens", withGroup)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.DELETE("/:id", h.Revoke)
	return r, svc, activity
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var tokCreated = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func TestTokenHandler_Create_ReturnsRawOnce(t *testing.T) {
	r, svc := newTokenRouter(t)
	raw := "us_" + strings.Repeat("a", 40)
	tok := &model.APIToken{ID: 4, UserID: 1, Name: "cli", TokenHash: "secrethash", TokenPrefix: "aaaaaaaa", CreatedAt: tokCreated}
	svc.EXPECT().Create(gomock.Any(), uint64(1), "cli", "pw").Return(raw, tok, nil)
	svc.EXPECT().List(gomock.Any(), uint64(1)).Return([]model.APIToken{*tok}, nil)

	w := doJSON(r, http.MethodPost, "/api/api-tokens", `{"name":"cli","password":"pw"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.JSONEq(t, `{"token":"`+raw+`","id":4,"name":"cli","prefix":"aaaaaaaa","created_at":"2026-10-24T12:00:00Z"}`, w.Body.String())

	// The raw token (and its hash) never appear in the list.
	w = doJSON(r, http.MethodGet, "/api/api-tokens", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), raw)
	assert.NotContains(t, w.Body.String(), "secrethash")
	assert.JSONEq(t, `{"tokens":[{"id":4,"name":"cli","prefix":"aaaaaaaa","last_used_at":null,"created_at":"2026-10-24T12:00:00Z"}]}`, w.Body.String())
}

func TestTokenHandler_Create_WrongPassword401(t *testing.T) {
	r, svc := newTokenRouter(t)
	svc.EXPECT().Create(gomock.Any(), uint64(1), "cli", "bad").Return("", nil, service.ErrInvalidCredentials)

	w := doJSON(r, http.MethodPost, "/api/api-tokens", `{"name":"cli","password":"bad"}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error":"invalid credentials"}`, w.Body.String())
}

func TestTokenHandler_Create_MissingFields400(t *testing.T) {
	r, _ := newTokenRouter(t)
	w := doJSON(r, http.MethodPost, "/api/api-tokens", `{"name":"cli"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTokenHandler_Create_BlankName400(t *testing.T) {
	r, svc := newTokenRouter(t)
	svc.EXPECT().Create(gomock.Any(), uint64(1), " ", "pw").Return("", nil, service.ErrInvalidTokenName)
	w := doJSON(r, http.MethodPost, "/api/api-tokens", `{"name":" ","password":"pw"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTokenHandler_List_Empty(t *testing.T) {
	r, svc := newTokenRouter(t)
	svc.EXPECT().List(gomock.Any(), uint64(1)).Return(nil, nil)

	w := doJSON(r, http.MethodGet, "/api/api-tokens", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, []any{}, body["tokens"])
}

func TestTokenHandler_Revoke204(t *testing.T) {
	r, svc := newTokenRouter(t)
	svc.EXPECT().Revoke(gomock.Any(), uint64(1), uint64(4)).Return(nil)

	w := doJSON(r, http.MethodDelete, "/api/api-tokens/4", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTokenHandler_RevokeOtherUsers404(t *testing.T) {
	r, svc := newTokenRouter(t)
	svc.EXPECT().Revoke(gomock.Any(), uint64(1), uint64(8)).Return(service.ErrAPITokenNotFound)

	w := doJSON(r, http.MethodDelete, "/api/api-tokens/8", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTokenHandler_RevokeInvalidID400(t *testing.T) {
	r, _ := newTokenRouter(t)
	w := doJSON(r, http.MethodDelete, "/api/api-tokens/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTokenHandler_LogsActivity(t *testing.T) {
	r, svc, activity := newTokenRouterWithActivity(t)
	tok := &model.APIToken{ID: 4, UserID: 1, Name: "cli", TokenPrefix: "aaaaaaaa", CreatedAt: tokCreated}
	svc.EXPECT().Create(gomock.Any(), uint64(1), "cli", "pw").Return("us_x", tok, nil)
	activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionTokenCreate, service.EntityToken, uint64(4),
		map[string]any{"name": "cli", "prefix": "aaaaaaaa"})
	w := doJSON(r, http.MethodPost, "/api/api-tokens", `{"name":"cli","password":"pw"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	svc.EXPECT().Revoke(gomock.Any(), uint64(1), uint64(4)).Return(nil)
	activity.EXPECT().Log(gomock.Any(), uint64(1), uint64(7), service.ActionTokenRevoke, service.EntityToken, uint64(4), gomock.Nil())
	w = doJSON(r, http.MethodDelete, "/api/api-tokens/4", "")
	require.Equal(t, http.StatusNoContent, w.Code)
}

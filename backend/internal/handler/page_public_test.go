package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newPublicPageRouter(t *testing.T) (*gin.Engine, *mocks.MockPagePublicService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockPagePublicService(gomock.NewController(t))
	h := NewPublicPageHandler(svc)
	r := gin.New()
	r.GET("/api/public/pages/:slug", h.Get)
	r.POST("/api/public/pages/:slug/view", h.View)
	return r, svc
}

func TestPublicPageHandler_DraftIs404(t *testing.T) {
	r, svc := newPublicPageRouter(t)
	svc.EXPECT().GetPublished(gomock.Any(), "draft-page").Return(nil, repository.ErrPageNotFound)
	w := doJSON(r, http.MethodGet, "/api/public/pages/draft-page", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"page not found"}`, w.Body.String())

	svc.EXPECT().RecordView(gomock.Any(), "draft-page", gomock.Any()).Return(repository.ErrPageNotFound)
	w = doJSON(r, http.MethodPost, "/api/public/pages/draft-page/view", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"page not found"}`, w.Body.String())

	svc.EXPECT().GetPublished(gomock.Any(), "jose").Return(nil, errors.New("db down"))
	w = doJSON(r, http.MethodGet, "/api/public/pages/jose", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"failed to get page"}`, w.Body.String())

	svc.EXPECT().GetPublished(gomock.Any(), "jose").Return(map[string]any{
		"slug": "jose", "title": "Jose",
		"blocks": []any{map[string]any{"id": 5, "type": "link", "short_url": "http://localhost:8080/abc?r=page"}},
	}, nil)
	w = doJSON(r, http.MethodGet, "/api/public/pages/jose", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"slug":"jose","title":"Jose","blocks":[{"id":5,"type":"link","short_url":"http://localhost:8080/abc?r=page"}]}`, w.Body.String())
}

func TestPublicPageHandler_View(t *testing.T) {
	r, svc := newPublicPageRouter(t)
	svc.EXPECT().RecordView(gomock.Any(), "jose", service.ViewInput{
		IP: "203.0.113.9", UserAgent: "Mozilla/5.0 (iPhone)", Referrer: "https://instagram.com/",
	}).Return(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/public/pages/jose/view", strings.NewReader(`{"ignored":true}`))
	req.RemoteAddr = "203.0.113.9:4242"
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone)")
	req.Header.Set("Referer", "https://instagram.com/")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())

	svc.EXPECT().RecordView(gomock.Any(), "jose", gomock.Any()).Return(errors.New("insert failed"))
	w = doJSON(r, http.MethodPost, "/api/public/pages/jose/view", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"failed to record view"}`, w.Body.String())
}

func TestPublicPageHandler_View_BodyReferrer(t *testing.T) {
	r, svc := newPublicPageRouter(t)
	post := func(body, contentType string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/public/pages/jose/view", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.9:4242"
		req.Header.Set("Referer", "https://app.test/m/jose")
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	want := func(ref string) {
		svc.EXPECT().RecordView(gomock.Any(), "jose", service.ViewInput{IP: "203.0.113.9", Referrer: ref}).Return(nil)
	}

	// The body's referrer wins over the header, whatever the content type
	// (sendBeacon posts text/plain); other fields are ignored.
	want("https://instagram.com/")
	assert.Equal(t, http.StatusNoContent, post(`{"referrer":"https://instagram.com/","x":1}`, "application/json"))
	want("https://t.co/abc")
	assert.Equal(t, http.StatusNoContent, post(`{"referrer":"https://t.co/abc"}`, "text/plain;charset=UTF-8"))

	// Empty, missing, non-string, malformed, non-JSON or oversized body:
	// fall back to the Referer header, never an error.
	for _, body := range []string{``, `{}`, `{"referrer":""}`, `{"referrer":"  "}`, `{"referrer":5}`, `{"referrer":`,
		`not json`, `[1,2]`, `{"referrer":"https://big.test/` + strings.Repeat("a", 5000) + `"}`} {
		want("https://app.test/m/jose")
		assert.Equal(t, http.StatusNoContent, post(body, "application/json"), body)
	}
}

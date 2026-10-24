package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAnalyticsHandler_Today(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockStatsService(gomock.NewController(t))
	h := NewAnalyticsHandler(svc)
	h.now = func() time.Time { return time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC) }
	r := gin.New()
	r.GET("/api/analytics/today", withGroup, h.Today)

	svc.EXPECT().GroupEngagements(gomock.Any(), uint64(7), d(2026, 10, 24), d(2026, 10, 24)).Return(int64(4), nil)
	w := doJSON(r, http.MethodGet, "/api/analytics/today", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"engagements":4,"range":"today"}`, w.Body.String())

	svc.EXPECT().GroupEngagements(gomock.Any(), uint64(7), d(2026, 10, 18), d(2026, 10, 24)).Return(int64(10), nil)
	w = doJSON(r, http.MethodGet, "/api/analytics/today?range=7d", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"engagements":10,"range":"7d"}`, w.Body.String())

	svc.EXPECT().GroupEngagements(gomock.Any(), uint64(7), d(2026, 9, 25), d(2026, 10, 24)).Return(int64(30), nil)
	w = doJSON(r, http.MethodGet, "/api/analytics/today?range=30d", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"engagements":30,"range":"30d"}`, w.Body.String())

	w = doJSON(r, http.MethodGet, "/api/analytics/today?range=1y", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

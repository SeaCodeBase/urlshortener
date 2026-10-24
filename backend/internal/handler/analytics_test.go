package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAnalyticsHandler_Today(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockStatsService(gomock.NewController(t))
	h := NewAnalyticsHandler(svc, nil)
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

func TestAnalyticsHandler_Overview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	an := mocks.NewMockAnalyticsService(ctrl)
	h := NewAnalyticsHandler(mocks.NewMockStatsService(ctrl), an)
	r := gin.New()
	r.GET("/api/analytics/overview", withGroup, h.Overview)

	pct := 12.5
	an.EXPECT().Overview(gomock.Any(), uint64(7)).Return(&service.Overview{
		ThisWeek:  service.OverviewWeek{From: "2026-10-18", To: "2026-10-24", Total: 45, Series: []service.SeriesPoint{{Date: "2026-10-18", Clicks: 45}}},
		LastWeek:  service.OverviewWeek{From: "2026-10-11", To: "2026-10-17", Total: 40, Series: []service.SeriesPoint{}},
		ChangePct: &pct,
		TopPerforming: []service.AnalyticsItem{{Type: "link", ID: 1, LinkID: 1, Title: "One", ShortURL: "http://localhost:8080/one",
			OriginalURL: "https://one.example", Clicks: 30, Percentage: 30}},
	}, nil)
	w := doJSON(r, http.MethodGet, "/api/analytics/overview", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"this_week":{"from":"2026-10-18","to":"2026-10-24","total":45,"series":[{"date":"2026-10-18","clicks":45}]},
		"last_week":{"from":"2026-10-11","to":"2026-10-17","total":40,"series":[]},"change_pct":12.5,
		"top_performing":[{"type":"link","id":1,"link_id":1,"title":"One","short_url":"http://localhost:8080/one",
		"original_url":"https://one.example","clicks":30,"percentage":30,"prev_clicks":0}]}`, w.Body.String())

	an.EXPECT().Overview(gomock.Any(), uint64(7)).Return(nil, assert.AnError)
	w = doJSON(r, http.MethodGet, "/api/analytics/overview", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

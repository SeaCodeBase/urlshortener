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

var statsNow = time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func newStatsRouter(t *testing.T) (*gin.Engine, *mocks.MockStatsService) {
	gin.SetMode(gin.TestMode)
	svc := mocks.NewMockStatsService(gomock.NewController(t))
	h := NewStatsHandler(svc)
	h.now = func() time.Time { return statsNow }
	r := gin.New()
	r.GET("/api/links/:id/stats", withGroup, h.GetLinkStats)
	return r, svc
}

func TestStatsHandler_GetLinkStats_Params(t *testing.T) {
	r, svc := newStatsRouter(t)
	pct := 12.5
	svc.EXPECT().GetLinkStats(gomock.Any(), uint64(1), uint64(5), service.StatsQuery{
		From: d(2026, 10, 1), To: d(2026, 10, 2), Unit: "hour", Compare: true, Source: "qr",
	}).Return(&service.LinkStatsV2{TotalAllTime: 9, TotalRange: 3, ChangePct: &pct,
		Series: []service.SeriesPoint{}, PrevSeries: []service.SeriesPoint{}, Countries: []service.CountryStat{},
		Cities: []service.CityStat{}, Referrers: []service.ReferrerStat{}, Devices: []service.KeyStat{},
		Browsers: []service.KeyStat{}, OS: []service.KeyStat{{Key: "iOS", Clicks: 3, Percentage: 100}}}, nil)

	w := doJSON(r, http.MethodGet, "/api/links/5/stats?from=2026-10-01&to=2026-10-02&unit=hour&compare=previous&source=qr", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"from":"","to":"","unit":"","total_all_time":9,"total_range":3,"prev_total_range":0,"change_pct":12.5,
		"series":[],"prev_series":[],"countries":[],"cities":[],"referrers":[],"devices":[],"browsers":[],
		"os":[{"key":"iOS","clicks":3,"percentage":100,"prev_clicks":0}]}`, w.Body.String())
}

func TestStatsHandler_GetLinkStats_Defaults(t *testing.T) {
	r, svc := newStatsRouter(t)
	svc.EXPECT().GetLinkStats(gomock.Any(), uint64(1), uint64(5), service.StatsQuery{
		From: d(2026, 9, 25), To: d(2026, 10, 24), Unit: "day", Compare: false, Source: "",
	}).Return(&service.LinkStatsV2{}, nil)
	w := doJSON(r, http.MethodGet, "/api/links/5/stats", "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	svc.EXPECT().GetLinkStats(gomock.Any(), uint64(1), uint64(5), service.StatsQuery{
		From: d(2026, 10, 1), To: d(2026, 10, 30), Unit: "day", Compare: false, Source: "",
	}).Return(&service.LinkStatsV2{}, nil)
	w = doJSON(r, http.MethodGet, "/api/links/5/stats?to=2026-10-30&compare=none&source=all", "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestStatsHandler_GetLinkStats_BadParams400(t *testing.T) {
	r, _ := newStatsRouter(t)
	for _, q := range []string{
		"from=2026-13-01", "to=yesterday", "unit=week", "compare=both", "source=email",
		"from=2026-10-05&to=2026-10-01", "from=2025-01-01&to=2026-10-01",
	} {
		w := doJSON(r, http.MethodGet, "/api/links/5/stats?"+q, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}
	w := doJSON(r, http.MethodGet, "/api/links/5/stats?from=2026-10-01&to=2026-10-03&unit=hour", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"hourly data is limited to 2 days"}`, w.Body.String())

	w = doJSON(r, http.MethodGet, "/api/links/abc/stats", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStatsHandler_GetLinkStats_NotOwner404(t *testing.T) {
	r, svc := newStatsRouter(t)
	svc.EXPECT().GetLinkStats(gomock.Any(), uint64(1), uint64(5), gomock.Any()).Return(nil, service.ErrNotLinkOwner)
	w := doJSON(r, http.MethodGet, "/api/links/5/stats", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.JSONEq(t, `{"error":"link not found"}`, w.Body.String())
}

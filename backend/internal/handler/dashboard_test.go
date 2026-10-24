package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/service/mocks"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var dashNow = time.Date(2026, 10, 24, 15, 0, 0, 0, time.UTC)

func newDashboardRouter(t *testing.T) (*gin.Engine, *mocks.MockDashboardService, *mocks.MockAnalyticsService) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	dash := mocks.NewMockDashboardService(ctrl)
	an := mocks.NewMockAnalyticsService(ctrl)
	h := NewDashboardHandler(dash, an)
	h.now = func() time.Time { return dashNow }
	r := gin.New()
	g := r.Group("/api/analytics/dashboards", withGroup)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.POST("/:id/duplicate", h.Duplicate)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/data", h.Data)
	g.GET("/:id/export", h.Export)
	return r, dash, an
}

func dashView(id uint64, name string) *service.DashboardView {
	ts := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	return &service.DashboardView{ID: id, GroupID: 7, Name: name, DateRange: "last_7_days",
		Filter: service.DashboardFilter{Type: "all"}, Layout: service.DefaultLayout,
		CreatedBy: service.DashboardCreator{ID: 1, Name: "Ann"}, CreatedAt: ts, UpdatedAt: ts}
}

func TestDashboardHandler_ListCreate(t *testing.T) {
	r, dash, _ := newDashboardRouter(t)

	def := dashView(1, "Default")
	def.IsDefault = true
	dash.EXPECT().List(gomock.Any(), uint64(1), uint64(7), "name").Return([]service.DashboardView{*def}, nil)
	w := doJSON(r, http.MethodGet, "/api/analytics/dashboards?sort=name", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.JSONEq(t, `{"dashboards":[{"id":1,"group_id":7,"name":"Default","is_default":true,"date_range":"last_7_days",
		"filter":{"type":"all"},"layout":{"modules":[{"key":"top_day","viz":"scorecard"},{"key":"top_location","viz":"scorecard"},
		{"key":"over_time","viz":"line"},{"key":"devices","viz":"doughnut"},{"key":"referrers","viz":"bar"},
		{"key":"locations","viz":"table"},{"key":"items","viz":"table"}]},"created_by":{"id":1,"name":"Ann"},
		"created_at":"2026-10-24T12:00:00Z","updated_at":"2026-10-24T12:00:00Z"}],"total":1}`, w.Body.String())

	dash.EXPECT().List(gomock.Any(), uint64(1), uint64(7), "").Return([]service.DashboardView{}, nil)
	w = doJSON(r, http.MethodGet, "/api/analytics/dashboards", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"dashboards":[],"total":0}`, w.Body.String())

	dash.EXPECT().List(gomock.Any(), uint64(1), uint64(7), "size").Return(nil, &service.ValidationError{Msg: "bad sort"})
	w = doJSON(r, http.MethodGet, "/api/analytics/dashboards?sort=size", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	dash.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), "Q4").Return(dashView(2, "Q4"), nil)
	w = doJSON(r, http.MethodPost, "/api/analytics/dashboards", `{"name":"Q4"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"name":"Q4"`)

	dash.EXPECT().Create(gomock.Any(), uint64(1), uint64(7), "  ").Return(nil, &service.ValidationError{Msg: "name is required"})
	w = doJSON(r, http.MethodPost, "/api/analytics/dashboards", `{"name":"  "}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"name is required"}`, w.Body.String())

	w = doJSON(r, http.MethodPost, "/api/analytics/dashboards", `not json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	dash.EXPECT().Duplicate(gomock.Any(), uint64(1), uint64(7), uint64(2)).Return(dashView(3, "Q4 (copy)"), nil)
	w = doJSON(r, http.MethodPost, "/api/analytics/dashboards/2/duplicate", "")
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestDashboardHandler_Get_OtherGroup404(t *testing.T) {
	r, dash, _ := newDashboardRouter(t)
	dash.EXPECT().Get(gomock.Any(), uint64(7), uint64(9)).Return(nil, repository.ErrDashboardNotFound).Times(3)
	for _, p := range []string{"/api/analytics/dashboards/9", "/api/analytics/dashboards/9/data", "/api/analytics/dashboards/9/export"} {
		w := doJSON(r, http.MethodGet, p, "")
		assert.Equal(t, http.StatusNotFound, w.Code, p)
		assert.JSONEq(t, `{"error":"dashboard not found"}`, w.Body.String())
	}
	dash.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(9)).Return(repository.ErrDashboardNotFound)
	w := doJSON(r, http.MethodDelete, "/api/analytics/dashboards/9", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = doJSON(r, http.MethodGet, "/api/analytics/dashboards/abc", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	dash.EXPECT().Get(gomock.Any(), uint64(7), uint64(2)).Return(dashView(2, "Q4"), nil)
	w = doJSON(r, http.MethodGet, "/api/analytics/dashboards/2", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":2`)
}

func TestDashboardHandler_Patch_Validation400(t *testing.T) {
	r, dash, _ := newDashboardRouter(t)

	// Empty body (and unknown fields only) → nothing to update.
	for _, body := range []string{`{}`, `{"color":"red"}`} {
		w := doJSON(r, http.MethodPatch, "/api/analytics/dashboards/2", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.JSONEq(t, `{"error":"nothing to update"}`, w.Body.String())
	}
	w := doJSON(r, http.MethodPatch, "/api/analytics/dashboards/2", `{"name":`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	dash.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(2), gomock.Any()).
		Return(nil, &service.ValidationError{Msg: "filter may hold at most 200 ids"})
	w = doJSON(r, http.MethodPatch, "/api/analytics/dashboards/2", `{"filter":{"type":"links","ids":[1]}}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":"filter may hold at most 200 ids"}`, w.Body.String())

	dash.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(9), gomock.Any()).Return(nil, repository.ErrDashboardNotFound)
	w = doJSON(r, http.MethodPatch, "/api/analytics/dashboards/9", `{"name":"x"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Every field is passed through; an unknown field is ignored.
	name, rng := "Renamed", "last_30_days"
	layout := service.DefaultLayout
	want := service.DashboardPatch{Name: &name, DateRange: &rng,
		Filter: &service.DashboardFilter{Type: "qr", IDs: []uint64{5}}, Layout: &layout}
	dash.EXPECT().Update(gomock.Any(), uint64(1), uint64(7), uint64(2), want).Return(dashView(2, "Renamed"), nil)
	w = doJSON(r, http.MethodPatch, "/api/analytics/dashboards/2", `{"name":"Renamed","date_range":"last_30_days","color":"red",
		"filter":{"type":"qr","ids":[5]},"layout":{"modules":[{"key":"top_day","viz":"scorecard"},{"key":"top_location","viz":"scorecard"},
		{"key":"over_time","viz":"line"},{"key":"devices","viz":"doughnut"},{"key":"referrers","viz":"bar"},
		{"key":"locations","viz":"table"},{"key":"items","viz":"table"}]}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"name":"Renamed"`)
}

func TestDashboardHandler_Delete_Default409(t *testing.T) {
	r, dash, _ := newDashboardRouter(t)
	dash.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(1)).Return(service.ErrDashboardDefault)
	w := doJSON(r, http.MethodDelete, "/api/analytics/dashboards/1", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.JSONEq(t, `{"error":"the default dashboard cannot be deleted"}`, w.Body.String())

	dash.EXPECT().Delete(gomock.Any(), uint64(1), uint64(7), uint64(2)).Return(nil)
	w = doJSON(r, http.MethodDelete, "/api/analytics/dashboards/2", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestDashboardHandler_Data_PassesStoredFilter(t *testing.T) {
	r, dash, an := newDashboardRouter(t)
	v := dashView(2, "Q4")
	v.Filter = service.DashboardFilter{Type: "links", IDs: []uint64{3}}
	dash.EXPECT().Get(gomock.Any(), uint64(7), uint64(2)).Return(v, nil)
	wantQ := service.StatsQuery{From: d(2026, 10, 18), To: d(2026, 10, 24), Unit: "day", Compare: true}
	an.EXPECT().DashboardData(gomock.Any(), uint64(7), v.Filter, wantQ).
		Return(&service.DashboardData{From: "2026-10-18", To: "2026-10-24", Unit: "day", Compare: true, Total: 120}, nil)

	w := doJSON(r, http.MethodGet, "/api/analytics/dashboards/2/data?from=2026-10-18&to=2026-10-24&compare=previous&source=qr", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total":120`)
	assert.Contains(t, w.Body.String(), `"from":"2026-10-18"`)

	// Bad queries are 400 before any service call.
	for _, q := range []string{"from=2026-13-01", "compare=yes", "from=2026-10-24&to=2026-10-18"} {
		w = doJSON(r, http.MethodGet, "/api/analytics/dashboards/2/data?"+q, "")
		assert.Equal(t, http.StatusBadRequest, w.Code, q)
	}
}

func TestDashboardHandler_Export_Headers(t *testing.T) {
	r, dash, an := newDashboardRouter(t)
	v := dashView(2, "  Q4 Launch: Spring/Summer!! ")
	dash.EXPECT().Get(gomock.Any(), uint64(7), uint64(2)).Return(v, nil)
	wantQ := service.StatsQuery{From: d(2026, 10, 18), To: d(2026, 10, 24), Unit: "day"}
	an.EXPECT().ExportZip(gomock.Any(), uint64(7), v.Name, v.Filter, wantQ).Return([]byte("PK-zip"), nil)

	w := doJSON(r, http.MethodGet, "/api/analytics/dashboards/2/export?from=2026-10-18&to=2026-10-24", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="q4-launch-spring-summer-2026-10-18-2026-10-24.zip"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "PK-zip", w.Body.String())
}

func TestDashboardSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Default": "default", "  Q4 Launch: Spring/Summer!! ": "q4-launch-spring-summer",
		"": "dashboard", "!!!": "dashboard", "Café 2026": "caf-2026", "a--b": "a-b",
	} {
		assert.Equal(t, want, dashboardSlug(in), in)
	}
}

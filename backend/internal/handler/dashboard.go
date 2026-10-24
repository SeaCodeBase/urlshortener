package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DashboardHandler serves the analytics dashboards of the active group.
type DashboardHandler struct {
	dashboards service.DashboardService
	analytics  service.AnalyticsService
	now        func() time.Time
}

func NewDashboardHandler(dashboards service.DashboardService, analytics service.AnalyticsService) *DashboardHandler {
	return &DashboardHandler{dashboards: dashboards, analytics: analytics, now: time.Now}
}

type createDashboardRequest struct {
	Name string `json:"name"`
}

// updateDashboardRequest holds the PATCH fields; absent fields stay nil and
// unknown fields are ignored.
type updateDashboardRequest struct {
	Name      *string                  `json:"name"`
	DateRange *string                  `json:"date_range"`
	Filter    *service.DashboardFilter `json:"filter"`
	Layout    *service.DashboardLayout `json:"layout"`
}

func (h *DashboardHandler) List(c *gin.Context) {
	views, err := h.dashboards.List(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), c.Query("sort"))
	if writeDashboardError(c, "list dashboards", err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"dashboards": views, "total": len(views)})
}

func (h *DashboardHandler) Create(c *gin.Context) {
	var req createDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	view, err := h.dashboards.Create(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), req.Name)
	if writeDashboardError(c, "create dashboard", err) {
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (h *DashboardHandler) Get(c *gin.Context) {
	view, ok := h.load(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *DashboardHandler) Update(c *gin.Context) {
	id, ok := parseDashboardID(c)
	if !ok {
		return
	}
	var req updateDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == nil && req.DateRange == nil && req.Filter == nil && req.Layout == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	patch := service.DashboardPatch{Name: req.Name, DateRange: req.DateRange, Filter: req.Filter, Layout: req.Layout}
	view, err := h.dashboards.Update(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, patch)
	if writeDashboardError(c, "update dashboard", err) {
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *DashboardHandler) Duplicate(c *gin.Context) {
	id, ok := parseDashboardID(c)
	if !ok {
		return
	}
	view, err := h.dashboards.Duplicate(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writeDashboardError(c, "duplicate dashboard", err) {
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (h *DashboardHandler) Delete(c *gin.Context) {
	id, ok := parseDashboardID(c)
	if !ok {
		return
	}
	err := h.dashboards.Delete(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writeDashboardError(c, "delete dashboard", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// Data returns the module data of the dashboard's stored filter over the
// from/to/compare window.
func (h *DashboardHandler) Data(c *gin.Context) {
	view, q, ok := h.loadWithQuery(c)
	if !ok {
		return
	}
	data, err := h.analytics.DashboardData(c.Request.Context(), view.GroupID, view.Filter, q)
	if writeDashboardError(c, "get dashboard data", err) {
		return
	}
	c.JSON(http.StatusOK, data)
}

// Export returns a zip with one CSV per module of the dashboard's data.
func (h *DashboardHandler) Export(c *gin.Context) {
	view, q, ok := h.loadWithQuery(c)
	if !ok {
		return
	}
	zip, err := h.analytics.ExportZip(c.Request.Context(), view.GroupID, view.Name, view.Filter, q)
	if writeDashboardError(c, "export dashboard", err) {
		return
	}
	filename := dashboardSlug(view.Name) + "-" + q.From.Format("2006-01-02") + "-" + q.To.Format("2006-01-02") + ".zip"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/zip", zip)
}

// load returns the dashboard named by the :id param of the active group,
// writing the error response when it cannot.
func (h *DashboardHandler) load(c *gin.Context) (*service.DashboardView, bool) {
	id, ok := parseDashboardID(c)
	if !ok {
		return nil, false
	}
	view, err := h.dashboards.Get(c.Request.Context(), middleware.GetGroupID(c), id)
	if writeDashboardError(c, "get dashboard", err) {
		return nil, false
	}
	return view, true
}

// loadWithQuery parses the window query, then loads the dashboard. The
// source param does not apply to dashboards (their filter scopes the data).
// When neither from nor to is given, the window is the dashboard's stored
// date range in UTC days instead of the 30-day default.
func (h *DashboardHandler) loadWithQuery(c *gin.Context) (*service.DashboardView, service.StatsQuery, bool) {
	now := h.now()
	q, err := parseStatsQuery(c, now)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, q, false
	}
	q.Source = ""
	view, ok := h.load(c)
	if !ok {
		return nil, q, false
	}
	if c.Query("from") == "" && c.Query("to") == "" {
		q.From, q.To = service.DashboardWindow(view.DateRange, now)
	}
	return view, q, true
}

func parseDashboardID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dashboard ID"})
		return 0, false
	}
	return id, true
}

// writeDashboardError writes the response for a non-nil err and reports
// whether it did.
func writeDashboardError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	var ve *service.ValidationError
	var qe *service.StatsQueryError
	switch {
	case errors.As(err, &ve):
		c.JSON(http.StatusBadRequest, gin.H{"error": ve.Msg})
	case errors.As(err, &qe):
		c.JSON(http.StatusBadRequest, gin.H{"error": qe.Msg})
	case errors.Is(err, repository.ErrDashboardNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "dashboard not found"})
	case errors.Is(err, service.ErrDashboardDefault):
		c.JSON(http.StatusConflict, gin.H{"error": service.ErrDashboardDefault.Error()})
	default:
		logger.Error(c.Request.Context(), "dashboard: failed to "+op,
			zap.Uint64("group_id", middleware.GetGroupID(c)),
			zap.String("dashboard_id", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to " + op})
	}
	return true
}

// dashboardSlug lowercases name and turns each run of non-alphanumeric
// (ASCII) characters into one "-", trimmed; "dashboard" when nothing is left.
func dashboardSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	if b.Len() == 0 {
		return "dashboard"
	}
	return b.String()
}

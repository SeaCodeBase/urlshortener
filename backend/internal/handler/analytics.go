package handler

import (
	"net/http"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AnalyticsHandler serves group-level analytics.
type AnalyticsHandler struct {
	statsService     service.StatsService
	analyticsService service.AnalyticsService
	now              func() time.Time
}

func NewAnalyticsHandler(statsService service.StatsService, analyticsService service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{statsService: statsService, analyticsService: analyticsService, now: time.Now}
}

// todayRanges maps the range parameter to its length in days, ending today.
var todayRanges = map[string]int{"today": 1, "7d": 7, "30d": 30}

// Today returns the active group's engagements (clicks from every source)
// for range=today|7d|30d, each ending today (UTC).
func (h *AnalyticsHandler) Today(c *gin.Context) {
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)
	rng := c.DefaultQuery("range", "today")
	days, ok := todayRanges[rng]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range must be today, 7d or 30d"})
		return
	}

	n := h.now().UTC()
	to := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -(days - 1))
	count, err := h.statsService.GroupEngagements(ctx, groupID, from, to)
	if err != nil {
		logger.Error(ctx, "analytics: failed to count engagements",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get engagements"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"engagements": count, "range": rng})
}

// Overview returns the analytics landing page data of the active group.
func (h *AnalyticsHandler) Overview(c *gin.Context) {
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)
	overview, err := h.analyticsService.Overview(ctx, groupID)
	if err != nil {
		logger.Error(ctx, "analytics: failed to get overview",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get overview"})
		return
	}
	c.JSON(http.StatusOK, overview)
}

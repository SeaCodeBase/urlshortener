package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const activityDateLayout = "2006-01-02"

type ActivityHandler struct {
	activityService service.ActivityService
}

func NewActivityHandler(activityService service.ActivityService) *ActivityHandler {
	return &ActivityHandler{activityService: activityService}
}

// parseDateParam parses an optional YYYY-MM-DD query parameter as UTC midnight.
func parseDateParam(c *gin.Context, name string) (*time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	t, err := time.ParseInLocation(activityDateLayout, raw, time.UTC)
	if err != nil {
		return nil, false
	}
	return &t, true
}

// parseIntParam parses an optional integer query parameter.
func parseIntParam(c *gin.Context, name string, def int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

// List handles GET /api/activity-log?from=YYYY-MM-DD&to=YYYY-MM-DD&page&limit.
// Dates are UTC; "to" includes the whole day.
func (h *ActivityHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)

	from, ok := parseDateParam(c, "from")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must be a date (YYYY-MM-DD)"})
		return
	}
	to, ok := parseDateParam(c, "to")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to must be a date (YYYY-MM-DD)"})
		return
	}
	if to != nil {
		end := to.AddDate(0, 0, 1) // exclusive end: start of the next day
		to = &end
	}
	if from != nil && to != nil && !from.Before(*to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must not be after to"})
		return
	}
	page, ok := parseIntParam(c, "page", 1)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a number"})
		return
	}
	limit, ok := parseIntParam(c, "limit", service.DefaultActivityLimit)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
		return
	}
	if page < 1 {
		page = 1
	}
	if page > maxPage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page must be at most 10000"})
		return
	}
	if limit < 1 {
		limit = service.DefaultActivityLimit
	}
	if limit > service.MaxActivityLimit {
		limit = service.MaxActivityLimit
	}

	res, err := h.activityService.List(ctx, groupID, from, to, page, limit)
	if err != nil {
		logger.Error(ctx, "activity-handler: failed to list activity",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list activity"})
		return
	}
	if res.Entries == nil {
		res.Entries = []model.ActivityEntry{}
	}
	c.JSON(http.StatusOK, res)
}

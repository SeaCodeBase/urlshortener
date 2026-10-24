package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StatsHandler struct {
	statsService service.StatsService
	now          func() time.Time
}

func NewStatsHandler(statsService service.StatsService) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
		now:          time.Now,
	}
}

// parseStatsQuery reads from, to (YYYY-MM-DD, UTC), unit, compare and source
// and applies the service defaults and limits.
func parseStatsQuery(c *gin.Context, now time.Time) (service.StatsQuery, error) {
	var q service.StatsQuery
	for _, p := range []struct {
		name string
		dst  *time.Time
	}{{"from", &q.From}, {"to", &q.To}} {
		if raw := c.Query(p.name); raw != "" {
			t, err := time.ParseInLocation("2006-01-02", raw, time.UTC)
			if err != nil {
				return q, &service.StatsQueryError{Msg: "invalid " + p.name + " date, expected YYYY-MM-DD"}
			}
			*p.dst = t
		}
	}
	q.Unit = c.Query("unit")
	switch c.DefaultQuery("compare", "none") {
	case "none":
	case "previous":
		q.Compare = true
	default:
		return q, &service.StatsQueryError{Msg: "compare must be previous or none"}
	}
	q.Source = c.Query("source")
	return service.NormalizeStatsQuery(q, now)
}

func (h *StatsHandler) GetLinkStats(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn(ctx, "stats: invalid link ID",
			zap.String("link_id_param", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	q, err := parseStatsQuery(c, h.now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// compare_link is read here, not in parseStatsQuery, so QR stats ignore it.
	if raw := c.Query("compare_link"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "compare_link must be a link id"})
			return
		}
		if id == linkID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "compare_link must differ from the link"})
			return
		}
		q.CompareLinkID = id
	}

	stats, err := h.statsService.GetLinkStats(ctx, userID, linkID, q)
	var qe *service.StatsQueryError
	if errors.As(err, &qe) {
		c.JSON(http.StatusBadRequest, gin.H{"error": qe.Msg})
		return
	}
	if errors.Is(err, repository.ErrLinkNotFound) || errors.Is(err, service.ErrLinkNotFound) ||
		errors.Is(err, service.ErrNotLinkOwner) {
		logger.Warn(ctx, "stats: link not found",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		logger.Error(ctx, "stats: failed to get stats",
			zap.Uint64("link_id", linkID),
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

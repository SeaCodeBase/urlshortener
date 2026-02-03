package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StatsHandler struct {
	statsService service.StatsService
}

func NewStatsHandler(statsService service.StatsService) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
	}
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

	stats, err := h.statsService.GetLinkStats(ctx, userID, linkID)
	if errors.Is(err, repository.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
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

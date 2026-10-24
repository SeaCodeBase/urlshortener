package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LinkCounter counts a group's links matching a filter
// (implemented by repository.LinkRepository).
type LinkCounter interface {
	CountByGroup(ctx context.Context, groupID uint64, f repository.LinkFilter) (int64, error)
}

// SearchHandler serves the top-bar global search (spec §5.12).
type SearchHandler struct {
	links LinkCounter
}

func NewSearchHandler(links LinkCounter) *SearchHandler {
	return &SearchHandler{links: links}
}

// Search returns how many of the active group's links and QR codes match q.
// QR codes do not exist yet, so their count is always 0.
func (h *SearchHandler) Search(c *gin.Context) {
	ctx := c.Request.Context()
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}
	groupID := middleware.GetGroupID(c)
	links, err := h.links.CountByGroup(ctx, groupID, repository.LinkFilter{Query: q})
	if err != nil {
		logger.Error(ctx, "search: failed to count links",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"links": links, "qr_codes": 0})
}

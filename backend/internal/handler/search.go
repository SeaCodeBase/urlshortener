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

// QRCounter counts a group's QR codes matching a filter
// (implemented by repository.QRRepository).
type QRCounter interface {
	CountByGroup(ctx context.Context, groupID uint64, f repository.QRFilter) (int64, error)
}

// SearchHandler serves the top-bar global search (spec §5.12).
type SearchHandler struct {
	links LinkCounter
	qrs   QRCounter
}

func NewSearchHandler(links LinkCounter, qrs QRCounter) *SearchHandler {
	return &SearchHandler{links: links, qrs: qrs}
}

// Search returns how many of the active group's links and QR codes match q.
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
	qrCodes, err := h.qrs.CountByGroup(ctx, groupID, repository.QRFilter{Query: q})
	if err != nil {
		logger.Error(ctx, "search: failed to count qr codes",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"links": links, "qr_codes": qrCodes})
}

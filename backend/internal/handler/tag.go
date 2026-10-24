package handler

import (
	"errors"
	"net/http"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type TagHandler struct {
	tagService service.TagService
}

func NewTagHandler(tagService service.TagService) *TagHandler {
	return &TagHandler{tagService: tagService}
}

// List handles GET /api/tags?type=link and returns the active group's
// distinct tags.
func (h *TagHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)
	typ := c.DefaultQuery("type", service.TagTypeLink)

	tags, err := h.tagService.ListByGroup(ctx, groupID, typ)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTagType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type must be link or qr"})
			return
		}
		logger.Error(ctx, "tag-handler: failed to list tags",
			zap.Uint64("group_id", groupID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tags"})
		return
	}
	if tags == nil {
		tags = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

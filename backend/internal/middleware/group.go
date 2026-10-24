package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GroupHeader names the group a request acts on; when absent the user's
// default group is used.
const GroupHeader = "X-Group-ID"

// GroupMiddleware resolves the active group for an authenticated request and
// stores its ID under "group_id". It must run after AuthMiddleware.
func GroupMiddleware(groupSvc service.GroupService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := GetUserID(c)

		var headerGroupID *uint64
		if raw := c.GetHeader(GroupHeader); raw != "" {
			id, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
				return
			}
			headerGroupID = &id
		}

		group, err := groupSvc.ResolveActive(ctx, userID, headerGroupID)
		if errors.Is(err, service.ErrNotGroupOwner) || errors.Is(err, service.ErrGroupNotFound) {
			logger.Warn(ctx, "group: group not accessible",
				zap.Uint64("user_id", userID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "group not found"})
			return
		}
		if err != nil {
			logger.Error(ctx, "group: failed to resolve active group",
				zap.Uint64("user_id", userID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve group"})
			return
		}

		c.Set("group_id", group.ID)
		c.Next()
	}
}

// GetGroupID returns the active group ID set by GroupMiddleware, or 0.
func GetGroupID(c *gin.Context) uint64 {
	groupID, exists := c.Get("group_id")
	if !exists {
		return 0
	}
	return groupID.(uint64)
}

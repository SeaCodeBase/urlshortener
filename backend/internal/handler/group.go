package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GroupHandler serves /api/groups. Routes use auth only (no active group):
// the group being read or written is the one in the path.
type GroupHandler struct {
	groups   service.GroupService
	activity service.ActivityService
}

func NewGroupHandler(groups service.GroupService, activity service.ActivityService) *GroupHandler {
	return &GroupHandler{groups: groups, activity: activity}
}

type groupResponse struct {
	ID              uint64    `json:"id"`
	Name            string    `json:"name"`
	DefaultDomainID *uint64   `json:"default_domain_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toGroupResponse(g *model.Group) groupResponse {
	return groupResponse{ID: g.ID, Name: g.Name, DefaultDomainID: g.DefaultDomainID, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

type createGroupRequest struct {
	Name string `json:"name"`
}

func (h *GroupHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	groups, err := h.groups.List(ctx, userID)
	if writeGroupError(c, "list-groups", err) {
		return
	}
	items := make([]groupResponse, len(groups))
	for i := range groups {
		items[i] = toGroupResponse(&groups[i])
	}
	c.JSON(http.StatusOK, gin.H{"groups": items})
}

func (h *GroupHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	group, err := h.groups.Create(ctx, userID, req.Name)
	if writeGroupError(c, "create-group", err) {
		return
	}
	h.activity.Log(context.WithoutCancel(ctx), userID, group.ID, service.ActionGroupCreate, service.EntityGroup, group.ID,
		map[string]any{"name": group.Name})
	c.JSON(http.StatusCreated, toGroupResponse(group))
}

func (h *GroupHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseGroupID(c)
	if !ok {
		return
	}
	group, err := h.groups.Get(ctx, userID, id)
	if writeGroupError(c, "get-group", err) {
		return
	}
	c.JSON(http.StatusOK, toGroupResponse(group))
}

func (h *GroupHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	id, ok := parseGroupID(c)
	if !ok {
		return
	}
	var input service.UpdateGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	group, err := h.groups.Update(ctx, userID, id, input)
	if writeGroupError(c, "update-group", err) {
		return
	}
	h.activity.Log(context.WithoutCancel(ctx), userID, group.ID, service.ActionGroupUpdate, service.EntityGroup, group.ID,
		map[string]any{"name": group.Name, "default_domain_id": group.DefaultDomainID})
	c.JSON(http.StatusOK, toGroupResponse(group))
}

func parseGroupID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group ID"})
		return 0, false
	}
	return id, true
}

// writeGroupError maps group service errors to responses. It reports whether
// err was handled (non-nil).
func writeGroupError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	ctx := c.Request.Context()
	switch {
	case errors.Is(err, service.ErrGroupNotFound), errors.Is(err, service.ErrNotGroupOwner):
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
	case errors.Is(err, service.ErrNotDomainOwner):
		c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
	case errors.Is(err, service.ErrDomainNotVerified):
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is not verified"})
	case errors.Is(err, service.ErrInvalidGroupName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		logger.Error(ctx, op+": failed", zap.Uint64("user_id", middleware.GetUserID(c)), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to " + strings.ReplaceAll(op, "-", " ")})
		return true
	}
	logger.Warn(ctx, op+": rejected", zap.Error(err))
	return true
}

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DomainHandler serves /api/domains (custom domains, spec §5.10).
type DomainHandler struct {
	domains  service.DomainService
	activity service.ActivityService
}

func NewDomainHandler(domains service.DomainService, activity service.ActivityService) *DomainHandler {
	return &DomainHandler{domains: domains, activity: activity}
}

// updateDomainRequest distinguishes an absent group_id (unchanged) from
// null (clear) by keeping it raw.
type updateDomainRequest struct {
	RootRedirect     *string         `json:"root_redirect"`
	NotFoundRedirect *string         `json:"not_found_redirect"`
	GroupID          json.RawMessage `json:"group_id"`
}

func (h *DomainHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	domains, err := h.domains.List(ctx, middleware.GetUserID(c))
	if writeDomainError(c, "list-domains", err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (h *DomainHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var input service.CreateDomainInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	domain, err := h.domains.Create(ctx, userID, input)
	if writeDomainError(c, "create-domain", err) {
		return
	}
	h.log(c, domain, service.ActionDomainCreate, map[string]any{"domain": domain.Domain})
	c.JSON(http.StatusCreated, domain)
}

func (h *DomainHandler) Verify(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := parseDomainID(c)
	if !ok {
		return
	}
	domain, err := h.domains.Verify(ctx, middleware.GetUserID(c), id)
	if writeDomainError(c, "verify-domain", err) {
		return
	}
	h.log(c, domain, service.ActionDomainVerify, map[string]any{
		"domain": domain.Domain, "status": domain.VerificationStatus,
	})
	c.JSON(http.StatusOK, domain)
}

func (h *DomainHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := parseDomainID(c)
	if !ok {
		return
	}
	var req updateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	input := service.UpdateDomainInput{RootRedirect: req.RootRedirect, NotFoundRedirect: req.NotFoundRedirect}
	if raw := bytes.TrimSpace(req.GroupID); len(raw) > 0 {
		if bytes.Equal(raw, []byte("null")) {
			input.ClearGroup = true
		} else {
			var gid uint64
			if err := json.Unmarshal(raw, &gid); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "group_id must be a group ID or null"})
				return
			}
			input.GroupID = &gid
		}
	}
	domain, err := h.domains.Update(ctx, middleware.GetUserID(c), id, input)
	if writeDomainError(c, "update-domain", err) {
		return
	}
	h.log(c, domain, service.ActionDomainUpdate, map[string]any{"domain": domain.Domain})
	c.JSON(http.StatusOK, domain)
}

func (h *DomainHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	id, ok := parseDomainID(c)
	if !ok {
		return
	}
	domain, err := h.domains.Delete(ctx, middleware.GetUserID(c), id)
	if writeDomainError(c, "delete-domain", err) {
		return
	}
	h.log(c, domain, service.ActionDomainDelete, map[string]any{"domain": domain.Domain})
	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted"})
}

// log records a domain action in the domain's group, or in the active group
// when the domain is shared by all groups.
func (h *DomainHandler) log(c *gin.Context, domain *model.Domain, action string, meta map[string]any) {
	groupID := middleware.GetGroupID(c)
	if domain.GroupID != nil {
		groupID = *domain.GroupID
	}
	if groupID == 0 {
		return
	}
	h.activity.Log(context.WithoutCancel(c.Request.Context()), middleware.GetUserID(c), groupID, action,
		service.EntityDomain, domain.ID, meta)
}

func parseDomainID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain ID"})
		return 0, false
	}
	return id, true
}

// writeDomainError maps domain service errors to responses. It reports
// whether err was handled (non-nil).
func writeDomainError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	ctx := c.Request.Context()
	switch {
	case errors.Is(err, service.ErrNotDomainOwner):
		c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
	case errors.Is(err, service.ErrGroupNotFound), errors.Is(err, service.ErrNotGroupOwner):
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
	case errors.Is(err, service.ErrDomainExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Domain already in use"})
	case errors.Is(err, service.ErrInvalidDomain), errors.Is(err, service.ErrInvalidRedirectURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		logger.Error(ctx, op+": failed", zap.Uint64("user_id", middleware.GetUserID(c)), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to " + strings.ReplaceAll(op, "-", " ")})
		return true
	}
	logger.Warn(ctx, op+": rejected", zap.Error(err))
	return true
}

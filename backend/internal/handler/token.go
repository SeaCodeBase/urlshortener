package handler

import (
	"context"
	"errors"
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

type TokenHandler struct {
	tokenService service.TokenService
	activity     service.ActivityService
}

func NewTokenHandler(tokenService service.TokenService, activity service.ActivityService) *TokenHandler {
	return &TokenHandler{tokenService: tokenService, activity: activity}
}

// logActivity records a token action in the active group (tokens are
// user-level; the entry shows up in the group the user was working in).
func (h *TokenHandler) logActivity(c *gin.Context, action string, tokenID uint64, meta map[string]any) {
	groupID := middleware.GetGroupID(c)
	if groupID == 0 {
		return
	}
	h.activity.Log(context.WithoutCancel(c.Request.Context()), middleware.GetUserID(c), groupID, action,
		service.EntityToken, tokenID, meta)
}

type CreateTokenRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateTokenResponse is the only response that carries the raw token.
type CreateTokenResponse struct {
	Token     string    `json:"token"`
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
}

type TokenListItem struct {
	ID         uint64         `json:"id"`
	Name       string         `json:"name"`
	Prefix     string         `json:"prefix"`
	LastUsedAt model.NullTime `json:"last_used_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (h *TokenHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	tokens, err := h.tokenService.List(ctx, userID)
	if err != nil {
		logger.Error(ctx, "token-handler: failed to list tokens",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tokens"})
		return
	}

	items := make([]TokenListItem, 0, len(tokens))
	for _, t := range tokens {
		items = append(items, TokenListItem{
			ID:         t.ID,
			Name:       t.Name,
			Prefix:     t.TokenPrefix,
			LastUsedAt: t.LastUsedAt,
			CreatedAt:  t.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"tokens": items})
}

func (h *TokenHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	var req CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(ctx, "token-handler: invalid request body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and password are required"})
		return
	}

	raw, token, err := h.tokenService.Create(ctx, userID, req.Name, req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if errors.Is(err, service.ErrInvalidTokenName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		logger.Error(ctx, "token-handler: failed to create token",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	h.logActivity(c, service.ActionTokenCreate, token.ID, map[string]any{"name": token.Name, "prefix": token.TokenPrefix})
	c.JSON(http.StatusCreated, CreateTokenResponse{
		Token:     raw,
		ID:        token.ID,
		Name:      token.Name,
		Prefix:    token.TokenPrefix,
		CreatedAt: token.CreatedAt,
	})
}

func (h *TokenHandler) Revoke(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}

	err = h.tokenService.Revoke(ctx, userID, id)
	if errors.Is(err, service.ErrAPITokenNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}
	if err != nil {
		logger.Error(ctx, "token-handler: failed to revoke token",
			zap.Uint64("user_id", userID),
			zap.Uint64("token_id", id),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke token"})
		return
	}
	h.logActivity(c, service.ActionTokenRevoke, id, nil)
	c.Status(http.StatusNoContent)
}

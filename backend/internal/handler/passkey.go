package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"go.uber.org/zap"
)

type PasskeyHandler struct {
	passkeyService service.PasskeyService
}

func NewPasskeyHandler(passkeyService service.PasskeyService) *PasskeyHandler {
	return &PasskeyHandler{passkeyService: passkeyService}
}

type BeginRegistrationResponse struct {
	Options     *protocol.CredentialCreation `json:"options"`
	SessionData string                       `json:"session_data"`
}

func (h *PasskeyHandler) BeginRegistration(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	options, sessionData, err := h.passkeyService.BeginRegistration(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-register-begin: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin registration"})
		return
	}
	c.JSON(http.StatusOK, BeginRegistrationResponse{
		Options:     options,
		SessionData: sessionData,
	})
}

type FinishRegistrationRequest struct {
	SessionData string `json:"session_data"`
	Name        string `json:"name"`
}

func (h *PasskeyHandler) FinishRegistration(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	// Read the raw body for WebAuthn credential parsing
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Warn(ctx, "passkey-register-finish: failed to read request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Parse session_data and name from the body
	var wrapper struct {
		SessionData string `json:"session_data"`
		Name        string `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		logger.Warn(ctx, "passkey-register-finish: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if wrapper.SessionData == "" || wrapper.Name == "" {
		logger.Warn(ctx, "passkey-register-finish: missing required fields",
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_data and name are required"})
		return
	}

	// The body contains both wrapper fields and WebAuthn credential data
	passkey, err := h.passkeyService.FinishRegistration(ctx, userID, wrapper.SessionData, bodyBytes, wrapper.Name)
	if err != nil {
		logger.Error(ctx, "passkey-register-finish: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, passkey)
}

func (h *PasskeyHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	passkeys, err := h.passkeyService.List(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-list: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list passkeys"})
		return
	}
	c.JSON(http.StatusOK, passkeys)
}

type RenameRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *PasskeyHandler) Rename(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	passkeyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn(ctx, "passkey-rename: invalid passkey ID",
			zap.String("passkey_id_param", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(ctx, "passkey-rename: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Uint64("passkey_id", passkeyID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.passkeyService.Rename(ctx, userID, passkeyID, req.Name); err != nil {
		if err == service.ErrPasskeyNotOwned {
			logger.Warn(ctx, "passkey-rename: not owned",
				zap.Uint64("user_id", userID),
				zap.Uint64("passkey_id", passkeyID),
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "passkey not found"})
			return
		}
		logger.Error(ctx, "passkey-rename: failed",
			zap.Uint64("user_id", userID),
			zap.Uint64("passkey_id", passkeyID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rename passkey"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey renamed"})
}

func (h *PasskeyHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	passkeyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn(ctx, "passkey-delete: invalid passkey ID",
			zap.String("passkey_id_param", c.Param("id")),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}
	if err := h.passkeyService.Delete(ctx, userID, passkeyID); err != nil {
		if err == service.ErrPasskeyNotOwned {
			logger.Warn(ctx, "passkey-delete: not owned",
				zap.Uint64("user_id", userID),
				zap.Uint64("passkey_id", passkeyID),
			)
			c.JSON(http.StatusForbidden, gin.H{"error": "passkey not found"})
			return
		}
		logger.Error(ctx, "passkey-delete: failed",
			zap.Uint64("user_id", userID),
			zap.Uint64("passkey_id", passkeyID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete passkey"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey deleted"})
}

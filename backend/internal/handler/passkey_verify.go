package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
)

type PasskeyVerifyHandler struct {
	passkeyService service.PasskeyService
	authService    service.AuthService
}

func NewPasskeyVerifyHandler(passkeyService service.PasskeyService, authService service.AuthService) *PasskeyVerifyHandler {
	return &PasskeyVerifyHandler{
		passkeyService: passkeyService,
		authService:    authService,
	}
}

type BeginVerifyRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

type BeginVerifyResponse struct {
	Options     *protocol.CredentialAssertion `json:"options"`
	SessionData string                        `json:"session_data"`
}

func (h *PasskeyVerifyHandler) BeginVerify(c *gin.Context) {
	var req BeginVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	options, sessionData, err := h.passkeyService.BeginLogin(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin verification"})
		return
	}
	c.JSON(http.StatusOK, BeginVerifyResponse{
		Options:     options,
		SessionData: sessionData,
	})
}

func (h *PasskeyVerifyHandler) FinishVerify(c *gin.Context) {
	// Read the raw body for WebAuthn credential parsing
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Parse user_id and session_data from the body
	var wrapper struct {
		UserID      uint64 `json:"user_id"`
		SessionData string `json:"session_data"`
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if wrapper.UserID == 0 || wrapper.SessionData == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and session_data are required"})
		return
	}

	if err := h.passkeyService.FinishLogin(c.Request.Context(), wrapper.UserID, wrapper.SessionData, bodyBytes); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "passkey verification failed"})
		return
	}

	// Generate token after successful passkey verification
	user, err := h.authService.GetUserByID(c.Request.Context(), wrapper.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}
	token, err := h.authService.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

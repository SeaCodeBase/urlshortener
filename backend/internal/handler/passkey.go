package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
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
	userID := middleware.GetUserID(c)
	options, sessionData, err := h.passkeyService.BeginRegistration(c.Request.Context(), userID)
	if err != nil {
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
	userID := middleware.GetUserID(c)

	// Read the raw body for WebAuthn credential parsing
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Parse session_data and name from the body
	var wrapper struct {
		SessionData string `json:"session_data"`
		Name        string `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if wrapper.SessionData == "" || wrapper.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_data and name are required"})
		return
	}

	// The body contains both wrapper fields and WebAuthn credential data
	passkey, err := h.passkeyService.FinishRegistration(c.Request.Context(), userID, wrapper.SessionData, bodyBytes, wrapper.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, passkey)
}

func (h *PasskeyHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	passkeys, err := h.passkeyService.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list passkeys"})
		return
	}
	c.JSON(http.StatusOK, passkeys)
}

type RenameRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *PasskeyHandler) Rename(c *gin.Context) {
	userID := middleware.GetUserID(c)
	passkeyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.passkeyService.Rename(c.Request.Context(), userID, passkeyID, req.Name); err != nil {
		if err == service.ErrPasskeyNotOwned {
			c.JSON(http.StatusForbidden, gin.H{"error": "passkey not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rename passkey"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey renamed"})
}

func (h *PasskeyHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	passkeyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid passkey id"})
		return
	}
	if err := h.passkeyService.Delete(c.Request.Context(), userID, passkeyID); err != nil {
		if err == service.ErrPasskeyNotOwned {
			c.JSON(http.StatusForbidden, gin.H{"error": "passkey not found"})
			return
		}
		if err == service.ErrLastPasskey {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete last passkey"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete passkey"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "passkey deleted"})
}

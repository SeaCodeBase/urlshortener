// backend/internal/handler/auth.go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
)

type AuthHandler struct {
	authService    service.AuthService
	passkeyService service.PasskeyService
}

func NewAuthHandler(authService service.AuthService, passkeyService service.PasskeyService) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		passkeyService: passkeyService,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), input)
	if errors.Is(err, service.ErrEmailTaken) {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

type LoginResponse struct {
	Token           string      `json:"token,omitempty"`
	User            interface{} `json:"user,omitempty"`
	RequiresPasskey bool        `json:"requires_passkey,omitempty"`
	UserID          uint64      `json:"user_id,omitempty"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), input)
	if errors.Is(err, service.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	// Check if user has passkeys
	hasPasskeys, err := h.passkeyService.HasPasskeys(c.Request.Context(), resp.User.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check passkeys"})
		return
	}

	if hasPasskeys {
		c.JSON(http.StatusOK, LoginResponse{
			RequiresPasskey: true,
			UserID:          resp.User.ID,
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.authService.ChangePassword(c.Request.Context(), userID, input)
	if errors.Is(err, service.ErrWrongPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

type UpdateMeRequest struct {
	DisplayName string `json:"display_name"`
}

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.authService.UpdateDisplayName(c.Request.Context(), userID, req.DisplayName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}
	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}
	c.JSON(http.StatusOK, user)
}

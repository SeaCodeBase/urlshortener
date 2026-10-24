// backend/internal/handler/auth.go
package handler

import (
	"errors"
	"net/http"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthHandler struct {
	authService    service.AuthService
	passkeyService service.PasskeyService
	cfg            *config.Config
}

func NewAuthHandler(authService service.AuthService, passkeyService service.PasskeyService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		passkeyService: passkeyService,
		cfg:            cfg,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if registration is allowed
	if h.cfg.Server.AllowRegistration != nil && !*h.cfg.Server.AllowRegistration {
		logger.Warn(ctx, "register: registration is disabled")
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled"})
		return
	}

	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "register: invalid request body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Register(ctx, input)
	if errors.Is(err, service.ErrEmailTaken) {
		logger.Warn(ctx, "register: email already taken",
			zap.String("email", input.Email),
		)
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != nil {
		logger.Error(ctx, "register: failed to register user",
			zap.String("email", input.Email),
			zap.Error(err),
		)
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
	ctx := c.Request.Context()
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "login: invalid request body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Login(ctx, input)
	if errors.Is(err, service.ErrInvalidCredentials) {
		logger.Warn(ctx, "login: invalid credentials",
			zap.String("email", input.Email),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		logger.Error(ctx, "login: failed to authenticate",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	hasPasskeys, err := h.passkeyService.HasPasskeys(ctx, resp.User.ID)
	if err != nil {
		logger.Error(ctx, "login: failed to check passkeys",
			zap.Uint64("user_id", resp.User.ID),
			zap.Error(err),
		)
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
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "me: failed to get user",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "change-password: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.authService.ChangePassword(ctx, userID, input)
	if errors.Is(err, service.ErrWrongPassword) {
		logger.Warn(ctx, "change-password: wrong current password",
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	if err != nil {
		logger.Error(ctx, "change-password: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// UpdateMe applies a partial profile/account update
// ({display_name, account_name, default_group_id, home_checklist_dismissed})
// and returns the updated user.
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var input service.UpdateAccountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "update-me: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	err := h.authService.UpdateAccount(ctx, userID, input)
	switch {
	case errors.Is(err, service.ErrGroupNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	case errors.Is(err, service.ErrInvalidAccountName):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case err != nil:
		logger.Error(ctx, "update-me: failed to update account",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}
	h.respondWithUser(c, "update-me")
}

type UpdateEmailRequest struct {
	Password string `json:"password" binding:"required"`
	NewEmail string `json:"new_email" binding:"required"`
}

// UpdateEmail changes the user's email after re-checking the password.
func (h *AuthHandler) UpdateEmail(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password and new_email are required"})
		return
	}
	err := h.authService.UpdateEmail(ctx, userID, req.Password, req.NewEmail)
	switch {
	case errors.Is(err, service.ErrWrongPassword):
		logger.Warn(ctx, "update-email: wrong password", zap.Uint64("user_id", userID))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "password is incorrect"})
		return
	case errors.Is(err, service.ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
		return
	case err != nil:
		logger.Error(ctx, "update-email: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update email"})
		return
	}
	h.respondWithUser(c, "update-email")
}

func (h *AuthHandler) respondWithUser(c *gin.Context, op string) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, op+": failed to get user after update",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}
	c.JSON(http.StatusOK, user)
}

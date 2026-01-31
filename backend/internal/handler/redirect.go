// backend/internal/handler/redirect.go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jose/urlshortener/internal/service"
)

type RedirectHandler struct {
	redirectService *service.RedirectService
}

func NewRedirectHandler(redirectService *service.RedirectService) *RedirectHandler {
	return &RedirectHandler{redirectService: redirectService}
}

func (h *RedirectHandler) Redirect(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid short code"})
		return
	}

	originalURL, linkID, err := h.redirectService.Resolve(c.Request.Context(), code)
	if errors.Is(err, service.ErrLinkNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if errors.Is(err, service.ErrLinkExpired) || errors.Is(err, service.ErrLinkInactive) {
		c.JSON(http.StatusGone, gin.H{"error": "link is no longer available"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve link"})
		return
	}

	// TODO: Log click event to Redis buffer (Task 13)
	_ = linkID

	c.Redirect(http.StatusFound, originalURL)
}

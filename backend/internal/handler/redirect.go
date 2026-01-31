// backend/internal/handler/redirect.go
package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
)

type RedirectHandler struct {
	redirectService *service.RedirectService
	clickService    *service.ClickService
	ipSalt          string
}

func NewRedirectHandler(redirectService *service.RedirectService, clickService *service.ClickService, ipSalt string) *RedirectHandler {
	return &RedirectHandler{
		redirectService: redirectService,
		clickService:    clickService,
		ipSalt:          ipSalt,
	}
}

func (h *RedirectHandler) hashIP(ip string) string {
	hasher := sha256.New()
	hasher.Write([]byte(h.ipSalt + ip))
	return hex.EncodeToString(hasher.Sum(nil))
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

	// Record click event asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		event := service.ClickEvent{
			LinkID:      linkID,
			ClickedAt:   time.Now().UTC(),
			IPHash:      h.hashIP(c.ClientIP()),
			IPAddress:   c.ClientIP(),
			UserAgent:   c.GetHeader("User-Agent"),
			Referrer:    c.GetHeader("Referer"),
			UTMSource:   c.Query("utm_source"),
			UTMMedium:   c.Query("utm_medium"),
			UTMCampaign: c.Query("utm_campaign"),
		}
		if err := h.clickService.RecordClick(ctx, event); err != nil {
			logger.Log.Warnf("failed to record click: %v", err)
		}
	}()

	c.Redirect(http.StatusFound, originalURL)
}

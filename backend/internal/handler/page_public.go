package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PublicPageHandler serves published pages without authentication; drafts
// are never exposed.
type PublicPageHandler struct {
	svc service.PagePublicService
}

func NewPublicPageHandler(svc service.PagePublicService) *PublicPageHandler {
	return &PublicPageHandler{svc: svc}
}

// Get handles GET /public/pages/:slug: the published snapshot or 404.
func (h *PublicPageHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")
	page, err := h.svc.GetPublished(ctx, slug)
	switch {
	case errors.Is(err, repository.ErrPageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
	case err != nil:
		logger.Error(ctx, "public-page: failed to get page", zap.String("slug", slug), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get page"})
	default:
		c.JSON(http.StatusOK, page)
	}
}

// maxViewBody bounds the optional view beacon body.
const maxViewBody = 4 << 10

// viewReferrer returns the "referrer" of the optional JSON body (the
// visitor's document.referrer, sent by the frontend beacon whose own Referer
// header is the page itself) when present and non-empty, else the Referer
// header. Any content type is read; other fields and a malformed, non-JSON
// or oversized body are ignored.
func viewReferrer(c *gin.Context) string {
	if c.Request.Body != nil {
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxViewBody+1))
		if err == nil && len(raw) <= maxViewBody {
			var body struct {
				Referrer any `json:"referrer"`
			}
			if json.Unmarshal(raw, &body) == nil {
				if ref, ok := body.Referrer.(string); ok && strings.TrimSpace(ref) != "" {
					return strings.TrimSpace(ref)
				}
			}
		}
	}
	return c.GetHeader("Referer")
}

// View handles POST /public/pages/:slug/view {referrer?}: records one view
// of a published page and answers 204; 404 when unpublished.
func (h *PublicPageHandler) View(c *gin.Context) {
	ctx := c.Request.Context()
	slug := c.Param("slug")
	err := h.svc.RecordView(ctx, slug, service.ViewInput{
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Referrer:  viewReferrer(c),
	})
	switch {
	case errors.Is(err, repository.ErrPageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
	case err != nil:
		logger.Error(ctx, "public-page: failed to record view", zap.String("slug", slug), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record view"})
	default:
		c.Status(http.StatusNoContent)
	}
}

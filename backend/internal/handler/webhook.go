package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/gin-gonic/gin"
)

// WebhookHandler serves the active group's webhooks. Responses include the
// signing secret: every route is group-scoped, so only members see it.
type WebhookHandler struct {
	webhooks service.WebhookService
}

func NewWebhookHandler(webhooks service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhooks: webhooks}
}

// RegisterWebhookRoutes mounts /webhooks on api behind mw (the auth and
// group middleware).
func RegisterWebhookRoutes(api *gin.RouterGroup, h *WebhookHandler, mw ...gin.HandlerFunc) {
	g := api.Group("/webhooks", mw...)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/verify", h.Verify)
	g.POST("/:id/deactivate", h.Deactivate)
	g.POST("/:id/activate", h.Activate)
	g.GET("/:id/deliveries", h.Deliveries)
}

func (h *WebhookHandler) List(c *gin.Context) {
	ws, err := h.webhooks.List(c.Request.Context(), middleware.GetGroupID(c))
	if writeWebhookError(c, "list-webhooks", err) {
		return
	}
	if ws == nil {
		ws = []model.Webhook{}
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": ws})
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var in service.WebhookInput
	if !bindPageJSON(c, &in) {
		return
	}
	w, err := h.webhooks.Create(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), in)
	if writeWebhookError(c, "create-webhook", err) {
		return
	}
	c.JSON(http.StatusCreated, w)
}

func (h *WebhookHandler) Get(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	w, err := h.webhooks.Get(c.Request.Context(), middleware.GetGroupID(c), id)
	if writeWebhookError(c, "get-webhook", err) {
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *WebhookHandler) Update(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	var in service.WebhookInput
	if !bindPageJSON(c, &in) {
		return
	}
	w, err := h.webhooks.Update(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id, in)
	if writeWebhookError(c, "update-webhook", err) {
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	err := h.webhooks.Delete(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writeWebhookError(c, "delete-webhook", err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// Verify sends a signed ping and returns the delivery with the webhook's
// updated status.
func (h *WebhookHandler) Verify(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	groupID := middleware.GetGroupID(c)
	d, err := h.webhooks.Verify(ctx, middleware.GetUserID(c), groupID, id)
	if writeWebhookError(c, "verify-webhook", err) {
		return
	}
	w, err := h.webhooks.Get(ctx, groupID, id)
	if writeWebhookError(c, "verify-webhook", err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivery": d, "webhook": w})
}

func (h *WebhookHandler) Deactivate(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	w, err := h.webhooks.Deactivate(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writeWebhookError(c, "deactivate-webhook", err) {
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *WebhookHandler) Activate(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	w, err := h.webhooks.Activate(c.Request.Context(), middleware.GetUserID(c), middleware.GetGroupID(c), id)
	if writeWebhookError(c, "activate-webhook", err) {
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *WebhookHandler) Deliveries(c *gin.Context) {
	id, ok := parseWebhookID(c)
	if !ok {
		return
	}
	ds, err := h.webhooks.Deliveries(c.Request.Context(), middleware.GetGroupID(c), id)
	if writeWebhookError(c, "list-webhook-deliveries", err) {
		return
	}
	if ds == nil {
		ds = []model.WebhookDelivery{}
	}
	c.JSON(http.StatusOK, gin.H{"deliveries": ds})
}

func parseWebhookID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook ID"})
		return 0, false
	}
	return id, true
}

// writeWebhookError maps ValidationError to 400 and ErrWebhookNotFound to
// 404; anything else is a logged 500. It reports whether it wrote.
func writeWebhookError(c *gin.Context, op string, err error) bool {
	if err == nil {
		return false
	}
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		c.JSON(http.StatusBadRequest, gin.H{"error": ve.Msg})
	case errors.Is(err, repository.ErrWebhookNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
	default:
		return writeServiceError(c, op, err)
	}
	return true
}

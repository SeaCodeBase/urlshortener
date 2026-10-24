package handler

import (
	"net/http"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/gin-gonic/gin"
)

// MetaHandler serves public, non-secret configuration the web UI needs
// (GET /api/meta, no auth).
type MetaHandler struct {
	resp metaResponse
}

type metaResponse struct {
	BaseURL           string `json:"base_url"`
	RedirectHost      string `json:"redirect_host"`
	RedirectIP        string `json:"redirect_ip"`
	FaviconService    bool   `json:"favicon_service"`
	AllowRegistration bool   `json:"allow_registration"`
}

func NewMetaHandler(cfg *config.Config) *MetaHandler {
	return &MetaHandler{resp: metaResponse{
		BaseURL:           cfg.URLs.BaseURL,
		RedirectHost:      cfg.URLs.RedirectHost,
		RedirectIP:        cfg.URLs.RedirectIP,
		FaviconService:    cfg.UI.FaviconService == nil || *cfg.UI.FaviconService,
		AllowRegistration: cfg.Server.AllowRegistration == nil || *cfg.Server.AllowRegistration,
	}}
}

func (h *MetaHandler) Get(c *gin.Context) {
	c.JSON(http.StatusOK, h.resp)
}

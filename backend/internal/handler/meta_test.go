package handler

import (
	"net/http"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	favicons, allowReg := false, true
	cfg := &config.Config{}
	cfg.URLs.BaseURL = "https://sho.rt"
	cfg.URLs.RedirectHost = "sho.rt"
	cfg.URLs.RedirectIP = "203.0.113.7"
	cfg.UI.FaviconService = &favicons
	cfg.Server.AllowRegistration = &allowReg

	r := gin.New()
	r.GET("/api/meta", NewMetaHandler(cfg).Get)
	w := doJSON(r, http.MethodGet, "/api/meta", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"base_url":"https://sho.rt","redirect_host":"sho.rt","redirect_ip":"203.0.113.7",
		"favicon_service":false,"allow_registration":true}`, w.Body.String())

	// Unset booleans default to enabled, matching config.Load defaults.
	r = gin.New()
	r.GET("/api/meta", NewMetaHandler(&config.Config{}).Get)
	w = doJSON(r, http.MethodGet, "/api/meta", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"base_url":"","redirect_host":"","redirect_ip":"",
		"favicon_service":true,"allow_registration":true}`, w.Body.String())
}

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clientIPFor(t *testing.T, proxies []string, remoteAddr string, headers map[string]string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NoError(t, applyTrustedProxies(engine, proxies))
	engine.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	return w.Body.String()
}

func TestTrustedProxies_IgnoredWhenNoneConfigured(t *testing.T) {
	for _, proxies := range [][]string{nil, {}} {
		ip := clientIPFor(t, proxies, "10.0.0.9:51234", map[string]string{
			"X-Forwarded-For": "1.2.3.4",
			"X-Real-IP":       "5.6.7.8",
		})
		assert.Equal(t, "10.0.0.9", ip)
	}
}

func TestTrustedProxies_HonouredFromTrustedPeer(t *testing.T) {
	proxies := []string{"10.0.0.0/8", "127.0.0.1"}
	ip := clientIPFor(t, proxies, "10.0.0.9:51234", map[string]string{"X-Forwarded-For": "1.2.3.4"})
	assert.Equal(t, "1.2.3.4", ip)

	// An untrusted peer cannot spoof its address.
	ip = clientIPFor(t, proxies, "192.168.1.5:51234", map[string]string{"X-Forwarded-For": "1.2.3.4"})
	assert.Equal(t, "192.168.1.5", ip)
}

func TestTrustedProxies_InvalidEntryErrors(t *testing.T) {
	assert.Error(t, applyTrustedProxies(gin.New(), []string{"not-an-ip"}))
}

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// MatchOrigin checks if an origin matches any of the allowed patterns.
// Supports exact matches and single-level wildcard subdomains (*.example.com).
func MatchOrigin(origin string, patterns []string) bool {
	if origin == "" {
		return false
	}

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "https://*.") || strings.HasPrefix(pattern, "http://*.") {
			// Wildcard pattern: extract protocol and suffix
			protocolEnd := strings.Index(pattern, "://") + 3
			suffix := pattern[protocolEnd+1:] // e.g., ".example.com" from "https://*.example.com"
			protocol := pattern[:protocolEnd] // e.g., "https://"

			if !strings.HasPrefix(origin, protocol) {
				continue
			}

			if !strings.HasSuffix(origin, suffix) {
				continue
			}

			// Extract subdomain part
			subdomain := origin[len(protocol) : len(origin)-len(suffix)]

			// Subdomain must exist and not contain dots (single level only)
			if len(subdomain) > 0 && !strings.Contains(subdomain, ".") {
				return true
			}
		} else if origin == pattern {
			return true
		}
	}
	return false
}

// CORSMiddleware creates a CORS middleware with configurable origins.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" && MatchOrigin(origin, allowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

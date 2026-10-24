package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMatchOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		patterns []string
		expected bool
	}{
		{
			name:     "exact match",
			origin:   "http://localhost:3000",
			patterns: []string{"http://localhost:3000"},
			expected: true,
		},
		{
			name:     "no match",
			origin:   "http://evil.com",
			patterns: []string{"http://localhost:3000"},
			expected: false,
		},
		{
			name:     "wildcard subdomain match",
			origin:   "https://app.example.com",
			patterns: []string{"https://*.example.com"},
			expected: true,
		},
		{
			name:     "wildcard does not match root domain",
			origin:   "https://example.com",
			patterns: []string{"https://*.example.com"},
			expected: false,
		},
		{
			name:     "wildcard does not match nested subdomain",
			origin:   "https://sub.app.example.com",
			patterns: []string{"https://*.example.com"},
			expected: false,
		},
		{
			name:     "multiple patterns - second matches",
			origin:   "https://api.myapp.com",
			patterns: []string{"http://localhost:3000", "https://*.myapp.com"},
			expected: true,
		},
		{
			name:     "empty origin",
			origin:   "",
			patterns: []string{"http://localhost:3000"},
			expected: false,
		},
		{
			name:     "empty patterns",
			origin:   "http://localhost:3000",
			patterns: []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchOrigin(tt.origin, tt.patterns)
			if result != tt.expected {
				t.Errorf("MatchOrigin(%q, %v) = %v, want %v", tt.origin, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestCORSMiddleware_AllowsPatchAndGroupHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:3000"}))
	r.PATCH("/api/links/bulk", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodOptions, "/api/links/bulk", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	req.Header.Set("Access-Control-Request-Headers", "X-Group-ID")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", w.Code)
	}
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "PATCH") {
		t.Errorf("Access-Control-Allow-Methods = %q, want PATCH", methods)
	}
	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "X-Group-ID") {
		t.Errorf("Access-Control-Allow-Headers = %q, want X-Group-ID", headers)
	}
}

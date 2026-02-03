package middleware

import "testing"

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

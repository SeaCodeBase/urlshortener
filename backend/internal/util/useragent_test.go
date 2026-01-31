package util

import "testing"

func TestParseUserAgent_Chrome(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	result := ParseUserAgent(ua)

	if result.Browser != "Chrome" {
		t.Errorf("expected browser Chrome, got %s", result.Browser)
	}
	if result.DeviceType != "desktop" {
		t.Errorf("expected device desktop, got %s", result.DeviceType)
	}
}

func TestParseUserAgent_Safari_Mobile(t *testing.T) {
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	result := ParseUserAgent(ua)

	if result.Browser != "Safari" {
		t.Errorf("expected browser Safari, got %s", result.Browser)
	}
	if result.DeviceType != "mobile" {
		t.Errorf("expected device mobile, got %s", result.DeviceType)
	}
}

func TestParseUserAgent_Empty(t *testing.T) {
	result := ParseUserAgent("")

	if result.Browser != "Unknown" {
		t.Errorf("expected browser Unknown, got %s", result.Browser)
	}
	if result.DeviceType != "unknown" {
		t.Errorf("expected device unknown, got %s", result.DeviceType)
	}
}

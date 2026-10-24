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

func TestParseUserAgent_iPadIsTablet(t *testing.T) {
	ua := "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	result := ParseUserAgent(ua)

	if result.DeviceType != "tablet" {
		t.Errorf("expected device tablet, got %s", result.DeviceType)
	}
	if result.OS != "iOS" {
		t.Errorf("expected OS iOS, got %s", result.OS)
	}
}

func TestParseUserAgent_OS(t *testing.T) {
	cases := []struct {
		name, ua, os, device string
	}{
		{"iphone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "iOS", "mobile"},
		{"android", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "Android", "mobile"},
		{"android tablet", "Mozilla/5.0 (Linux; Android 10; Tablet) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "Android", "tablet"},
		{"windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Windows", "desktop"},
		{"mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "macOS", "desktop"},
		{"linux", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Linux", "desktop"},
		{"empty", "", "Unknown", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseUserAgent(tc.ua)
			if result.OS != tc.os {
				t.Errorf("expected OS %q, got %q", tc.os, result.OS)
			}
			if result.DeviceType != tc.device {
				t.Errorf("expected device %q, got %q", tc.device, result.DeviceType)
			}
		})
	}
}

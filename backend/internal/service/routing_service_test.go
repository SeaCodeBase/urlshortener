package service

import (
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

func TestRoutingService_Pick_FirstMatchWins(t *testing.T) {
	rs := NewRoutingService()
	rules := []model.RoutingRule{
		{Position: 0, LongURL: "https://desktop.example.com", DeviceMatch: model.StringList{"desktop"}},
		{Position: 1, LongURL: "https://ios.example.com", OSMatch: model.StringList{"ios"}},
		{Position: 2, LongURL: "https://mobile-ios.example.com", DeviceMatch: model.StringList{"mobile"}, OSMatch: model.StringList{"ios"}},
		{Position: 3, LongURL: "https://catch-all.example.com"},
	}

	url, ok := rs.Pick(rules, RoutingRequest{DeviceType: "mobile", OS: "iOS", Country: "US"})
	if !ok || url != "https://ios.example.com" {
		t.Fatalf("expected first matching rule (ios), got %q %v", url, ok)
	}

	url, ok = rs.Pick(rules, RoutingRequest{DeviceType: "mobile", OS: "Android", Country: "US"})
	if !ok || url != "https://catch-all.example.com" {
		t.Fatalf("expected catch-all rule, got %q %v", url, ok)
	}

	countryRules := []model.RoutingRule{
		{LongURL: "https://fr.example.com", CountryMatch: model.StringList{"FR", "BE"}, OSMatch: model.StringList{"android"}},
		{LongURL: "https://be.example.com", CountryMatch: model.StringList{"BE"}},
	}
	url, ok = rs.Pick(countryRules, RoutingRequest{DeviceType: "desktop", OS: "Windows", Country: "BE"})
	if !ok || url != "https://be.example.com" {
		t.Fatalf("expected BE rule (all non-empty lists must match), got %q %v", url, ok)
	}
}

func TestRoutingService_Pick_CountryExclude(t *testing.T) {
	rs := NewRoutingService()
	rules := []model.RoutingRule{
		{LongURL: "https://not-us.example.com", CountryExclude: model.StringList{"US", "CA"}},
	}

	if url, ok := rs.Pick(rules, RoutingRequest{DeviceType: "desktop", OS: "Windows", Country: "US"}); ok {
		t.Fatalf("excluded country must not match, got %q", url)
	}
	url, ok := rs.Pick(rules, RoutingRequest{DeviceType: "desktop", OS: "Windows", Country: "DE"})
	if !ok || url != "https://not-us.example.com" {
		t.Fatalf("expected match for non-excluded country, got %q %v", url, ok)
	}
}

func TestRoutingService_Pick_NoMatchReturnsFalse(t *testing.T) {
	rs := NewRoutingService()
	rules := []model.RoutingRule{
		{LongURL: "https://tablet.example.com", DeviceMatch: model.StringList{"tablet"}},
		{LongURL: "https://jp.example.com", CountryMatch: model.StringList{"JP"}},
	}

	url, ok := rs.Pick(rules, RoutingRequest{DeviceType: "desktop", OS: "macOS", Country: ""})
	if ok || url != "" {
		t.Fatalf("expected no match, got %q %v", url, ok)
	}
	if url, ok := rs.Pick(nil, RoutingRequest{DeviceType: "desktop"}); ok || url != "" {
		t.Fatalf("expected no match for empty rules, got %q %v", url, ok)
	}
}

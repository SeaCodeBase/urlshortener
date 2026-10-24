package util

import (
	"strings"

	"github.com/mssola/useragent"
)

// UAResult is the parsed form of a User-Agent header. DeviceType is one of
// desktop, mobile, tablet or unknown; OS is a normalised name such as "iOS",
// "Android", "Windows", "macOS" or "Linux" ("Unknown" when undetectable).
type UAResult struct {
	Browser    string
	DeviceType string
	OS         string
}

func ParseUserAgent(uaString string) UAResult {
	if uaString == "" {
		return UAResult{Browser: "Unknown", DeviceType: "unknown", OS: "Unknown"}
	}

	ua := useragent.New(uaString)

	browserName, _ := ua.Browser()
	if browserName == "" {
		browserName = "Unknown"
	}

	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
		if strings.Contains(uaString, "iPad") || strings.Contains(uaString, "Tablet") {
			deviceType = "tablet"
		}
	}

	return UAResult{
		Browser:    browserName,
		DeviceType: deviceType,
		OS:         normalizeOS(ua.OSInfo().Name, uaString),
	}
}

// normalizeOS maps the useragent library's OS names to stable display names.
// The library reports iPhones as "iPhone OS" and iPads as "OS".
func normalizeOS(name, uaString string) string {
	switch {
	case name == "":
		return "Unknown"
	case name == "iPhone OS" || strings.Contains(uaString, "iPad") || strings.Contains(uaString, "iPhone") || strings.Contains(uaString, "iPod"):
		return "iOS"
	case strings.HasPrefix(name, "Mac OS"):
		return "macOS"
	}
	return name
}

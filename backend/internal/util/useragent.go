package util

import "github.com/mssola/useragent"

type UAResult struct {
	Browser    string
	DeviceType string
}

func ParseUserAgent(uaString string) UAResult {
	if uaString == "" {
		return UAResult{Browser: "Unknown", DeviceType: "unknown"}
	}

	ua := useragent.New(uaString)

	browserName, _ := ua.Browser()
	if browserName == "" {
		browserName = "Unknown"
	}

	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
	}

	return UAResult{
		Browser:    browserName,
		DeviceType: deviceType,
	}
}

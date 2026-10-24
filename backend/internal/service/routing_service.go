package service

import (
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

// RoutingRequest describes the visitor a link's dynamic routing rules are
// evaluated against. DeviceType is desktop|mobile|tablet|unknown, OS is a
// display name such as "iOS" and Country an ISO 3166-1 alpha-2 code ("" when
// unknown).
type RoutingRequest struct {
	DeviceType string
	OS         string
	Country    string
}

// RoutingService evaluates dynamic routing rules. It is stateless.
type RoutingService struct{}

func NewRoutingService() *RoutingService {
	return &RoutingService{}
}

// Pick returns the LongURL of the first rule (in slice order) whose non-empty
// match lists all contain the request's value and whose CountryExclude does not
// contain the request's country. Comparisons are case-insensitive.
func (s *RoutingService) Pick(rules []model.RoutingRule, req RoutingRequest) (string, bool) {
	for _, r := range rules {
		if !listMatches(r.DeviceMatch, req.DeviceType) ||
			!listMatches(r.OSMatch, req.OS) ||
			!listMatches(r.CountryMatch, req.Country) {
			continue
		}
		if req.Country != "" && containsFold(r.CountryExclude, req.Country) {
			continue
		}
		return r.LongURL, true
	}
	return "", false
}

func listMatches(list model.StringList, value string) bool {
	return len(list) == 0 || containsFold(list, value)
}

func containsFold(list model.StringList, value string) bool {
	for _, v := range list {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

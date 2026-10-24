package model

// RoutingRule redirects a link to LongURL when the visitor matches. Rules are
// evaluated in Position order; empty match lists match everything.
type RoutingRule struct {
	ID             uint64     `db:"id" json:"id"`
	LinkID         uint64     `db:"link_id" json:"link_id"`
	Position       int        `db:"position" json:"position"`
	LongURL        string     `db:"long_url" json:"long_url"`
	DeviceMatch    StringList `db:"device_match" json:"device_match"`
	OSMatch        StringList `db:"os_match" json:"os_match"`
	CountryMatch   StringList `db:"country_match" json:"country_match"`
	CountryExclude StringList `db:"country_exclude" json:"country_exclude"`
}

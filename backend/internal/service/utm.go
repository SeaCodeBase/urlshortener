package service

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL      = errors.New("invalid URL")
	ErrInvalidUTMParam = errors.New("custom URL parameter key must not be empty")
)

// KV is a custom query parameter.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// UTMParams are the UTM fields of the link builder (spec §5.3). Empty
// standard fields are skipped.
type UTMParams struct {
	Source   string `json:"source"`
	Medium   string `json:"medium"`
	Campaign string `json:"campaign"`
	Term     string `json:"term"`
	Content  string `json:"content"`
	Custom   []KV   `json:"custom"`
}

// pairs returns the parameters to set, in order: utm_source, utm_medium,
// utm_campaign, utm_term, utm_content, then custom ones.
func (p UTMParams) pairs() ([]KV, error) {
	out := make([]KV, 0, 5+len(p.Custom))
	for _, kv := range []KV{
		{"utm_source", p.Source},
		{"utm_medium", p.Medium},
		{"utm_campaign", p.Campaign},
		{"utm_term", p.Term},
		{"utm_content", p.Content},
	} {
		if v := strings.TrimSpace(kv.Value); v != "" {
			out = append(out, KV{Key: kv.Key, Value: v})
		}
	}
	for _, kv := range p.Custom {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			return nil, ErrInvalidUTMParam
		}
		out = append(out, KV{Key: k, Value: kv.Value})
	}
	return out, nil
}

// ApplyUTM sets the UTM and custom parameters on rawURL. Existing parameters
// with the same key are overwritten in place (later duplicates dropped);
// new ones are appended. Other parameters keep their order and encoding, and
// the fragment is preserved.
func ApplyUTM(rawURL string, p UTMParams) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", ErrInvalidURL
	}
	set, err := p.pairs()
	if err != nil {
		return "", err
	}
	if len(set) == 0 {
		return u.String(), nil
	}

	// Later entries for the same key win.
	values := make(map[string]string, len(set))
	order := make([]string, 0, len(set))
	for _, kv := range set {
		if _, ok := values[kv.Key]; !ok {
			order = append(order, kv.Key)
		}
		values[kv.Key] = kv.Value
	}
	encode := func(k string) string { return url.QueryEscape(k) + "=" + url.QueryEscape(values[k]) }

	written := make(map[string]bool, len(order))
	var parts []string
	if u.RawQuery != "" {
		for _, part := range strings.Split(u.RawQuery, "&") {
			if part == "" {
				continue
			}
			rawKey, _, _ := strings.Cut(part, "=")
			key, err := url.QueryUnescape(rawKey)
			if err != nil {
				key = rawKey
			}
			if _, ok := values[key]; !ok {
				parts = append(parts, part)
				continue
			}
			if !written[key] {
				parts = append(parts, encode(key))
				written[key] = true
			}
		}
	}
	for _, k := range order {
		if !written[k] {
			parts = append(parts, encode(k))
		}
	}
	u.RawQuery = strings.Join(parts, "&")
	u.ForceQuery = false
	return u.String(), nil
}

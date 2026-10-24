package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	titleFetchTimeout  = 3 * time.Second
	titleFetchMaxBytes = 512 << 10
	titleMaxRunes      = 255
	titleMaxRedirects  = 5
)

// TitleFetcher returns a display title for a destination URL. It never
// fails: on any problem it falls back to the URL's hostname ("" if the URL
// cannot be parsed).
type TitleFetcher func(ctx context.Context, rawURL string) string

// TitleFetcherOptions configures NewTitleFetcher. Zero values select the
// production defaults.
type TitleFetcherOptions struct {
	// Timeout bounds the whole fetch (default 3 s).
	Timeout time.Duration
	// LookupIPAddr resolves hostnames before fetching
	// (default net.DefaultResolver.LookupIPAddr).
	LookupIPAddr func(ctx context.Context, host string) ([]net.IPAddr, error)
	// AllowPrivate disables the private/loopback/link-local address block.
	// Only tests should set it.
	AllowPrivate bool
}

var (
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

	errBlockedAddress = errors.New("destination address is not public")

	// cgnat is the shared address space (RFC 6598), not covered by IsPrivate.
	_, cgnat, _ = net.ParseCIDR("100.64.0.0/10")

	defaultTitleFetcher = NewTitleFetcher(TitleFetcherOptions{})
)

// FetchTitle fetches rawURL and returns its <title> (whitespace-collapsed,
// at most 255 characters), or the hostname when the page is unreachable,
// has no title, or resolves to a non-public address.
func FetchTitle(ctx context.Context, rawURL string) string {
	return defaultTitleFetcher(ctx, rawURL)
}

// isBlockedIP reports whether ip must not be fetched from the server.
func isBlockedIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip)
}

// NewTitleFetcher builds a TitleFetcher with the given options.
func NewTitleFetcher(opts TitleFetcherOptions) TitleFetcher {
	if opts.Timeout <= 0 {
		opts.Timeout = titleFetchTimeout
	}
	if opts.LookupIPAddr == nil {
		opts.LookupIPAddr = net.DefaultResolver.LookupIPAddr
	}

	dialer := &net.Dialer{Timeout: opts.Timeout}
	if !opts.AllowPrivate {
		// Check the address actually dialled, so redirects and DNS
		// rebinding cannot reach internal hosts.
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || isBlockedIP(ip) {
				return errBlockedAddress
			}
			return nil
		}
	}
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   opts.Timeout,
			ResponseHeaderTimeout: opts.Timeout,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= titleMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", titleMaxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("unsupported redirect scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}

	return func(ctx context.Context, rawURL string) string {
		u, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil {
			return ""
		}
		host := u.Hostname()
		if host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return host
		}

		ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
		defer cancel()

		if !opts.AllowPrivate {
			if ip := net.ParseIP(host); ip != nil {
				if isBlockedIP(ip) {
					return host
				}
			} else {
				addrs, err := opts.LookupIPAddr(ctx, host)
				if err != nil || len(addrs) == 0 {
					return host
				}
				for _, a := range addrs {
					if isBlockedIP(a.IP) {
						return host
					}
				}
			}
		}

		if title := fetchTitle(ctx, client, u.String()); title != "" {
			return title
		}
		return host
	}
}

func fetchTitle(ctx context.Context, client *http.Client, target string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; urlshortener-title-fetcher/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, titleFetchMaxBytes))
	if err != nil && len(body) == 0 {
		return ""
	}
	m := titleRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return cleanTitle(string(m[1]))
}

// cleanTitle unescapes entities, collapses whitespace and truncates to
// titleMaxRunes characters.
func cleanTitle(s string) string {
	s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
	if r := []rune(s); len(r) > titleMaxRunes {
		s = strings.TrimSpace(string(r[:titleMaxRunes]))
	}
	return s
}

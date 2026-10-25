package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

// webhookURLMsg is the ValidationError message for any rejected webhook URL.
const webhookURLMsg = "URL must be a public http(s) address"

// webhookDeactivateAfter is how long a webhook may keep failing before it is
// deactivated.
const webhookDeactivateAfter = 24 * time.Hour

// webhookAlertFailures is the consecutive-failure count that raises an alert.
const webhookAlertFailures = 2

// ValidateWebhookURL accepts http(s) URLs with a host, no userinfo, whose
// host (an IP literal, or every address it resolves to, at least one) is a
// public unicast address. Any rejection is *ValidationError.
func ValidateWebhookURL(ctx context.Context, raw string, r DNSResolver) error {
	bad := &ValidationError{Msg: webhookURLMsg}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return bad
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return bad
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return bad
		}
		return nil
	}
	addrs, err := r.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return bad
	}
	for _, a := range addrs {
		if !isPublicIP(a.IP) {
			return bad
		}
	}
	return nil
}

// deniedPrefixes are the non-public ranges a webhook may never target:
// this-network, private, CGNAT, loopback, link-local, IETF protocol
// assignments, benchmarking, multicast, reserved, broadcast, and their IPv6
// counterparts (unspecified, loopback, ULA, link-local, multicast).
var deniedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("fec0::/10"),      // deprecated site-local
	netip.MustParsePrefix("64:ff9b:1::/48"), // RFC 8215 local-use NAT64
	netip.MustParsePrefix("ff00::/8"),
}

// IPv6 ranges that embed an IPv4 address, which must itself be public.
var (
	nat64Prefix    = netip.MustParsePrefix("64:ff9b::/96") // RFC 6052: last 32 bits
	sixToFourPrefx = netip.MustParsePrefix("2002::/16")    // RFC 3056: bits 16-47
	v4CompatPrefix = netip.MustParsePrefix("::/96")        // deprecated IPv4-compatible
	// RFC 2765 IPv4-translated ::ffff:0:a.b.c.d (IPv4-mapped ::ffff:a.b.c.d
	// is handled by Unmap).
	v4TranslatedPrefix = netip.MustParsePrefix("::ffff:0:0:0/96")
)

// isPublicIP reports whether ip is a public unicast address: outside every
// deniedPrefixes range, with IPv4-mapped, IPv4-compatible, NAT64 and 6to4
// addresses judged by the IPv4 address they embed.
func isPublicIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	return isPublicAddr(a)
}

func isPublicAddr(a netip.Addr) bool {
	if !a.IsValid() {
		return false
	}
	a = a.WithZone("").Unmap() // ::ffff:0:0/96
	for _, p := range deniedPrefixes {
		if p.Contains(a) {
			return false
		}
	}
	if a.Is4() {
		return true
	}
	b := a.As16()
	switch {
	case nat64Prefix.Contains(a), v4CompatPrefix.Contains(a), v4TranslatedPrefix.Contains(a):
		return isPublicAddr(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	case sixToFourPrefx.Contains(a):
		return isPublicAddr(netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}))
	}
	return true
}

// WebhookURLHost returns scheme://host[:port] of a webhook URL, for logs and
// activity meta (the full URL may carry a token in its path or query).
func WebhookURLHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// WebhookErrorMessage renders a delivery error for logs and the stored
// delivery row (at most 500 characters): a *url.Error is reduced to its
// operation, the URL's scheme+host and the cause, so the full URL never
// leaks.
func WebhookErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		msg := ue.Op + " " + WebhookURLHost(ue.URL) + ": " + ue.Err.Error()
		return truncateWebhookError(msg)
	}
	return truncateWebhookError(err.Error())
}

// SignBody is the lowercase hex HMAC-SHA256 of body keyed by secret (the
// X-Signature header of every webhook request).
func SignBody(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// ApplyDeliveryResult updates w's status fields after a delivery attempt at
// now (pure; the caller persists w). Success resets to active; a failure
// counts up, remembers when failing started, alerts from the second failure
// on and deactivates once failing for longer than 24 h.
func ApplyDeliveryResult(w *model.Webhook, ok bool, now time.Time) {
	w.LastDeliveryAt.Time, w.LastDeliveryAt.Valid = now, true
	if ok {
		w.Status = model.WebhookStatusActive
		w.ConsecutiveFailures = 0
		w.FailingSince = model.NullTime{}
		return
	}
	w.ConsecutiveFailures++
	if !w.FailingSince.Valid {
		w.FailingSince.Time, w.FailingSince.Valid = now, true
	}
	if w.ConsecutiveFailures >= webhookAlertFailures {
		w.Status = model.WebhookStatusAlert
	}
	if now.Sub(w.FailingSince.Time) > webhookDeactivateAfter {
		w.Status = model.WebhookStatusDeactivated
		w.IsActive = false
	}
}

// errNonPublicAddress is returned by the webhook client's dialer.
var errNonPublicAddress = errors.New("webhook: refusing to connect to a non-public address")

// NewWebhookHTTPClient is the client for outbound webhook requests: 10 s
// timeout, at most 3 redirects, and a dialer that refuses non-public IPs
// (so DNS rebinding or a redirect cannot reach internal hosts).
func NewWebhookHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if !isPublicIP(net.ParseIP(host)) {
				return fmt.Errorf("%w: %s", errNonPublicAddress, host)
			}
			return nil
		},
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			// via holds the requests made so far: the 4th redirect is refused.
			if len(via) > 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

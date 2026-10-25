package service_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIPResolver answers LookupIPAddr from a map; other names fail.
type fakeIPResolver struct {
	ips  map[string][]string
	err  error
	seen []string
}

func (f *fakeIPResolver) LookupTXT(context.Context, string) ([]string, error) { return nil, nil }
func (f *fakeIPResolver) LookupCNAME(context.Context, string) (string, error) { return "", nil }
func (f *fakeIPResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	f.seen = append(f.seen, host)
	if f.err != nil {
		return nil, f.err
	}
	v, ok := f.ips[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	out := make([]net.IPAddr, 0, len(v))
	for _, s := range v {
		out = append(out, net.IPAddr{IP: net.ParseIP(s)})
	}
	return out, nil
}

func TestValidateWebhookURL(t *testing.T) {
	r := &fakeIPResolver{ips: map[string][]string{
		"hooks.example.test": {"93.184.216.34"},
		"mixed.example.test": {"93.184.216.34", "10.1.2.3"},
		"empty.example.test": {},
	}}
	cases := []struct {
		raw string
		ok  bool
	}{
		{"ftp://x", false},
		{"http://user:pw@x", false},
		{"http://localhost/h", false},
		{"http://10.0.0.5/h", false},
		{"http://169.254.1.1", false},
		{"http://127.0.0.1:8080/h", false},
		{"http://[::1]/h", false},
		{"http://0.0.0.0/h", false},
		{"http://224.0.0.1/h", false},
		{"http:///nohost", false},
		{"not a url", false},
		{"http://mixed.example.test/x", false},
		{"http://empty.example.test/x", false},
		{"https://hooks.example.test/x", true},
		{"http://93.184.216.34/x", true},
	}
	for _, c := range cases {
		err := service.ValidateWebhookURL(context.Background(), c.raw, r)
		if c.ok {
			assert.NoError(t, err, c.raw)
			continue
		}
		var ve *service.ValidationError
		if assert.True(t, errors.As(err, &ve), "%s: want ValidationError, got %v", c.raw, err) {
			assert.Equal(t, "URL must be a public http(s) address", ve.Msg)
		}
	}

	t.Run("resolver error", func(t *testing.T) {
		re := &fakeIPResolver{err: errors.New("dns down")}
		err := service.ValidateWebhookURL(context.Background(), "https://hooks.example.test/x", re)
		var ve *service.ValidationError
		assert.True(t, errors.As(err, &ve))
	})
}

func TestSignBody(t *testing.T) {
	// printf '%s' '{"a":1}' | openssl dgst -sha256 -hmac s3cret
	assert.Equal(t, "5910e62016ef5034272c926c27071992a465c2335cecf41851bda071577f4f6d",
		service.SignBody("s3cret", []byte(`{"a":1}`)))
}

func TestApplyDeliveryResult(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	nt := func(t time.Time) model.NullTime {
		var n model.NullTime
		n.Time, n.Valid = t, true
		return n
	}

	t.Run("ok resets", func(t *testing.T) {
		w := &model.Webhook{IsActive: true, Status: model.WebhookStatusAlert, ConsecutiveFailures: 3, FailingSince: nt(now.Add(-time.Hour))}
		service.ApplyDeliveryResult(w, true, now)
		assert.Equal(t, model.WebhookStatusActive, w.Status)
		assert.Equal(t, 0, w.ConsecutiveFailures)
		assert.False(t, w.FailingSince.Valid)
		assert.True(t, w.LastDeliveryAt.Valid)
		assert.True(t, w.LastDeliveryAt.Time.Equal(now))
		assert.True(t, w.IsActive)
	})

	t.Run("two failures alert", func(t *testing.T) {
		w := &model.Webhook{IsActive: true, Status: model.WebhookStatusActive}
		service.ApplyDeliveryResult(w, false, now)
		assert.Equal(t, model.WebhookStatusActive, w.Status)
		assert.Equal(t, 1, w.ConsecutiveFailures)
		require.True(t, w.FailingSince.Valid)
		assert.True(t, w.FailingSince.Time.Equal(now))

		later := now.Add(time.Minute)
		service.ApplyDeliveryResult(w, false, later)
		assert.Equal(t, model.WebhookStatusAlert, w.Status)
		assert.Equal(t, 2, w.ConsecutiveFailures)
		assert.True(t, w.FailingSince.Time.Equal(now), "failing_since keeps the first failure")
		assert.True(t, w.LastDeliveryAt.Time.Equal(later))
		assert.True(t, w.IsActive)
	})

	t.Run("failing for over 24h deactivates", func(t *testing.T) {
		w := &model.Webhook{IsActive: true, Status: model.WebhookStatusAlert, ConsecutiveFailures: 4, FailingSince: nt(now.Add(-25 * time.Hour))}
		service.ApplyDeliveryResult(w, false, now)
		assert.Equal(t, model.WebhookStatusDeactivated, w.Status)
		assert.False(t, w.IsActive)
		assert.Equal(t, 5, w.ConsecutiveFailures)
	})

	t.Run("success after alert", func(t *testing.T) {
		w := &model.Webhook{IsActive: true, Status: model.WebhookStatusAlert, ConsecutiveFailures: 2, FailingSince: nt(now.Add(-time.Hour))}
		service.ApplyDeliveryResult(w, true, now)
		assert.Equal(t, model.WebhookStatusActive, w.Status)
		assert.Equal(t, 0, w.ConsecutiveFailures)
		assert.False(t, w.FailingSince.Valid)
	})
}

func TestNewWebhookHTTPClient_RefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	resp, err := service.NewWebhookHTTPClient().Get(srv.URL)
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-public address")
}

// TestValidateWebhookURL_DenyList covers every non-public range, as an IP
// literal and as a resolved address.
func TestValidateWebhookURL_DenyList(t *testing.T) {
	denied := []string{
		"0.1.2.3", "10.9.8.7", "100.64.0.1", "100.127.255.254", "127.0.0.2", "169.254.169.254",
		"172.16.0.1", "172.31.255.255", "192.0.0.8", "192.168.1.1", "198.18.0.1", "198.19.255.255",
		"224.0.0.251", "239.1.2.3", "240.0.0.1", "255.255.255.255",
		"::", "::1", "fd00::1", "fc00::1", "fe80::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254",
		"64:ff9b::7f00:1", "64:ff9b::a00:1", "2002:7f00:1::", "2002:a9fe:a9fe::1", "::7f00:1", "::a00:1",
	}
	for _, ip := range denied {
		r := &fakeIPResolver{ips: map[string][]string{"h.example.test": {ip}}}
		for _, raw := range []string{"http://" + literalHost(ip) + "/x", "https://h.example.test/x"} {
			err := service.ValidateWebhookURL(context.Background(), raw, r)
			var ve *service.ValidationError
			assert.True(t, errors.As(err, &ve), "%s (%s) must be rejected, got %v", raw, ip, err)
		}
	}
	allowed := []string{"93.184.216.34", "100.63.255.255", "100.128.0.1", "172.32.0.1", "198.20.0.1",
		"2606:4700::1111", "::ffff:93.184.216.34", "64:ff9b::5db8:d822", "2002:5db8:d822::1"}
	for _, ip := range allowed {
		err := service.ValidateWebhookURL(context.Background(), "http://"+literalHost(ip)+"/x", nil)
		assert.NoError(t, err, ip)
	}
}

func literalHost(ip string) string {
	if strings.Contains(ip, ":") {
		return "[" + ip + "]"
	}
	return ip
}

func TestNewWebhookHTTPClient_Limits(t *testing.T) {
	c := service.NewWebhookHTTPClient()
	assert.Equal(t, 10*time.Second, c.Timeout)

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		http.Redirect(rw, r, fmt.Sprintf("/r/%d", n), http.StatusFound)
	}))
	defer srv.Close()

	// Reuse the redirect policy with a transport that may reach 127.0.0.1.
	c.Transport = http.DefaultTransport
	resp, err := c.Get(srv.URL + "/start")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusFound, resp.StatusCode, "the 4th redirect is returned, not followed")
	assert.Equal(t, int32(4), hits.Load(), "original request + 3 followed redirects")
}

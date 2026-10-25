package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

// Compile-time check: WebhookServiceImpl implements WebhookService
var _ WebhookService = (*WebhookServiceImpl)(nil)

const (
	// WebhookEventEngagement is the only webhook event type.
	WebhookEventEngagement = "engagement"

	webhookNameMax       = 120
	webhookURLMax        = 2048
	webhookDeliveryLimit = 20
	// webhookResponseMax is how much of a response body is drained.
	webhookResponseMax = 64 << 10
)

// WebhookInput is the body of POST/PUT /webhooks. Event "" means engagement.
type WebhookInput struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Event string `json:"event"`
}

type WebhookServiceDeps struct {
	Webhooks repository.WebhookRepository
	Activity ActivityService
	// Client sends verify pings; nil means NewWebhookHTTPClient().
	Client   *http.Client
	Resolver DNSResolver
}

type WebhookServiceImpl struct {
	webhooks repository.WebhookRepository
	activity ActivityService
	client   *http.Client
	resolver DNSResolver
	now      func() time.Time
}

func NewWebhookService(d WebhookServiceDeps) *WebhookServiceImpl {
	client := d.Client
	if client == nil {
		client = NewWebhookHTTPClient()
	}
	resolver := d.Resolver
	if resolver == nil {
		resolver = NewNetResolver()
	}
	return &WebhookServiceImpl{
		webhooks: d.Webhooks,
		activity: d.Activity,
		client:   client,
		resolver: resolver,
		now:      time.Now,
	}
}

// SetClock overrides the time source (tests).
func (s *WebhookServiceImpl) SetClock(now func() time.Time) { s.now = now }

// validate trims in and checks name, event and URL.
func (s *WebhookServiceImpl) validate(ctx context.Context, in *WebhookInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.URL = strings.TrimSpace(in.URL)
	in.Event = strings.TrimSpace(in.Event)
	if in.Event == "" {
		in.Event = WebhookEventEngagement
	}
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > webhookNameMax {
		return invalid("name must be 1-%d characters", webhookNameMax)
	}
	if in.Event != WebhookEventEngagement {
		return invalid("event must be %q", WebhookEventEngagement)
	}
	if len(in.URL) > webhookURLMax {
		return invalid("URL must be at most %d characters", webhookURLMax)
	}
	return ValidateWebhookURL(ctx, in.URL, s.resolver)
}

// load returns the webhook when it belongs to groupID, else ErrWebhookNotFound.
func (s *WebhookServiceImpl) load(ctx context.Context, groupID, id uint64) (*model.Webhook, error) {
	w, err := s.webhooks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w.GroupID != groupID {
		return nil, repository.ErrWebhookNotFound
	}
	return w, nil
}

func newWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *WebhookServiceImpl) Create(ctx context.Context, userID, groupID uint64, in WebhookInput) (*model.Webhook, error) {
	if err := s.validate(ctx, &in); err != nil {
		return nil, err
	}
	secret, err := newWebhookSecret()
	if err != nil {
		return nil, err
	}
	w := &model.Webhook{
		UserID: userID, GroupID: groupID, Name: in.Name, URL: in.URL, Event: in.Event,
		Secret: secret, IsActive: true, Status: model.WebhookStatusActive,
	}
	if err := s.webhooks.Create(ctx, w); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionWebhookCreate, EntityWebhook, w.ID,
		map[string]any{"name": w.Name, "url_host": WebhookURLHost(w.URL)})
	return w, nil
}

func (s *WebhookServiceImpl) Get(ctx context.Context, groupID, id uint64) (*model.Webhook, error) {
	return s.load(ctx, groupID, id)
}

func (s *WebhookServiceImpl) List(ctx context.Context, groupID uint64) ([]model.Webhook, error) {
	ws, err := s.webhooks.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if ws == nil {
		ws = []model.Webhook{}
	}
	return ws, nil
}

func (s *WebhookServiceImpl) Update(ctx context.Context, userID, groupID, id uint64, in WebhookInput) (*model.Webhook, error) {
	w, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	if err := s.validate(ctx, &in); err != nil {
		return nil, err
	}
	w.Name, w.URL = in.Name, in.URL
	if err := s.webhooks.Update(ctx, w); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionWebhookUpdate, EntityWebhook, w.ID,
		map[string]any{"name": w.Name, "url_host": WebhookURLHost(w.URL)})
	return w, nil
}

func (s *WebhookServiceImpl) Delete(ctx context.Context, userID, groupID, id uint64) error {
	w, err := s.load(ctx, groupID, id)
	if err != nil {
		return err
	}
	if err := s.webhooks.Delete(ctx, w.ID); err != nil {
		return err
	}
	s.activity.Log(ctx, userID, groupID, ActionWebhookDelete, EntityWebhook, w.ID, map[string]any{"name": w.Name})
	return nil
}

func (s *WebhookServiceImpl) Deactivate(ctx context.Context, userID, groupID, id uint64) (*model.Webhook, error) {
	w, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	w.IsActive = false
	w.Status = model.WebhookStatusDeactivated
	if err := s.webhooks.Update(ctx, w); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionWebhookDeactivate, EntityWebhook, w.ID, map[string]any{"name": w.Name})
	return w, nil
}

func (s *WebhookServiceImpl) Activate(ctx context.Context, userID, groupID, id uint64) (*model.Webhook, error) {
	w, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	w.IsActive = true
	w.Status = model.WebhookStatusActive
	w.ConsecutiveFailures = 0
	w.FailingSince = model.NullTime{}
	if err := s.webhooks.Update(ctx, w); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionWebhookUpdate, EntityWebhook, w.ID, map[string]any{"activated": true})
	return w, nil
}

// webhookPing is the Verify request body.
type webhookPing struct {
	EventType string `json:"event_type"`
	WebhookID uint64 `json:"webhook_id"`
	Datetime  string `json:"datetime"`
}

// Verify pings the stored URL. The URL is not re-validated here (it was on
// Create/Update); the production client's dialer refuses non-public IPs.
func (s *WebhookServiceImpl) Verify(ctx context.Context, userID, groupID, id uint64) (*model.WebhookDelivery, error) {
	w, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	body, err := json.Marshal(webhookPing{EventType: "ping", WebhookID: w.ID, Datetime: now.Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	eventID := "ping-" + strconv.FormatInt(now.Unix(), 10)
	// The ping and its bookkeeping outlive a cancelled request (the client
	// timeout bounds the call) so the delivery and status are always recorded.
	ctx = context.WithoutCancel(ctx)
	status, sendErr := s.send(ctx, w, eventID, body)
	ok := sendErr == nil && status >= 200 && status < 300

	d := &model.WebhookDelivery{WebhookID: w.ID, EventID: eventID, StatusCode: status, Attempt: 1, DeliveredAt: now}
	if sendErr != nil {
		d.Error = WebhookErrorMessage(sendErr)
	}
	if err := s.webhooks.AddDelivery(ctx, d); err != nil {
		return nil, err
	}

	// Column-scoped and applied to the locked current row, so a concurrent
	// edit of name/URL survives. A deactivated webhook stays deactivated
	// until Activate (the repository also guarantees this).
	if _, err := s.webhooks.UpdateDeliveryState(ctx, w.ID, func(cur *model.Webhook) {
		wasDeactivated := !cur.IsActive || cur.Status == model.WebhookStatusDeactivated
		ApplyDeliveryResult(cur, ok, now)
		if wasDeactivated {
			cur.IsActive = false
			cur.Status = model.WebhookStatusDeactivated
		}
	}); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionWebhookVerify, EntityWebhook, w.ID, map[string]any{"status_code": status})
	return d, nil
}

// send POSTs body to w.URL with the signature headers and returns the
// response status (0 when no response was received).
func (s *WebhookServiceImpl) send(ctx context.Context, w *model.Webhook, eventID string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", SignBody(w.Secret, body))
	req.Header.Set("X-Webhook-Id", strconv.FormatUint(w.ID, 10))
	req.Header.Set("X-Event-Id", eventID)
	resp, err := s.client.Do(req)
	if err != nil {
		logger.Info(ctx, "webhook-service: ping failed", zap.Uint64("webhook_id", w.ID),
			zap.String("url_host", WebhookURLHost(w.URL)), zap.String("error", WebhookErrorMessage(err)))
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, webhookResponseMax))
	return resp.StatusCode, nil
}

func truncateWebhookError(s string) string {
	const max = 500
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

func (s *WebhookServiceImpl) Deliveries(ctx context.Context, groupID, id uint64) ([]model.WebhookDelivery, error) {
	w, err := s.load(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	ds, err := s.webhooks.ListDeliveries(ctx, w.ID, webhookDeliveryLimit)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		ds = []model.WebhookDelivery{}
	}
	return ds, nil
}

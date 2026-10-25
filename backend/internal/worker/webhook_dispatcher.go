package worker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// WebhookQueueKey is the Redis ZSET of pending webhook deliveries: score =
// unix seconds not-before, member = JSON WebhookJob.
const WebhookQueueKey = "webhooks:queue"

const (
	// webhookMaxAttempts is the last attempt; a failure there drops the job.
	webhookMaxAttempts = 5
	// webhookPollLimit caps the due jobs claimed per poll (and the page size
	// when reading the queue); webhookPollPages caps the pages read per poll.
	webhookPollLimit = 100
	webhookPollPages = 10
	// webhookMaxInFlight caps concurrent deliveries across all webhooks.
	webhookMaxInFlight = 32
	// webhookHookInFlight caps one webhook's concurrent deliveries (tracked
	// across polls), so a slow endpoint cannot take every slot;
	// webhookAlertInFlight is the cap (one probe) while it is in alert.
	webhookHookInFlight  = 2
	webhookAlertInFlight = 1
	// webhookSlotWait is how long the claim loop waits for a webhook at its
	// cap to free a slot before treating it as saturated for the rest of the
	// poll (a fast endpoint frees one within milliseconds);
	// webhookSlotWaitBudget bounds the timed-out waits of one poll.
	webhookSlotWait       = 200 * time.Millisecond
	webhookSlotWaitBudget = time.Second
	// webhookDeferDelay is how far a job for a saturated webhook is pushed
	// back (same attempt: it was not tried). Alert webhooks' extra jobs wait
	// webhookBackoff[1] instead.
	webhookDeferDelay = 5 * time.Second
	// webhookDefaultTimeout bounds a delivery when the client has no timeout.
	webhookDefaultTimeout = 10 * time.Second
	// webhookResponseMax is how much of a response body is drained.
	webhookResponseMax = 64 << 10
	webhookUserAgent   = "urlshortener-webhooks/1.0"
	webhookEventType   = "engagement"
)

// webhookBackoff is the delay before attempt n+1 after attempt n failed.
var webhookBackoff = map[int]time.Duration{
	1: 60 * time.Second,
	2: 5 * time.Minute,
	3: 30 * time.Minute,
	4: 2 * time.Hour,
}

// WebhookJob is one queued delivery of Payload to a webhook.
type WebhookJob struct {
	WebhookID uint64          `json:"webhook_id"`
	EventID   string          `json:"event_id"`
	Attempt   int             `json:"attempt"`
	Payload   json.RawMessage `json:"payload"`
}

// EngagementPayload is the body of an engagement event (one click).
type EngagementPayload struct {
	EventID    string   `json:"event_id"`
	EventType  string   `json:"event_type"`
	Datetime   string   `json:"datetime"`
	Bitlink    string   `json:"bitlink"`
	LongURL    string   `json:"long_url"`
	Referrer   string   `json:"referrer"`
	Country    string   `json:"country"`
	DeviceType string   `json:"device_type"`
	OS         string   `json:"os"`
	Source     string   `json:"source"`
	Tags       []string `json:"tags"`
	GroupID    uint64   `json:"group_id"`
}

// ClickEnqueuer receives every batch of clicks the flusher stored.
type ClickEnqueuer interface {
	Enqueue(ctx context.Context, clicks []model.Click)
}

// webhookActiveCacheTTL is how long a group's active-webhook list is reused
// by the enqueuer: a flush with no listeners costs at most one query per
// group per TTL, and a webhook created, edited or deactivated takes effect
// for new events within this delay (delivery re-checks the webhook anyway).
const webhookActiveCacheTTL = 5 * time.Second

type activeHooksEntry struct {
	hooks     []model.Webhook
	fetchedAt time.Time
}

// WebhookEnqueuer turns stored clicks into queued engagement deliveries for
// the active webhooks of each link's group.
type WebhookEnqueuer struct {
	rdb      *redis.Client
	webhooks repository.WebhookRepository
	links    repository.LinkRepository
	tags     repository.TagRepository
	domains  repository.DomainRepository
	baseURL  string
	now      func() time.Time

	cacheMu sync.Mutex
	active  map[uint64]activeHooksEntry // groupID -> active webhooks
}

var _ ClickEnqueuer = (*WebhookEnqueuer)(nil)

func NewWebhookEnqueuer(rdb *redis.Client, webhooks repository.WebhookRepository, links repository.LinkRepository,
	tags repository.TagRepository, domains repository.DomainRepository, baseURL string) *WebhookEnqueuer {
	return &WebhookEnqueuer{rdb: rdb, webhooks: webhooks, links: links, tags: tags, domains: domains,
		baseURL: baseURL, now: time.Now, active: map[uint64]activeHooksEntry{}}
}

// activeHooks returns the group's active webhooks, from the cache when the
// entry is younger than webhookActiveCacheTTL. Errors are not cached.
func (e *WebhookEnqueuer) activeHooks(ctx context.Context, groupID uint64) ([]model.Webhook, error) {
	now := e.now()
	e.cacheMu.Lock()
	ent, ok := e.active[groupID]
	e.cacheMu.Unlock()
	if ok && now.Sub(ent.fetchedAt) < webhookActiveCacheTTL && !now.Before(ent.fetchedAt) {
		return ent.hooks, nil
	}
	hooks, err := e.webhooks.ListActiveByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	e.cacheMu.Lock()
	for id, old := range e.active { // drop expired entries so the map stays small
		if now.Sub(old.fetchedAt) >= webhookActiveCacheTTL {
			delete(e.active, id)
		}
	}
	e.active[groupID] = activeHooksEntry{hooks: hooks, fetchedAt: now}
	e.cacheMu.Unlock()
	return hooks, nil
}

// Enqueue queues one job per click per active webhook of the click's link's
// group (attempt 1, due now) with a single ZADD. Lookups are batched: links
// once, active webhooks once per group (cached for webhookActiveCacheTTL),
// and tags and domains only for links
// that have a listener. Errors are logged, never returned: the click flush
// must not fail because of webhooks.
func (e *WebhookEnqueuer) Enqueue(ctx context.Context, clicks []model.Click) {
	if len(clicks) == 0 {
		return
	}
	ids := make([]uint64, 0)
	seen := map[uint64]bool{}
	for _, c := range clicks {
		if !seen[c.LinkID] {
			seen[c.LinkID] = true
			ids = append(ids, c.LinkID)
		}
	}
	links, err := e.links.GetByIDs(ctx, ids)
	if err != nil {
		logger.Error(ctx, "webhook-enqueuer: failed to load links", zap.Int("links", len(ids)), zap.Error(err))
		return
	}
	linkByID := make(map[uint64]*model.Link, len(links))
	hooksByGroup := map[uint64][]model.Webhook{}
	var hooked []uint64
	domainIDs := map[uint64]bool{}
	for i := range links {
		l := &links[i]
		hooks, ok := hooksByGroup[l.GroupID]
		if !ok {
			hooks, err = e.activeHooks(ctx, l.GroupID)
			if err != nil {
				logger.Error(ctx, "webhook-enqueuer: failed to load webhooks", zap.Uint64("group_id", l.GroupID), zap.Error(err))
				hooks = nil
			}
			hooksByGroup[l.GroupID] = hooks
		}
		if len(hooks) == 0 {
			continue
		}
		linkByID[l.ID] = l
		hooked = append(hooked, l.ID)
		if l.DomainID != nil {
			domainIDs[*l.DomainID] = true
		}
	}
	if len(hooked) == 0 {
		return
	}

	tags, err := e.tags.GetTagsForLinks(ctx, hooked)
	if err != nil {
		logger.Warn(ctx, "webhook-enqueuer: failed to load tags; sending none", zap.Error(err))
		tags = nil
	}
	domainNames := make(map[uint64]string, len(domainIDs))
	for id := range domainIDs {
		d, err := e.domains.GetByID(ctx, id)
		if err != nil {
			logger.Warn(ctx, "webhook-enqueuer: failed to load domain; using base URL",
				zap.Uint64("domain_id", id), zap.Error(err))
			continue
		}
		domainNames[id] = d.Domain
	}

	now := e.now()
	members := make([]redis.Z, 0)
	for _, c := range clicks {
		l := linkByID[c.LinkID]
		if l == nil {
			continue
		}
		domainName := ""
		if l.DomainID != nil {
			domainName = domainNames[*l.DomainID]
		}
		linkTags := tags[l.ID]
		if linkTags == nil {
			linkTags = []string{}
		}
		for _, w := range hooksByGroup[l.GroupID] {
			eventID, err := newEventID()
			if err != nil {
				logger.Error(ctx, "webhook-enqueuer: failed to generate event id", zap.Error(err))
				return
			}
			payload, err := json.Marshal(EngagementPayload{
				EventID: eventID, EventType: webhookEventType,
				Datetime: c.ClickedAt.UTC().Format(time.RFC3339),
				Bitlink:  service.ShortURL(e.baseURL, l, domainName), LongURL: l.OriginalURL,
				Referrer: c.Referrer, Country: c.Country, DeviceType: c.DeviceType, OS: c.OS, Source: c.Source,
				Tags: linkTags, GroupID: l.GroupID,
			})
			if err != nil {
				logger.Error(ctx, "webhook-enqueuer: failed to encode payload", zap.Error(err))
				continue
			}
			job, err := json.Marshal(WebhookJob{WebhookID: w.ID, EventID: eventID, Attempt: 1, Payload: payload})
			if err != nil {
				logger.Error(ctx, "webhook-enqueuer: failed to encode job", zap.Error(err))
				continue
			}
			members = append(members, redis.Z{Score: float64(now.Unix()), Member: string(job)})
		}
	}
	if len(members) == 0 {
		return
	}
	if err := e.rdb.ZAdd(ctx, WebhookQueueKey, members...).Err(); err != nil {
		logger.Error(ctx, "webhook-enqueuer: failed to queue jobs", zap.Int("jobs", len(members)), zap.Error(err))
		return
	}
	logger.Info(ctx, "webhook-enqueuer: queued engagement events", zap.Int("jobs", len(members)))
}

func newEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// WebhookDispatcher delivers due jobs from WebhookQueueKey: it signs and
// POSTs each payload, records the delivery, updates the webhook's status
// and requeues failures with backoff (at most webhookMaxAttempts attempts).
// Deliveries run in goroutines that outlive the poll that claimed them.
type WebhookDispatcher struct {
	rdb       *redis.Client
	webhooks  repository.WebhookRepository
	client    *http.Client
	pollEvery time.Duration
	now       func() time.Time
	stopCh    chan struct{}
	doneCh    chan struct{}

	sem        chan struct{}  // global in-flight bound (webhookMaxInFlight)
	deliveries sync.WaitGroup // in-flight deliveries, waited for by Stop
	mu         sync.Mutex
	inFlight   map[uint64]int // webhook id -> deliveries in flight
	released   chan struct{}  // signalled (non-blocking) when a slot frees
}

func NewWebhookDispatcher(rdb *redis.Client, webhooks repository.WebhookRepository, client *http.Client,
	pollEvery time.Duration) *WebhookDispatcher {
	if pollEvery <= 0 {
		pollEvery = 2 * time.Second
	}
	return &WebhookDispatcher{
		rdb: rdb, webhooks: webhooks, client: client, pollEvery: pollEvery, now: time.Now,
		stopCh: make(chan struct{}), doneCh: make(chan struct{}),
		sem: make(chan struct{}, webhookMaxInFlight), inFlight: map[uint64]int{}, released: make(chan struct{}, 1),
	}
}

func (d *WebhookDispatcher) Start() {
	go d.run()
}

// Stop ends the loop and waits until every delivery in flight has been
// recorded; jobs not yet claimed stay queued for the next start.
func (d *WebhookDispatcher) Stop() {
	close(d.stopCh)
	<-d.doneCh
}

func (d *WebhookDispatcher) run() {
	defer close(d.doneCh)
	defer d.deliveries.Wait()
	ticker := time.NewTicker(d.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// A full poll means more is due: poll again without waiting
			// for the next tick.
			for d.poll() >= webhookPollLimit && !d.stopping() {
			}
		case <-d.stopCh:
			return
		}
	}
}

func (d *WebhookDispatcher) stopping() bool {
	select {
	case <-d.stopCh:
		return true
	default:
		return false
	}
}

// waitDeliveries blocks until every started delivery has been recorded.
func (d *WebhookDispatcher) waitDeliveries() { d.deliveries.Wait() }

// hookState is what one poll knows about a webhook.
type hookState struct {
	drop      bool // missing, inactive or deactivated: its jobs are removed
	skip      bool // could not be loaded: its jobs stay queued
	alert     bool
	saturated bool // at its cap and not freeing slots: defer its jobs
}

// poll claims up to webhookPollLimit due jobs, starts their deliveries and
// returns how many it started, without waiting for them. A job is claimed
// by ZREM; when another instance removed it first it is skipped. Each
// webhook has at most webhookHookInFlight deliveries in flight (one while in
// alert), counted across polls. When a webhook is at its cap the poll waits
// up to webhookSlotWait for a slot; a webhook that does not free one (or
// any alert webhook at its cap) is saturated, and its remaining due jobs are
// rescheduled in place with the same attempt. Jobs for missing, inactive or
// deactivated webhooks are dropped. Up to webhookPollPages pages are read.
func (d *WebhookDispatcher) poll() int {
	ctx := context.Background()
	now := d.now()
	states := map[uint64]*hookState{}
	started := 0
	waitBudget := webhookSlotWaitBudget
	for page := 0; page < webhookPollPages && started < webhookPollLimit && !d.stopping(); page++ {
		members, err := d.rdb.ZRangeByScore(ctx, WebhookQueueKey, &redis.ZRangeBy{
			Min: "-inf", Max: strconv.FormatInt(now.Unix(), 10), Count: webhookPollLimit,
		}).Result()
		if err != nil {
			logger.Error(ctx, "webhook-dispatcher: failed to read queue", zap.Error(err))
			return started
		}
		progressed := false
		for _, m := range members {
			if d.stopping() || started >= webhookPollLimit {
				return started
			}
			var job WebhookJob
			if err := json.Unmarshal([]byte(m), &job); err != nil {
				logger.Warn(ctx, "webhook-dispatcher: dropping malformed job", zap.Error(err))
				_ = d.rdb.ZRem(ctx, WebhookQueueKey, m).Err()
				progressed = true
				continue
			}
			st := states[job.WebhookID]
			if st == nil {
				st = d.loadState(ctx, job.WebhookID)
				states[job.WebhookID] = st
			}
			switch {
			case st.skip:
				continue
			case st.drop:
				_ = d.rdb.ZRem(ctx, WebhookQueueKey, m).Err()
				progressed = true
				continue
			}
			if !st.saturated && !d.reserve(job.WebhookID, st, &waitBudget) {
				st.saturated = true
			}
			if st.saturated {
				d.deferJob(ctx, m, job.WebhookID, now, st.alert)
				progressed = true
				continue
			}
			select {
			case d.sem <- struct{}{}:
			default:
				// Every global slot is busy: leave the rest due for the
				// next poll.
				d.release(job.WebhookID)
				return started
			}
			n, err := d.rdb.ZRem(ctx, WebhookQueueKey, m).Result()
			if err != nil || n == 0 {
				if err != nil {
					logger.Error(ctx, "webhook-dispatcher: failed to claim job", zap.Error(err))
				}
				<-d.sem
				d.release(job.WebhookID)
				continue
			}
			progressed = true
			started++
			d.deliveries.Add(1)
			go func() {
				defer d.deliveries.Done()
				defer func() { <-d.sem }()
				defer d.release(job.WebhookID)
				defer func() {
					if r := recover(); r != nil {
						logger.Error(ctx, "webhook-dispatcher: delivery panicked",
							zap.Uint64("webhook_id", job.WebhookID), zap.String("event_id", job.EventID), zap.Any("panic", r))
					}
				}()
				d.deliver(ctx, job)
			}()
		}
		if len(members) < webhookPollLimit || !progressed {
			break // every due job was seen, or this page will not change
		}
	}
	return started
}

// loadState reads the webhook once per poll.
func (d *WebhookDispatcher) loadState(ctx context.Context, webhookID uint64) *hookState {
	w, err := d.webhooks.GetByID(ctx, webhookID)
	if errors.Is(err, repository.ErrWebhookNotFound) {
		return &hookState{drop: true}
	}
	if err != nil {
		logger.Error(ctx, "webhook-dispatcher: failed to load webhook; leaving its jobs queued",
			zap.Uint64("webhook_id", webhookID), zap.Error(err))
		return &hookState{skip: true}
	}
	if !w.IsActive || w.Status == model.WebhookStatusDeactivated {
		return &hookState{drop: true}
	}
	return &hookState{alert: w.Status == model.WebhookStatusAlert}
}

// reserve takes one of the webhook's in-flight slots. At the cap it waits up
// to webhookSlotWait (charged to *budget when it times out) for a slot to
// free; alert webhooks never wait. It returns false when no slot was taken.
func (d *WebhookDispatcher) reserve(webhookID uint64, st *hookState, budget *time.Duration) bool {
	limit := webhookHookInFlight
	if st.alert {
		limit = webhookAlertInFlight
	}
	var timer *time.Timer
	var waitStart time.Time
	for {
		d.mu.Lock()
		if d.inFlight[webhookID] < limit {
			d.inFlight[webhookID]++
			d.mu.Unlock()
			if timer != nil {
				timer.Stop()
			}
			return true
		}
		d.mu.Unlock()
		if st.alert || *budget <= 0 {
			return false
		}
		if timer == nil {
			wait := min(webhookSlotWait, *budget)
			timer = time.NewTimer(wait)
			waitStart = time.Now()
		}
		select {
		case <-d.released: // some slot freed: re-check
		case <-timer.C:
			*budget -= time.Since(waitStart)
			return false
		case <-d.stopCh:
			timer.Stop()
			return false
		}
	}
}

func (d *WebhookDispatcher) release(webhookID uint64) {
	d.mu.Lock()
	if d.inFlight[webhookID] <= 1 {
		delete(d.inFlight, webhookID)
	} else {
		d.inFlight[webhookID]--
	}
	d.mu.Unlock()
	select {
	case d.released <- struct{}{}:
	default:
	}
}

// deferJob pushes a job for a saturated webhook back without claiming it
// (ZADD XX keeps the member, so its attempt is unchanged, and does nothing
// when another instance has claimed it meanwhile).
func (d *WebhookDispatcher) deferJob(ctx context.Context, member string, webhookID uint64, now time.Time, alert bool) {
	delay := webhookDeferDelay
	if alert {
		delay = webhookBackoff[1]
	}
	err := d.rdb.ZAddArgs(ctx, WebhookQueueKey, redis.ZAddArgs{
		XX:      true,
		Members: []redis.Z{{Score: float64(now.Add(delay).Unix()), Member: member}},
	}).Err()
	if err != nil {
		logger.Error(ctx, "webhook-dispatcher: failed to defer job", zap.Uint64("webhook_id", webhookID), zap.Error(err))
	}
}

// deliver sends one job and records the outcome.
func (d *WebhookDispatcher) deliver(ctx context.Context, job WebhookJob) {
	w, err := d.webhooks.GetByID(ctx, job.WebhookID)
	if err != nil {
		if !errors.Is(err, repository.ErrWebhookNotFound) {
			logger.Error(ctx, "webhook-dispatcher: failed to load webhook; dropping job",
				zap.Uint64("webhook_id", job.WebhookID), zap.Error(err))
		}
		return
	}
	if !w.IsActive || w.Status == model.WebhookStatusDeactivated {
		return
	}

	status, sendErr := d.send(ctx, w, job)
	ok := sendErr == nil && status >= 200 && status < 300
	now := d.now().UTC()
	rec := &model.WebhookDelivery{WebhookID: w.ID, EventID: job.EventID, StatusCode: status, Attempt: job.Attempt,
		DeliveredAt: now}
	if sendErr != nil {
		rec.Error = service.WebhookErrorMessage(sendErr)
	}
	if err := d.webhooks.AddDelivery(ctx, rec); err != nil {
		logger.Error(ctx, "webhook-dispatcher: failed to record delivery", zap.Uint64("webhook_id", w.ID), zap.Error(err))
	}
	// Column-scoped write on the locked current row: a concurrent edit of
	// name/URL survives, parallel failures all count, and a webhook
	// deactivated meanwhile stays deactivated.
	updated, err := d.webhooks.UpdateDeliveryState(ctx, w.ID, func(cur *model.Webhook) {
		service.ApplyDeliveryResult(cur, ok, now)
	})
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return // deleted meanwhile: nothing to retry
		}
		logger.Error(ctx, "webhook-dispatcher: failed to update webhook", zap.Uint64("webhook_id", w.ID), zap.Error(err))
		updated = w
	}
	if ok {
		return
	}
	logger.Info(ctx, "webhook-dispatcher: delivery failed",
		zap.Uint64("webhook_id", w.ID), zap.String("url_host", service.WebhookURLHost(w.URL)),
		zap.Int("attempt", job.Attempt), zap.Int("status_code", status), zap.String("error", rec.Error))
	if job.Attempt >= webhookMaxAttempts || !updated.IsActive || updated.Status == model.WebhookStatusDeactivated {
		return
	}
	next := job
	next.Attempt++
	b, err := json.Marshal(next)
	if err != nil {
		logger.Error(ctx, "webhook-dispatcher: failed to encode retry", zap.Error(err))
		return
	}
	score := float64(d.now().Add(webhookBackoff[job.Attempt]).Unix())
	if err := d.rdb.ZAdd(ctx, WebhookQueueKey, redis.Z{Score: score, Member: string(b)}).Err(); err != nil {
		logger.Error(ctx, "webhook-dispatcher: failed to requeue job", zap.Uint64("webhook_id", w.ID), zap.Error(err))
	}
}

// send POSTs the signed payload and returns the response status (0 when no
// response was received).
func (d *WebhookDispatcher) send(ctx context.Context, w *model.Webhook, job WebhookJob) (int, error) {
	timeout := d.client.Timeout
	if timeout <= 0 {
		timeout = webhookDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := []byte(job.Payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", webhookUserAgent)
	req.Header.Set("X-Signature", service.SignBody(w.Secret, body))
	req.Header.Set("X-Webhook-Id", strconv.FormatUint(w.ID, 10))
	req.Header.Set("X-Event-Id", job.EventID)
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, webhookResponseMax))
	return resp.StatusCode, nil
}

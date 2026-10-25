package worker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var hookNow = time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC)

// fakeWebhookRepo is an in-memory repository.WebhookRepository.
type fakeWebhookRepo struct {
	mu         sync.Mutex
	hooks      map[uint64]*model.Webhook
	deliveries []model.WebhookDelivery
	updates    int
}

var _ repository.WebhookRepository = (*fakeWebhookRepo)(nil)

func newFakeWebhookRepo(ws ...model.Webhook) *fakeWebhookRepo {
	f := &fakeWebhookRepo{hooks: map[uint64]*model.Webhook{}}
	for i := range ws {
		w := ws[i]
		f.hooks[w.ID] = &w
	}
	return f
}

func (f *fakeWebhookRepo) Create(_ context.Context, w *model.Webhook) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := *w
	f.hooks[w.ID] = &c
	return nil
}

func (f *fakeWebhookRepo) GetByID(_ context.Context, id uint64) (*model.Webhook, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.hooks[id]
	if !ok {
		return nil, repository.ErrWebhookNotFound
	}
	c := *w
	return &c, nil
}

func (f *fakeWebhookRepo) ListByGroup(_ context.Context, groupID uint64) ([]model.Webhook, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []model.Webhook{}
	for _, w := range f.hooks {
		if w.GroupID == groupID {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (f *fakeWebhookRepo) ListActiveByGroup(ctx context.Context, groupID uint64) ([]model.Webhook, error) {
	all, _ := f.ListByGroup(ctx, groupID)
	out := []model.Webhook{}
	for _, w := range all {
		if w.IsActive && w.Status != model.WebhookStatusDeactivated {
			out = append(out, w)
		}
	}
	return out, nil
}

func (f *fakeWebhookRepo) Update(_ context.Context, w *model.Webhook) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.hooks[w.ID]; !ok {
		return repository.ErrWebhookNotFound
	}
	c := *w
	f.hooks[w.ID] = &c
	f.updates++
	return nil
}

// UpdateDeliveryState mirrors the repository: apply on the current row,
// write only the delivery-state columns, never reactivate.
func (f *fakeWebhookRepo) UpdateDeliveryState(_ context.Context, id uint64, apply func(*model.Webhook)) (*model.Webhook, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored, ok := f.hooks[id]
	if !ok {
		return nil, repository.ErrWebhookNotFound
	}
	cur := *stored
	wasActive, wasStatus := cur.IsActive, cur.Status
	apply(&cur)
	if !wasActive || wasStatus == model.WebhookStatusDeactivated {
		cur.IsActive, cur.Status = wasActive, wasStatus
	}
	stored.IsActive, stored.Status = cur.IsActive, cur.Status
	stored.ConsecutiveFailures, stored.FailingSince, stored.LastDeliveryAt = cur.ConsecutiveFailures, cur.FailingSince, cur.LastDeliveryAt
	f.updates++
	out := *stored
	return &out, nil
}

func (f *fakeWebhookRepo) Delete(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.hooks, id)
	return nil
}

func (f *fakeWebhookRepo) AddDelivery(_ context.Context, d *model.WebhookDelivery) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliveries = append(f.deliveries, *d)
	return nil
}

func (f *fakeWebhookRepo) ListDeliveries(_ context.Context, webhookID uint64, _ int) ([]model.WebhookDelivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []model.WebhookDelivery{}
	for _, d := range f.deliveries {
		if d.WebhookID == webhookID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeWebhookRepo) snapshot(id uint64) model.Webhook {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.hooks[id]
}

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func hook(id, groupID uint64, url string) model.Webhook {
	return model.Webhook{ID: id, UserID: 1, GroupID: groupID, Name: "h" + strconv.FormatUint(id, 10), URL: url,
		Event: "engagement", Secret: "s3cret-" + strconv.FormatUint(id, 10), IsActive: true, Status: model.WebhookStatusActive}
}

func queueJobs(t *testing.T, rdb *redis.Client) []redis.Z {
	t.Helper()
	zs, err := rdb.ZRangeWithScores(context.Background(), WebhookQueueKey, 0, -1).Result()
	require.NoError(t, err)
	return zs
}

func decodeJob(t *testing.T, member any) WebhookJob {
	t.Helper()
	var j WebhookJob
	require.NoError(t, json.Unmarshal([]byte(member.(string)), &j))
	return j
}

func pushJob(t *testing.T, rdb *redis.Client, j WebhookJob, score time.Time) {
	t.Helper()
	b, err := json.Marshal(j)
	require.NoError(t, err)
	require.NoError(t, rdb.ZAdd(context.Background(), WebhookQueueKey,
		redis.Z{Score: float64(score.Unix()), Member: string(b)}).Err())
}

func newTestDispatcher(rdb *redis.Client, repo repository.WebhookRepository, client *http.Client) *WebhookDispatcher {
	d := NewWebhookDispatcher(rdb, repo, client, time.Hour)
	d.now = func() time.Time { return hookNow }
	return d
}

func TestEnqueuer_OneJobPerClickPerActiveWebhook(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctrl := gomock.NewController(t)
	links := mocks.NewMockLinkRepository(ctrl)
	tags := mocks.NewMockTagRepository(ctrl)
	domains := mocks.NewMockDomainRepository(ctrl)

	deactivated := hook(3, 7, "https://c.example.test")
	deactivated.IsActive, deactivated.Status = false, model.WebhookStatusDeactivated
	repo := newFakeWebhookRepo(hook(1, 7, "https://a.example.test"), hook(2, 7, "https://b.example.test"),
		deactivated, hook(4, 8, "https://other-group.example.test"))

	domainID := uint64(9)
	link := model.Link{ID: 42, GroupID: 7, ShortCode: "abc", OriginalURL: "https://dest.example.com/page", DomainID: &domainID}
	links.EXPECT().GetByIDs(gomock.Any(), []uint64{42}).Return([]model.Link{link}, nil)
	tags.EXPECT().GetTagsForLinks(gomock.Any(), []uint64{42}).Return(map[uint64][]string{42: {"promo", "q4"}}, nil)
	domains.EXPECT().GetByID(gomock.Any(), uint64(9)).Return(&model.Domain{ID: 9, Domain: "go.example.com"}, nil)

	e := NewWebhookEnqueuer(rdb, repo, links, tags, domains, "https://sho.rt")
	e.now = func() time.Time { return hookNow }
	clicked := time.Date(2026, 10, 25, 14, 59, 30, 0, time.FixedZone("x", 3600))
	e.Enqueue(context.Background(), []model.Click{
		{LinkID: 42, ClickedAt: clicked, Referrer: "https://news.example", Country: "DE", DeviceType: "mobile", OS: "iOS", Source: "qr"},
		{LinkID: 42, ClickedAt: clicked, Source: "link"},
	})

	zs := queueJobs(t, rdb)
	require.Len(t, zs, 4)
	perHook := map[uint64]int{}
	events := map[string]bool{}
	for _, z := range zs {
		assert.Equal(t, float64(hookNow.Unix()), z.Score)
		j := decodeJob(t, z.Member)
		perHook[j.WebhookID]++
		assert.Equal(t, 1, j.Attempt)
		assert.Len(t, j.EventID, 32)
		var p map[string]any
		require.NoError(t, json.Unmarshal(j.Payload, &p))
		assert.Equal(t, j.EventID, p["event_id"])
		events[j.EventID+strconv.FormatUint(j.WebhookID, 10)] = true
		assert.Equal(t, "engagement", p["event_type"])
		assert.Equal(t, "2026-10-25T13:59:30Z", p["datetime"])
		assert.Equal(t, "https://go.example.com/abc", p["bitlink"])
		assert.Equal(t, "https://dest.example.com/page", p["long_url"])
		assert.Equal(t, []any{"promo", "q4"}, p["tags"])
		assert.Equal(t, float64(7), p["group_id"])
		for _, k := range []string{"referrer", "country", "device_type", "os", "source"} {
			assert.Contains(t, p, k)
		}
		if p["source"] == "qr" {
			assert.Equal(t, "https://news.example", p["referrer"])
			assert.Equal(t, "DE", p["country"])
			assert.Equal(t, "mobile", p["device_type"])
			assert.Equal(t, "iOS", p["os"])
		} else {
			assert.Equal(t, "link", p["source"])
		}
	}
	assert.Equal(t, map[uint64]int{1: 2, 2: 2}, perHook)
	assert.Len(t, events, 4)
}

func TestEnqueuer_NoWebhooksSkipsLookups(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctrl := gomock.NewController(t)
	links := mocks.NewMockLinkRepository(ctrl)
	links.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).Return([]model.Link{{ID: 1, GroupID: 7, ShortCode: "x"}}, nil)
	// Tags and domains are never queried when no webhook listens.
	e := NewWebhookEnqueuer(rdb, newFakeWebhookRepo(), links, mocks.NewMockTagRepository(ctrl),
		mocks.NewMockDomainRepository(ctrl), "https://sho.rt")
	e.Enqueue(context.Background(), []model.Click{{LinkID: 1, ClickedAt: hookNow}})
	assert.Empty(t, queueJobs(t, rdb))
}

// countingWebhookRepo counts ListActiveByGroup calls per group.
type countingWebhookRepo struct {
	*fakeWebhookRepo
	mu    sync.Mutex
	calls map[uint64]int
}

func (c *countingWebhookRepo) ListActiveByGroup(ctx context.Context, groupID uint64) ([]model.Webhook, error) {
	c.mu.Lock()
	c.calls[groupID]++
	c.mu.Unlock()
	return c.fakeWebhookRepo.ListActiveByGroup(ctx, groupID)
}

func TestEnqueuer_CachesActiveWebhooks(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctrl := gomock.NewController(t)
	links := mocks.NewMockLinkRepository(ctrl)
	tags := mocks.NewMockTagRepository(ctrl)
	tags.EXPECT().GetTagsForLinks(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	// Group 7 has a webhook, group 8 has none.
	repo := &countingWebhookRepo{fakeWebhookRepo: newFakeWebhookRepo(hook(1, 7, "https://a.example.test")),
		calls: map[uint64]int{}}
	links.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).Return([]model.Link{
		{ID: 1, GroupID: 7, ShortCode: "a"}, {ID: 2, GroupID: 8, ShortCode: "b"},
	}, nil).Times(3)
	e := NewWebhookEnqueuer(rdb, repo, links, tags, mocks.NewMockDomainRepository(ctrl), "https://sho.rt")
	now := hookNow
	e.now = func() time.Time { return now }
	batch := []model.Click{{LinkID: 1, ClickedAt: hookNow}, {LinkID: 2, ClickedAt: hookNow}}

	e.Enqueue(context.Background(), batch)
	now = hookNow.Add(4 * time.Second)
	e.Enqueue(context.Background(), batch)
	assert.Equal(t, map[uint64]int{7: 1, 8: 1}, repo.calls, "one lookup per group within 5 s")
	assert.Len(t, queueJobs(t, rdb), 2, "both batches still queue their events")

	now = hookNow.Add(6 * time.Second)
	e.Enqueue(context.Background(), batch)
	assert.Equal(t, map[uint64]int{7: 2, 8: 2}, repo.calls, "the cache expires after 5 s")
}

func TestDispatcher_SignsBody(t *testing.T) {
	_, rdb := newTestRedis(t)
	type captured struct {
		header http.Header
		body   []byte
	}
	got := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- captured{header: r.Header.Clone(), body: b}
		rw.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	w := hook(5, 7, srv.URL+"/hook")
	w.Status, w.ConsecutiveFailures = model.WebhookStatusAlert, 3
	w.FailingSince.Time, w.FailingSince.Valid = hookNow.Add(-time.Hour), true
	repo := newFakeWebhookRepo(w)
	payload := json.RawMessage(`{"event_id":"ev1","event_type":"engagement"}`)
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: payload}, hookNow)

	d := newTestDispatcher(rdb, repo, srv.Client())
	d.poll()

	c := <-got
	assert.Equal(t, string(payload), string(c.body))
	assert.Equal(t, service.SignBody(w.Secret, c.body), c.header.Get("X-Signature"))
	assert.Equal(t, "ev1", c.header.Get("X-Event-Id"))
	assert.Equal(t, "5", c.header.Get("X-Webhook-Id"))
	assert.Equal(t, "application/json", c.header.Get("Content-Type"))
	assert.Equal(t, "urlshortener-webhooks/1.0", c.header.Get("User-Agent"))

	assert.Empty(t, queueJobs(t, rdb))
	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, model.WebhookDelivery{WebhookID: 5, EventID: "ev1", StatusCode: 204, Attempt: 1, DeliveredAt: hookNow},
		repo.deliveries[0])
	after := repo.snapshot(5)
	assert.Equal(t, model.WebhookStatusActive, after.Status)
	assert.Equal(t, 0, after.ConsecutiveFailures)
	assert.False(t, after.FailingSince.Valid)
	assert.True(t, after.LastDeliveryAt.Valid)
}

func TestDispatcher_RetriesWithBackoff(t *testing.T) {
	_, rdb := newTestRedis(t)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	repo := newFakeWebhookRepo(hook(5, 7, srv.URL))
	d := newTestDispatcher(rdb, repo, srv.Client())
	payload := json.RawMessage(`{"event_id":"ev1"}`)
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: payload}, hookNow)

	d.poll()
	zs := queueJobs(t, rdb)
	require.Len(t, zs, 1)
	assert.Equal(t, float64(hookNow.Add(60*time.Second).Unix()), zs[0].Score)
	j := decodeJob(t, zs[0].Member)
	assert.Equal(t, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 2, Payload: payload}, j)
	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, 500, repo.deliveries[0].StatusCode)
	assert.Equal(t, 1, repo.deliveries[0].Attempt)
	assert.Equal(t, 1, repo.snapshot(5).ConsecutiveFailures)

	// Not due yet: nothing happens.
	d.poll()
	assert.Len(t, repo.deliveries, 1)

	// Each later attempt waits longer; attempt 5 is the last.
	wantDelay := map[int]time.Duration{2: 5 * time.Minute, 3: 30 * time.Minute, 4: 2 * time.Hour}
	for attempt := 2; attempt <= 5; attempt++ {
		now := time.Unix(int64(queueJobs(t, rdb)[0].Score), 0).UTC()
		d.now = func() time.Time { return now }
		d.poll()
		require.Len(t, repo.deliveries, attempt)
		assert.Equal(t, attempt, repo.deliveries[attempt-1].Attempt)
		zs = queueJobs(t, rdb)
		if attempt == 5 {
			assert.Empty(t, zs, "no retry after the 5th attempt")
			break
		}
		require.Len(t, zs, 1)
		assert.Equal(t, float64(now.Add(wantDelay[attempt]).Unix()), zs[0].Score)
		assert.Equal(t, attempt+1, decodeJob(t, zs[0].Member).Attempt)
	}
	assert.Equal(t, model.WebhookStatusAlert, repo.snapshot(5).Status)
	assert.Equal(t, 5, repo.snapshot(5).ConsecutiveFailures)
}

func TestDispatcher_Timeout(t *testing.T) {
	_, rdb := newTestRedis(t)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		select {
		case <-release:
		case <-time.After(200 * time.Millisecond):
		}
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	repo := newFakeWebhookRepo(hook(5, 7, srv.URL+"/secret-token-path?token=abc"))
	client := srv.Client()
	client.Timeout = 50 * time.Millisecond
	d := newTestDispatcher(rdb, repo, client)
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)

	start := time.Now()
	d.poll()
	assert.Less(t, time.Since(start), 190*time.Millisecond)
	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, 0, repo.deliveries[0].StatusCode)
	assert.NotEmpty(t, repo.deliveries[0].Error)
	assert.NotContains(t, repo.deliveries[0].Error, "secret-token-path", "the full URL is never stored")
	assert.LessOrEqual(t, len([]rune(repo.deliveries[0].Error)), 500)
	assert.Len(t, queueJobs(t, rdb), 1, "a timeout is retried")
}

func TestDispatcher_DropsDeactivated(t *testing.T) {
	_, rdb := newTestRedis(t)
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		hits++
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	off := hook(5, 7, srv.URL)
	off.IsActive, off.Status = false, model.WebhookStatusDeactivated
	paused := hook(6, 7, srv.URL)
	paused.IsActive = false
	repo := newFakeWebhookRepo(off, paused)
	d := newTestDispatcher(rdb, repo, srv.Client())
	for _, id := range []uint64{5, 6, 99} { // 99: deleted
		pushJob(t, rdb, WebhookJob{WebhookID: id, EventID: "ev", Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)
	}

	d.poll()
	assert.Equal(t, 0, hits)
	assert.Empty(t, queueJobs(t, rdb))
	assert.Empty(t, repo.deliveries)
	assert.Equal(t, 0, repo.updates)
}

func TestDispatcher_StopWaitsForInFlight(t *testing.T) {
	_, rdb := newTestRedis(t)
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(100 * time.Millisecond)
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	repo := newFakeWebhookRepo(hook(5, 7, srv.URL))
	d := NewWebhookDispatcher(rdb, repo, srv.Client(), 10*time.Millisecond)
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: json.RawMessage(`{}`)}, time.Now())

	d.Start()
	<-started
	d.Stop()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.deliveries, 1, "the in-flight delivery is recorded before Stop returns")
	assert.Equal(t, 200, repo.deliveries[0].StatusCode)
}

// recordingEnqueuer captures what the flusher hands over and, when rdb is
// set, the click buffer length at that moment.
type recordingEnqueuer struct {
	got       [][]model.Click
	rdb       *redis.Client
	bufferLen []int64
}

func (r *recordingEnqueuer) Enqueue(ctx context.Context, clicks []model.Click) {
	r.got = append(r.got, clicks)
	if r.rdb != nil {
		n, _ := r.rdb.LLen(ctx, "clicks:buffer").Result()
		r.bufferLen = append(r.bufferLen, n)
	}
}

func TestClickFlusher_TrimsBeforeEnqueue(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctrl := gomock.NewController(t)
	clicks := mocks.NewMockClickRepository(ctrl)
	ev, err := json.Marshal(service.ClickEvent{LinkID: 42, ClickedAt: hookNow, Referrer: "r"})
	require.NoError(t, err)
	require.NoError(t, rdb.RPush(context.Background(), "clicks:buffer", string(ev)).Err())

	rec := &recordingEnqueuer{rdb: rdb}
	inserted := false
	clicks.EXPECT().BatchInsert(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, []model.Click) error {
		inserted = true
		assert.Empty(t, rec.got, "enqueue happens after the insert")
		return nil
	})
	f := NewClickFlusher(rdb, clicks)
	f.SetEnqueuer(rec)
	f.flush()

	assert.True(t, inserted)
	require.Len(t, rec.got, 1)
	require.Len(t, rec.got[0], 1)
	assert.Equal(t, uint64(42), rec.got[0][0].LinkID)
	assert.Equal(t, "link", rec.got[0][0].Source)
	require.Len(t, rec.bufferLen, 1)
	assert.Zero(t, rec.bufferLen[0], "the buffer is trimmed before enqueue runs")
}

// panicEnqueuer panics; slowEnqueuer blocks until its context ends.
type panicEnqueuer struct{ calls int }

func (p *panicEnqueuer) Enqueue(context.Context, []model.Click) { p.calls++; panic("boom") }

type slowEnqueuer struct{ calls int }

func (s *slowEnqueuer) Enqueue(ctx context.Context, _ []model.Click) { s.calls++; <-ctx.Done() }

func TestClickFlusher_EnqueueFailureDoesNotBlockTrim(t *testing.T) {
	for name, enq := range map[string]ClickEnqueuer{"panics": &panicEnqueuer{}, "slow": &slowEnqueuer{}} {
		t.Run(name, func(t *testing.T) {
			_, rdb := newTestRedis(t)
			ctrl := gomock.NewController(t)
			clicks := mocks.NewMockClickRepository(ctrl)
			ctx := context.Background()
			for i := 0; i < 3; i++ {
				ev, _ := json.Marshal(service.ClickEvent{LinkID: uint64(i + 1), ClickedAt: hookNow})
				require.NoError(t, rdb.RPush(ctx, "clicks:buffer", string(ev)).Err())
			}
			// Exactly one insert across two flushes: nothing is re-inserted.
			clicks.EXPECT().BatchInsert(gomock.Any(), gomock.Len(3)).Return(nil).Times(1)
			f := NewClickFlusher(rdb, clicks)
			f.enqueueTimeout = 50 * time.Millisecond
			f.SetEnqueuer(enq)

			start := time.Now()
			assert.NotPanics(t, f.flush)
			assert.Less(t, time.Since(start), 2*time.Second, "a slow enqueuer is bounded by its own timeout")
			n, err := rdb.LLen(ctx, "clicks:buffer").Result()
			require.NoError(t, err)
			assert.Zero(t, n)

			f.flush()
		})
	}
}

func TestClickFlusher_InsertFailureSkipsEnqueue(t *testing.T) {
	_, rdb := newTestRedis(t)
	ctrl := gomock.NewController(t)
	clicks := mocks.NewMockClickRepository(ctrl)
	ev, _ := json.Marshal(service.ClickEvent{LinkID: 1, ClickedAt: hookNow})
	require.NoError(t, rdb.RPush(context.Background(), "clicks:buffer", string(ev)).Err())
	clicks.EXPECT().BatchInsert(gomock.Any(), gomock.Any()).Return(assert.AnError)
	rec := &recordingEnqueuer{}
	f := NewClickFlusher(rdb, clicks)
	f.SetEnqueuer(rec)
	f.flush()
	assert.Empty(t, rec.got)
}

func TestDispatcher_KeepsConcurrentURLChangeAndDeactivation(t *testing.T) {
	_, rdb := newTestRedis(t)
	var repo *fakeWebhookRepo
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		// While the delivery is in flight the user moves and deactivates it.
		repo.mu.Lock()
		repo.hooks[5].URL = "https://moved.example.test"
		repo.hooks[5].IsActive, repo.hooks[5].Status = false, model.WebhookStatusDeactivated
		repo.mu.Unlock()
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	repo = newFakeWebhookRepo(hook(5, 7, srv.URL))
	d := newTestDispatcher(rdb, repo, srv.Client())
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)

	d.poll()
	after := repo.snapshot(5)
	assert.Equal(t, "https://moved.example.test", after.URL)
	assert.False(t, after.IsActive)
	assert.Equal(t, model.WebhookStatusDeactivated, after.Status)
	assert.Equal(t, 1, after.ConsecutiveFailures)
	assert.Empty(t, queueJobs(t, rdb), "no retry for a webhook deactivated meanwhile")
}

// panickyRepo panics on AddDelivery.
type panickyRepo struct{ *fakeWebhookRepo }

func (p panickyRepo) AddDelivery(context.Context, *model.WebhookDelivery) error { panic("boom") }

func TestDispatcher_RecoversFromPanic(t *testing.T) {
	_, rdb := newTestRedis(t)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	d := newTestDispatcher(rdb, panickyRepo{newFakeWebhookRepo(hook(5, 7, srv.URL))}, srv.Client())
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)
	assert.NotPanics(t, d.poll)
}

func TestDispatcher_SlowEndpointDoesNotStarveOthers(t *testing.T) {
	_, rdb := newTestRedis(t)
	var slowHits, fastHits atomic.Int32
	slow := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		slowHits.Add(1)
		time.Sleep(2 * time.Second)
		rw.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		fastHits.Add(1)
		rw.WriteHeader(http.StatusOK)
	}))
	defer fast.Close()

	repo := newFakeWebhookRepo(hook(5, 7, slow.URL), hook(6, 7, fast.URL))
	client := &http.Client{Timeout: 3 * time.Second}
	d := newTestDispatcher(rdb, repo, client)
	for i := 0; i < 6; i++ {
		pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "slow" + strconv.Itoa(i), Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)
	}
	pushJob(t, rdb, WebhookJob{WebhookID: 6, EventID: "fast", Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)

	d.poll()

	assert.Equal(t, int32(1), fastHits.Load())
	assert.LessOrEqual(t, slowHits.Load(), int32(2))
	var fastDelivered bool
	for _, del := range repo.deliveries {
		if del.WebhookID == 6 && del.EventID == "fast" && del.StatusCode == 200 {
			fastDelivered = true
		}
	}
	assert.True(t, fastDelivered, "the fast webhook's delivery is recorded")
	zs := queueJobs(t, rdb)
	assert.Len(t, zs, 6-int(slowHits.Load()))
	for _, z := range zs {
		j := decodeJob(t, z.Member)
		assert.Equal(t, uint64(5), j.WebhookID)
		assert.Equal(t, 1, j.Attempt, "a deferred job is not a retry")
		assert.Equal(t, float64(hookNow.Add(5*time.Second).Unix()), z.Score)
	}
}

func TestDispatcher_AlertWebhookGetsOneProbePerPoll(t *testing.T) {
	_, rdb := newTestRedis(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	w := hook(5, 7, srv.URL)
	w.Status, w.ConsecutiveFailures = model.WebhookStatusAlert, 2
	w.FailingSince.Time, w.FailingSince.Valid = hookNow.Add(-time.Hour), true
	repo := newFakeWebhookRepo(w)
	d := newTestDispatcher(rdb, repo, srv.Client())
	for i := 0; i < 3; i++ {
		pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev" + strconv.Itoa(i), Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)
	}

	d.poll()

	assert.Equal(t, int32(1), hits.Load())
	require.Len(t, repo.deliveries, 1)
	zs := queueJobs(t, rdb)
	require.Len(t, zs, 3, "two deferred jobs plus the probe's retry")
	var deferred, retried int
	for _, z := range zs {
		j := decodeJob(t, z.Member)
		assert.Equal(t, float64(hookNow.Add(60*time.Second).Unix()), z.Score)
		switch j.Attempt {
		case 1:
			deferred++
		case 2:
			retried++
			assert.Equal(t, repo.deliveries[0].EventID, j.EventID)
		}
	}
	assert.Equal(t, 2, deferred)
	assert.Equal(t, 1, retried)
}

func TestDispatcher_SkipsTickWhilePolling(t *testing.T) {
	_, rdb := newTestRedis(t)
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		rw.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	repo := newFakeWebhookRepo(hook(5, 7, srv.URL))
	d := newTestDispatcher(rdb, repo, srv.Client())
	pushJob(t, rdb, WebhookJob{WebhookID: 5, EventID: "ev1", Attempt: 1, Payload: json.RawMessage(`{}`)}, hookNow)

	require.True(t, d.tryPoll())
	<-started
	assert.False(t, d.tryPoll(), "a tick while deliveries run is skipped")
	close(release)
	d.pollWG.Wait()
	assert.True(t, d.tryPoll(), "the next tick polls again")
	d.pollWG.Wait()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Len(t, repo.deliveries, 1)
}

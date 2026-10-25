# Phase 5: Campaigns, Webhooks, Bulk upload, Custom-domain completion — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver spec §8 phase 5 end to end: Campaigns (channels, membership, auto-UTM, performance), Webhooks (CRUD, verify ping, signed engagement deliveries with retry and alert/deactivate states), Bulk upload (CSV → links by a background worker with a results CSV), and the one missing custom-domain action ("Set as default for group").

**Architecture:** Three new backend feature slices, each repository → service → handler with mocks, one migration (`009_campaigns_webhooks_bulk.sql`). Campaign engagements reuse `ClickRepository` scope queries over the member link ids. Webhook events are enqueued by the click flusher after each successful `BatchInsert` into a Redis sorted set keyed by delivery time; a `WebhookDispatcher` worker signs and POSTs them with retries. Bulk uploads store the CSV in the job row; a `BulkWorker` polls queued jobs and creates links through the existing `LinkService.Create` in batches of 50. Frontend adds TanStack Query hooks, the Campaigns pages, the Webhooks and Bulk upload settings pages, the link "Add to campaign" dialog, and the Campaign filter.

**Tech Stack:** Go 1.25, Gin, sqlx, MariaDB, go-redis, `encoding/csv`, `crypto/hmac`, testify, mockgen; Next.js 16, React 19, TanStack Query 5, shadcn (Dialog, Select, Tabs, Table, Switch, Checkbox), recharts (`TimeSeriesChart`), Vitest.

**Spec:** `docs/superpowers/specs/2026-10-24-bitly-clone-design.md` §5.2 (link ⋯ menu "Add to campaign", Filters "Campaign"), §5.9, §5.10 (row action "Set as default for group"), §5.11 (Webhooks, Bulk upload, Activity log actions), §6.1 (campaigns/channels/campaign_channels/campaign_links/webhooks/webhook_deliveries/bulk_uploads), §6.2 (Campaigns, Webhooks, Bulk endpoints), §6.3 (webhook dispatch), §11.5 (campaign membership is metadata; auto-UTM only on create-from-channel). Ledgers: `docs/superpowers/plans/2026-10-25-phase4-ledger.md`.

## Global Constraints

- Repo root `/Users/jose/GolandProjects/urlshortener`, branch `feat/bitly-clone`, backend `backend/`, frontend `frontend/`. Commit after every task; messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` (an implementing subagent uses its own model name). Never `git add -A`; add files by path.
- Go: `cd backend && go vet ./... && TEST_DATABASE_DSN="urlshortener:urlshortener_pass@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC" go test -p 1 ./...` green. Apply the migration to BOTH local databases first: `docker exec -i us-mariadb mariadb -uroot -proot_secret_123 urlshortener < backend/migrations/009_campaigns_webhooks_bulk.sql` and the same for `urlshortener_test`. Migrations are idempotent (`CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`) and follow the `database.SplitStatements` rules (comments only as whole `--` lines; every statement ends with `;` at end of line). `go generate ./...` after any interface change; `go mod vendor` after any `go get` (none expected). Add new tables to `internal/testutil/db.go`'s delete list, children first.
- Frontend: `cd frontend && npx vitest run && npm run lint && npx tsc --noEmit && npm run build` green. No new npm dependencies.
- No billing/upgrade/AI/integrations UI. Copy fixed by spec: "Campaigns", "Create campaign", "Edit", "Delete", "Channels", "Links", "Add link to channel", "Performance", "Auto-add UTM parameters", "Add to campaign", "Webhooks", "Create webhook", "Verify", "Deactivate", "Active", "Alert", "Disabled", "Bulk upload", "Download template", "Download results CSV", "Set as default for group".
- Visual tokens: Phase 1 tokens in `frontend/src/app/globals.css`; cards `rounded-xl bg-card p-6`; charts palette in `src/components/charts/palette.ts`.
- Dates: UTC calendar days (`YYYY-MM-DD`), inclusive; reuse `NormalizeStatsQuery`/`StatsQuery` and the Phase 3 `rangeFor` UTC presets.
- Error JSON `{"error": "..."}`; every `/campaigns`, `/channels`, `/webhooks`, `/bulk-uploads` route is JWT/API-token protected and group-scoped (`X-Group-ID` via `middleware.GetGroupID`, user via `middleware.GetUserID`). Cross-group ids are 404.
- Activity log actions (constants in `service/activity_service.go`): `campaign.create`, `campaign.update`, `campaign.delete`, `webhook.create`, `webhook.update`, `webhook.verify`, `webhook.deactivate`, `webhook.delete`, `bulk.create`.

## Review Focus

1. A campaign, channel, webhook or bulk job of another group must be 404 on every route, and `POST /campaigns/:id/links` must reject link ids outside the group or with `source <> 'link'` (400) — Task 2 (`TestCampaignService_WrongGroup`, `TestCampaignService_AddLinks_ForeignLink`), Task 4 (`TestWebhookService_WrongGroup`), Task 6 (`TestBulkService_WrongGroup`).
2. Auto-UTM must apply only when a link is created from inside a channel, never when an existing link is added (§11.5) — Task 2 (`TestCampaignService_CreateLinkInChannel_AutoUTM`, `TestCampaignService_AddLinks_NeverRewritesURL`).
3. Webhook URLs are attacker-controlled outbound requests: only `http`/`https`, hostnames must not resolve to loopback, link-local or RFC1918 ranges at create/update/verify time; delivery uses a 10 s timeout and never follows more than 3 redirects — Task 4 (`TestWebhookService_RejectsPrivateURL`), Task 5 (`TestDispatcher_Timeout`).
4. Webhook secrets are per-user data: GET responses include the secret only for the owner's webhooks (group-scoped), deliveries never log the secret, and `X-Signature` is the lowercase hex HMAC-SHA256 of the exact raw body — Task 5 (`TestDispatcher_SignsBody`).
5. Bulk CSV input is untrusted: a malformed row never aborts the job (it becomes a failed row with an error), formula-leading cells in the results CSV are prefixed by the existing `csvSafe`, files over 2 MiB or over 1000 data rows are rejected with 400, and a duplicate custom back-half fails only that row — Task 6 (`TestBulkService_ParseLimits`, `TestBulkWorker_RowErrorsDoNotAbort`).
6. Deleting a link removes its campaign memberships and deleting a campaign/channel removes memberships without touching links (FK cascades) — Task 1 (`TestCampaignRepo_Cascades`).

---

## File map (backend)

- Create `backend/migrations/009_campaigns_webhooks_bulk.sql`; `internal/model/{campaign.go, webhook.go, bulk_upload.go}`; `internal/repository/{campaign_repo.go, webhook_repo.go, bulk_upload_repo.go}` (+`_test.go`); `internal/service/{campaign_service.go, webhook_service.go, webhook_url.go, bulk_service.go}` (+`_test.go`); `internal/handler/{campaign.go, webhook.go, bulk_upload.go}` (+`_test.go`); `internal/worker/{webhook_dispatcher.go, bulk_worker.go}` (+`_test.go`).
- Modify `repository/interfaces.go`, `repository/link_repo.go` (`CampaignID` filter), `service/interfaces.go`, `service/activity_service.go`, `worker/click_flusher.go` (enqueue hook), `internal/testutil/db.go`, `cmd/server/main.go`, `internal/config/config.go` (`Webhooks`/`Bulk` sections), `README.md`.

## File map (frontend)

- Create `src/lib/hooks/{useCampaigns.ts, useWebhooks.ts, useBulkUploads.ts}`; `src/components/campaigns/{CampaignDialog.tsx, CampaignCard.tsx, ChannelPicker.tsx, ChannelCard.tsx, AddLinkToChannelDialog.tsx, CampaignPerformance.tsx, CampaignLinksTable.tsx}`; `src/components/links/AddToCampaignDialog.tsx`; `src/components/settings/{WebhookDialog.tsx, WebhookRow.tsx, DeliveriesPanel.tsx, BulkUploadForm.tsx, BulkJobRow.tsx, bulkTemplate.ts}`; pages `src/app/dashboard/campaigns/page.tsx` (replace placeholder), `src/app/dashboard/campaigns/[id]/page.tsx`, `src/app/dashboard/settings/webhooks/page.tsx` (replace), `src/app/dashboard/settings/bulk-upload/page.tsx` (replace).
- Modify `src/types/index.ts`, `src/lib/api.ts`, `src/lib/hooks/{index,keys}.ts`, `src/components/links/{LinkActionsMenu,FiltersDialog,linksUrlState}.tsx|ts`, `src/components/domains/DomainRow.tsx`, `src/components/settings/activity.ts` (verbs `deactivate` → "Deactivated").

---

## Shared contracts (every task reads this)

**Migration `009_campaigns_webhooks_bulk.sql`:**

```sql
CREATE TABLE IF NOT EXISTS campaigns (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    group_id    BIGINT UNSIGNED NOT NULL,
    name        VARCHAR(120) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    auto_utm    TINYINT(1) NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_campaigns_group (group_id, created_at),
    CONSTRAINT fk_campaigns_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS channels (
    id         BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    group_id   BIGINT UNSIGNED NOT NULL,
    name       VARCHAR(80) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_channels_group_name (group_id, name),
    CONSTRAINT fk_channels_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS campaign_channels (
    campaign_id BIGINT UNSIGNED NOT NULL,
    channel_id  BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (campaign_id, channel_id),
    CONSTRAINT fk_cc_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE,
    CONSTRAINT fk_cc_channel  FOREIGN KEY (channel_id)  REFERENCES channels(id)  ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS campaign_links (
    campaign_id BIGINT UNSIGNED NOT NULL,
    channel_id  BIGINT UNSIGNED NOT NULL,
    link_id     BIGINT UNSIGNED NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (campaign_id, channel_id, link_id),
    INDEX idx_campaign_links_link (link_id),
    CONSTRAINT fk_cl_cc   FOREIGN KEY (campaign_id, channel_id) REFERENCES campaign_channels(campaign_id, channel_id) ON DELETE CASCADE,
    CONSTRAINT fk_cl_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS webhooks (
    id                   BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id              BIGINT UNSIGNED NOT NULL,
    group_id             BIGINT UNSIGNED NOT NULL,
    name                 VARCHAR(120) NOT NULL,
    url                  VARCHAR(2048) NOT NULL,
    event                VARCHAR(50) NOT NULL DEFAULT 'engagement',
    secret               VARCHAR(64) NOT NULL,
    is_active            TINYINT(1) NOT NULL DEFAULT 1,
    status               ENUM('active','alert','deactivated') NOT NULL DEFAULT 'active',
    consecutive_failures INT NOT NULL DEFAULT 0,
    failing_since        DATETIME NULL,
    last_delivery_at     DATETIME NULL,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_webhooks_group (group_id),
    CONSTRAINT fk_webhooks_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id           BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    webhook_id   BIGINT UNSIGNED NOT NULL,
    event_id     VARCHAR(64) NOT NULL,
    status_code  INT NOT NULL DEFAULT 0,
    attempt      INT NOT NULL DEFAULT 1,
    delivered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    error        VARCHAR(500) NOT NULL DEFAULT '',
    INDEX idx_deliveries_webhook (webhook_id, delivered_at),
    CONSTRAINT fk_deliveries_webhook FOREIGN KEY (webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS bulk_uploads (
    id         BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id    BIGINT UNSIGNED NOT NULL,
    group_id   BIGINT UNSIGNED NOT NULL,
    filename   VARCHAR(255) NOT NULL,
    status     ENUM('queued','processing','done','failed') NOT NULL DEFAULT 'queued',
    total      INT NOT NULL DEFAULT 0,
    processed  INT NOT NULL DEFAULT 0,
    succeeded  INT NOT NULL DEFAULT 0,
    failed     INT NOT NULL DEFAULT 0,
    source_csv MEDIUMTEXT NOT NULL,
    result_csv MEDIUMTEXT NULL,
    error      VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_bulk_group (group_id, created_at),
    INDEX idx_bulk_status (status, created_at),
    CONSTRAINT fk_bulk_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);
```

Deviations from spec §6.1, ruled: `webhooks.failing_since` and `last_delivery_at` (needed for the 24 h deactivation rule and the list's "last delivery" column); `bulk_uploads.source_csv`, `processed`, `error` (the worker needs the rows; the UI shows processed/total).

**Models (`internal/model`):** `Campaign{ID, UserID, GroupID, Name, Description, AutoUTM bool, CreatedAt, UpdatedAt}`; `Channel{ID, GroupID, Name, CreatedAt}`; `CampaignLink{CampaignID, ChannelID, LinkID, CreatedAt}`; `Webhook{ID, UserID, GroupID, Name, URL, Event, Secret, IsActive, Status, ConsecutiveFailures int, FailingSince NullTime, LastDeliveryAt NullTime, CreatedAt, UpdatedAt}` with constants `WebhookStatusActive/Alert/Deactivated`; `WebhookDelivery{ID, WebhookID, EventID, StatusCode int, Attempt int, DeliveredAt, Error}`; `BulkUpload{ID, UserID, GroupID, Filename, Status, Total, Processed, Succeeded, Failed int, SourceCSV string `json:"-"`, ResultCSV *string `json:"-"`, Error, CreatedAt, UpdatedAt}` with constants `BulkStatusQueued/Processing/Done/Failed`. All json tags snake_case; `Webhook.Secret` has json tag `secret` (owner-only responses).

**Repository interfaces (`repository/interfaces.go`):**

```go
type CampaignRepository interface {
    Create(ctx, c *model.Campaign) error
    GetByID(ctx, id uint64) (*model.Campaign, error)                      // ErrCampaignNotFound
    ListByGroup(ctx, groupID uint64) ([]model.CampaignSummary, error)     // newest first
    Update(ctx, c *model.Campaign) error
    Delete(ctx, id uint64) error
    // Channels (group-scoped, shared by campaigns)
    CreateChannel(ctx, ch *model.Channel) error                           // ErrChannelExists on dup name
    ListChannels(ctx, groupID uint64) ([]model.Channel, error)
    GetChannel(ctx, id uint64) (*model.Channel, error)                    // ErrChannelNotFound
    SetCampaignChannels(ctx, campaignID uint64, channelIDs []uint64) error // replaces the set; memberships of removed channels cascade
    CampaignChannels(ctx, campaignID uint64) ([]model.Channel, error)
    // Membership
    AddLinks(ctx, campaignID, channelID uint64, linkIDs []uint64) error   // INSERT IGNORE
    RemoveLink(ctx, campaignID, channelID, linkID uint64) error
    Links(ctx, campaignID uint64) ([]model.CampaignLink, error)
    LinkCampaigns(ctx, linkID uint64) ([]model.CampaignLink, error)
}
// CampaignSummary = Campaign + ChannelCount, LinkCount int (COUNT DISTINCT via LEFT JOINs).
type WebhookRepository interface {
    Create(ctx, w *model.Webhook) error
    GetByID(ctx, id uint64) (*model.Webhook, error)                       // ErrWebhookNotFound
    ListByGroup(ctx, groupID uint64) ([]model.Webhook, error)
    ListActiveByGroup(ctx, groupID uint64) ([]model.Webhook, error)       // is_active AND status <> 'deactivated'
    Update(ctx, w *model.Webhook) error                                   // name, url, is_active, status, consecutive_failures, failing_since, last_delivery_at
    Delete(ctx, id uint64) error
    AddDelivery(ctx, d *model.WebhookDelivery) error                      // then keeps the newest 50 rows per webhook
    ListDeliveries(ctx, webhookID uint64, limit int) ([]model.WebhookDelivery, error) // newest first
}
type BulkUploadRepository interface {
    Create(ctx, b *model.BulkUpload) error
    GetByID(ctx, id uint64) (*model.BulkUpload, error)                    // ErrBulkUploadNotFound
    ListByGroup(ctx, groupID uint64) ([]model.BulkUpload, error)          // newest first, SourceCSV/ResultCSV not selected
    ClaimNext(ctx) (*model.BulkUpload, error)                             // oldest queued → processing, atomically (UPDATE … WHERE status='queued' ORDER BY created_at LIMIT 1 then SELECT); nil,nil when none
    UpdateProgress(ctx, id uint64, processed, succeeded, failed int) error
    Finish(ctx, id uint64, status string, resultCSV string, errMsg string) error
}
```

`LinkFilter.CampaignID` (link_repo.go) becomes effective: `EXISTS (SELECT 1 FROM campaign_links cl WHERE cl.link_id = l.id AND cl.campaign_id = ?)`.

**Services:**

```go
// campaign_service.go
type CampaignServiceDeps struct { Campaigns repository.CampaignRepository; Links repository.LinkRepository; LinkSvc LinkService; Clicks repository.ClickRepository; Activity ActivityService }
func NewCampaignService(d CampaignServiceDeps) *CampaignServiceImpl
type CampaignInput struct { Name string; Description string; AutoUTM bool; ChannelIDs []uint64; NewChannels []string }
type CampaignDetail struct { model.Campaign; Channels []ChannelDetail }
type ChannelDetail struct { model.Channel; Links []CampaignLinkItem }
type CampaignLinkItem struct { model.LinkWithStats; ChannelID uint64; ChannelName string; ShortURL string }
type CreateLinkInChannelInput struct { OriginalURL string; Title string; CustomCode string; DomainID *uint64; Tags []string }
type CampaignStats struct { Totals struct{ Links int; Engagements int64 }; Channels []ChannelStats }
type ChannelStats struct { ChannelID uint64; Name string; Links int; Engagements int64; Series []repository.Bucket }
Create(ctx, userID, groupID, in CampaignInput) (*CampaignDetail, error)   // name 1–120 trimmed, ≤500 desc, ValidationError; NewChannels are created (dedup, 1–80 chars) then added to ChannelIDs
Get(ctx, groupID, id) (*CampaignDetail, error)
List(ctx, groupID) ([]model.CampaignSummary, error)
Update(ctx, groupID, id, in CampaignInput) (*CampaignDetail, error)
Delete(ctx, groupID, id) error
ListChannels(ctx, groupID) ([]model.Channel, error)
CreateChannel(ctx, groupID, name) (*model.Channel, error)
AddLinks(ctx, groupID, campaignID, channelID uint64, linkIDs []uint64) error   // channel must belong to campaign; every link: same group AND source='link' else ValidationError; never touches links
RemoveLink(ctx, groupID, campaignID, channelID, linkID) error
CreateLinkInChannel(ctx, userID, groupID, campaignID, channelID uint64, in CreateLinkInChannelInput) (*model.Link, error)
    // when campaign.AutoUTM: URL = ApplyUTM(url, UTMParams{Medium:"bitly", Source:channel.Name, Campaign:campaign.Name}) before LinkSvc.Create(CreateLinkInput{…, GroupID}); then AddLinks
Stats(ctx, groupID, id, q StatsQuery) (*CampaignStats, error)     // per channel: ClickScope{GroupID, LinkIDs: channel's link ids}; empty channel → zero-filled series via SeriesScope with no ids (return zeros without querying)
LinkMemberships(ctx, groupID, linkID) ([]LinkMembership, error)   // LinkMembership{CampaignID, CampaignName, ChannelID, ChannelName}

// webhook_service.go
type WebhookServiceDeps struct { Webhooks repository.WebhookRepository; Activity ActivityService; Client *http.Client; Resolver DNSResolver }
func NewWebhookService(d WebhookServiceDeps) *WebhookServiceImpl
type WebhookInput struct { Name string; URL string; Event string }  // Event must be "engagement"
Create(ctx, userID, groupID, in) (*model.Webhook, error)           // secret = 32 random bytes hex (64 chars); URL validated by ValidateWebhookURL
Get/List/Update(ctx, groupID, id, in)/Delete
Deactivate(ctx, groupID, id) (*model.Webhook, error)               // is_active=false, status=deactivated
Activate(ctx, groupID, id) (*model.Webhook, error)                 // is_active=true, status=active, failures=0, failing_since=NULL
Verify(ctx, groupID, id) (*model.WebhookDelivery, error)           // POSTs a signed {"event_type":"ping","webhook_id":id,"datetime":RFC3339} with 10 s timeout, records a delivery (event_id "ping-<unix>"), applies the same status rules as the dispatcher, returns the delivery
Deliveries(ctx, groupID, id) ([]model.WebhookDelivery, error)      // last 20
// webhook_url.go
func ValidateWebhookURL(ctx, raw string, r DNSResolver) error      // http/https only, host required, no userinfo, resolves ≥1 IP and none loopback/link-local/private/unspecified/multicast → ValidationError "URL must be a public http(s) address"
func SignBody(secret string, body []byte) string                    // lowercase hex HMAC-SHA256
// Shared status rule (exported helper used by Verify and the dispatcher):
func ApplyDeliveryResult(w *model.Webhook, ok bool, now time.Time)  // ok: status=active, failures=0, failing_since=NULL; fail: failures++, failing_since=now if NULL; failures>=2 → alert; failing_since older than 24h → deactivated + is_active=false; always last_delivery_at=now

// bulk_service.go
type BulkServiceDeps struct { Bulk repository.BulkUploadRepository; Activity ActivityService }
type BulkRow struct { Line int; LongURL, Title, CustomBackHalf, Domain string; Tags []string }
func ParseBulkCSV(r io.Reader) ([]BulkRow, error)                   // header exactly long_url,title,custom_back_half,tags,domain (case-insensitive, order fixed); ≤1000 data rows else ValidationError "at most 1000 rows"; tags split on "|" trimmed; blank lines skipped
Create(ctx, userID, groupID, filename string, csvBytes []byte) (*model.BulkUpload, error) // size ≤ 2 MiB (ValidationError), parse (ValidationError on bad header / too many rows), status queued, total=len(rows), activity bulk.create
List(ctx, groupID) ([]model.BulkUpload, error)
Get(ctx, groupID, id) (*model.BulkUpload, error)
Results(ctx, groupID, id) (string, error)                            // ErrBulkNotFinished (409) until done/failed
```

**Workers (`internal/worker`):**

```go
// webhook_dispatcher.go
const WebhookQueueKey = "webhooks:queue"       // Redis ZSET, score = unix seconds not-before, member = JSON WebhookJob
type WebhookJob struct { WebhookID uint64; EventID string; Attempt int; Payload json.RawMessage }
type EngagementPayload struct { EventID string `json:"event_id"`; EventType string `json:"event_type"`; Datetime string `json:"datetime"`; Bitlink string `json:"bitlink"`; LongURL string `json:"long_url"`; Referrer, Country, DeviceType, OS, Source string; Tags []string; GroupID uint64 }  // json keys per spec §6.3
type WebhookEnqueuer struct{ … }   // NewWebhookEnqueuer(rdb, webhookRepo, linkRepo, domainRepo, baseURL)
func (e *WebhookEnqueuer) Enqueue(ctx, clicks []model.Click)   // groups clicks by link, loads links+tags+domains once, for each active webhook of the link's group ZADD one job per click (attempt 1, score now); errors logged, never returned to the flusher
type WebhookDispatcher struct{ … }  // NewWebhookDispatcher(rdb, webhookRepo, client *http.Client, pollEvery time.Duration); Start()/Stop() like ClickFlusher
// run loop: every pollEvery, ZRANGEBYSCORE 0..now LIMIT 100; for each job ZREM then deliver: POST job.Payload, headers Content-Type application/json, X-Signature: SignBody(secret, body), X-Webhook-Id, X-Event-Id; 10 s timeout; 2xx = ok. Record delivery (status code, attempt, error ≤500 chars). ApplyDeliveryResult + Update. On failure and attempt < 5: ZADD again with score now + backoff[attempt] where backoff = {1: 60s, 2: 5m, 3: 30m, 4: 2h}. Webhooks not active (deleted/deactivated) drop the job.
// bulk_worker.go
type BulkWorker struct{ … }  // NewBulkWorker(bulkRepo, linkSvc LinkService, domainRepo, pollEvery); Start()/Stop()
// loop: ClaimNext; parse SourceCSV; for batches of 50 rows: for each row resolve domain by name among the user's verified domains (empty → group default via nil DomainID; unknown/unverified → row error "unknown domain"), LinkSvc.Create(CreateLinkInput{OriginalURL, Title, CustomCode: CustomBackHalf, DomainID, Tags, GroupID}); collect result rows; UpdateProgress after each batch; Finish(done, resultCSV) — result CSV header `row,long_url,short_url,status,error`, status ok|error, cells through csvSafe. A panic or DB error → Finish(failed, "", err).
```

The click flusher gets an optional `SetEnqueuer(e interface{ Enqueue(ctx, []model.Click) })` called after a successful `BatchInsert` (before LTrim). Both workers are started in `main.go` next to the click flusher; config gains `Webhooks{PollEvery time.Duration "poll_every" default 2s}` and `Bulk{PollEvery default 5s, BatchSize default 50}` under `workers:`.

**Endpoints (handlers, all under `/api`, auth + group middleware):**

- `GET /campaigns` → `{campaigns: [CampaignSummary + engagements? no — engagements come from the list query: add `Engagements int64` computed by `ClickRepository.CountScope` per campaign in the service, all-time]}`; `POST /campaigns {name, description, auto_utm, channel_ids[], new_channels[]}` → 201 CampaignDetail; `GET /campaigns/:id` → CampaignDetail; `PUT /campaigns/:id` same body; `DELETE /campaigns/:id` → 204.
- `GET /channels` → `{channels: []}`; `POST /channels {name}` → 201 (409 on duplicate).
- `POST /campaigns/:id/links {channel_id, link_ids[]}` → 204; `DELETE /campaigns/:id/links {channel_id, link_id}` → 204; `POST /campaigns/:id/channels/:cid/links {original_url, title, custom_code, domain_id, tags[]}` → 201 link (the link presenter response shape used by `POST /links`); `GET /campaigns/:id/stats?from&to&unit` → CampaignStats; `GET /links/:id/campaigns` → `{memberships: []}`.
- `GET /webhooks` → `{webhooks: []}` (with secret); `POST /webhooks {name, url, event}` → 201; `GET/PUT/DELETE /webhooks/:id`; `POST /webhooks/:id/verify` → 200 `{delivery, webhook}`; `POST /webhooks/:id/deactivate`, `POST /webhooks/:id/activate` → webhook; `GET /webhooks/:id/deliveries` → `{deliveries: []}`.
- `POST /bulk-uploads` multipart field `file` → 201 BulkUpload; `GET /bulk-uploads` → `{uploads: []}`; `GET /bulk-uploads/:id` → BulkUpload; `GET /bulk-uploads/:id/results` → `text/csv` attachment `bulk-<id>-results.csv` (409 while not finished).
- Error mapping: `ValidationError` → 400; not-found errors → 404; `ErrChannelExists` → 409; `ErrBulkNotFinished` → 409.

**Frontend types (`src/types/index.ts`):** `Campaign`, `CampaignSummary` (+`channel_count`, `link_count`, `engagements`), `Channel`, `CampaignDetail` (`channels: ChannelDetail[]`), `ChannelDetail` (`links: CampaignLinkItem[]`), `CampaignLinkItem` (`Link` fields + `channel_id`, `channel_name`), `CampaignInput`, `CreateLinkInChannelInput`, `CampaignStats`, `ChannelStats`, `LinkMembership`, `Webhook` (`status: 'active'|'alert'|'deactivated'`), `WebhookInput`, `WebhookDelivery`, `BulkUpload` (`status: 'queued'|'processing'|'done'|'failed'`). Query keys: `campaigns(groupId)`, `campaign(id)`, `campaignStats(id, query)`, `channels(groupId)`, `linkCampaigns(linkId)`, `webhooks(groupId)`, `webhookDeliveries(id)`, `bulkUploads(groupId)`.

---

### Task 1: Migration, models, repositories (campaigns, webhooks, bulk uploads), link filter

**Files:**
- Create: `backend/migrations/009_campaigns_webhooks_bulk.sql`, `backend/internal/model/campaign.go`, `model/webhook.go`, `model/bulk_upload.go`, `repository/campaign_repo.go` + `_test.go`, `repository/webhook_repo.go` + `_test.go`, `repository/bulk_upload_repo.go` + `_test.go`
- Modify: `repository/interfaces.go` (three interfaces + errors `ErrCampaignNotFound`, `ErrChannelNotFound`, `ErrChannelExists`, `ErrWebhookNotFound`, `ErrBulkUploadNotFound`), `repository/link_repo.go` (`CampaignID` filter), `internal/testutil/db.go` (add `webhook_deliveries, webhooks, bulk_uploads, campaign_links, campaign_channels, channels, campaigns` before `links`), `repository/mocks` via `go generate`

**Interfaces:** Produces everything under "Repository interfaces" and "Models" above.

- [ ] **Step 1: Write the migration** exactly as in Shared contracts; apply to both local databases; run `go test ./internal/database/...` (migration runner test) to confirm it splits.
- [ ] **Step 2: Write failing repo tests** `TestCampaignRepo_CRUDAndSummary` (create 2 channels, campaign with both, add 2 links to one channel → summary ChannelCount 2, LinkCount 2; `SetCampaignChannels` to one channel drops the other channel's memberships), `TestCampaignRepo_Cascades` (delete link → membership gone; delete campaign → memberships gone, links remain), `TestCampaignRepo_ChannelUniquePerGroup` (same name twice → `ErrChannelExists`; same name in another group OK), `TestLinkRepo_FilterByCampaign` (ListByGroup with `CampaignID` returns only members), `TestWebhookRepo_DeliveriesKeepNewest50` (insert 55 → 50 remain, `ListDeliveries(limit 20)` newest first), `TestWebhookRepo_ListActiveExcludesDeactivated`, `TestBulkRepo_ClaimNextIsAtomic` (two queued jobs, two sequential claims return distinct ids, third returns nil; status processing), `TestBulkRepo_ListOmitsCSVBodies`.
- [ ] **Step 3: Run tests** → FAIL (undefined symbols).
- [ ] **Step 4: Implement models, repositories, filter, testutil list.** `AddDelivery` prunes with `DELETE FROM webhook_deliveries WHERE webhook_id = ? AND id NOT IN (SELECT id FROM (SELECT id FROM webhook_deliveries WHERE webhook_id = ? ORDER BY id DESC LIMIT 50) t)`. `ClaimNext`: `UPDATE bulk_uploads SET status='processing' WHERE status='queued' ORDER BY created_at, id LIMIT 1` in a transaction, then `SELECT … WHERE id = LAST_INSERT_ID()` is not available — instead `UPDATE … SET id = LAST_INSERT_ID(id), status='processing' …` then `SELECT LAST_INSERT_ID()`; 0 rows → nil, nil.
- [ ] **Step 5: `go generate ./...`, `go vet ./...`, full Go suite** → PASS.
- [ ] **Step 6: Commit** `feat(phase5): campaigns, webhooks, bulk upload schema and repositories`.

### Task 2: CampaignService

**Files:**
- Create: `service/campaign_service.go` + `_test.go`
- Modify: `service/interfaces.go` (`CampaignService` interface), `service/activity_service.go` (campaign actions), `service/mocks` via `go generate`

**Interfaces:** Consumes Task 1 repos, `LinkService.Create`, `ApplyUTM` (`service/utm.go`), `ClickRepository.CountScope/SeriesScope`, `NormalizeStatsQuery`. Produces the CampaignService API in Shared contracts.

- [ ] **Step 1: Failing tests (mocks):** `TestCampaignService_Create_ValidatesAndCreatesChannels` (blank name → ValidationError; `NewChannels: ["Email"," email "]` creates one channel; channel ids set), `TestCampaignService_WrongGroup` (Get/Update/Delete/AddLinks with a campaign of group 2 → `ErrCampaignNotFound`), `TestCampaignService_AddLinks_ForeignLink` (link of another group → ValidationError; `source='qr'` → ValidationError; channel not in campaign → ValidationError), `TestCampaignService_AddLinks_NeverRewritesURL` (asserts `Links.Update` is never called), `TestCampaignService_CreateLinkInChannel_AutoUTM` (auto_utm on: `LinkSvc.Create` receives `https://x.com/?utm_medium=bitly&utm_source=Email&utm_campaign=Spring` and `GroupID`; off: URL unchanged; then AddLinks called with the new id), `TestCampaignService_Stats_PerChannel` (two channels; SeriesScope called with each channel's ids; empty channel yields zero series without a repo call; totals sum), `TestCampaignService_Delete_LogsActivity`.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** per Shared contracts; engagements in `List` via `CountScope` over all member link ids per campaign (all-time: from `time.Unix(0,0)` to now+1 day) — one call per campaign is acceptable.
- [ ] **Step 4: Tests + vet** → PASS. **Step 5: Commit** `feat(campaigns): campaign service`.

### Task 3: Campaign handlers and routes

**Files:**
- Create: `handler/campaign.go` + `_test.go`
- Modify: `cmd/server/main.go` (wire `NewCampaignService`, routes), `handler/link.go` (`GET /links/:id/campaigns`)

- [ ] **Step 1: Failing handler tests** with mocked service: `TestCampaignHandler_CreateValidation400`, `TestCampaignHandler_NotFound404`, `TestCampaignHandler_AddLinks204`, `TestCampaignHandler_CreateLinkInChannel201` (response uses the link presenter: `short_url` present), `TestCampaignHandler_Stats_BadRange400`, `TestChannelHandler_Duplicate409`, `TestLinkHandler_Campaigns`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement** `RegisterCampaignRoutes(r *gin.RouterGroup, h *CampaignHandler)` mirroring `handler/page.go` style (`writeCampaignError`). **Step 4: Suite** → PASS. Restart the dev backend. **Step 5: Commit** `feat(campaigns): handlers and routes`.

### Task 4: WebhookService (CRUD, URL validation, verify ping, status rules)

**Files:**
- Create: `service/webhook_service.go` + `_test.go`, `service/webhook_url.go` + `_test.go`
- Modify: `service/interfaces.go` (`WebhookService`), `service/activity_service.go` (webhook actions), mocks

- [ ] **Step 1: Failing tests:** `TestValidateWebhookURL` (table: `ftp://x` ✗, `http://user:pw@x` ✗, `http://localhost/h` ✗, `http://10.0.0.5/h` ✗, `http://169.254.1.1` ✗, `https://hooks.example.test/x` with resolver returning `93.184.216.34` ✓, resolver error ✗), `TestSignBody` (known vector: secret `s3cret`, body `{"a":1}` → the hex you compute with `openssl dgst -sha256 -hmac s3cret`), `TestApplyDeliveryResult` (ok resets; two failures → alert; failing_since 25 h ago → deactivated + inactive; success after alert → active), `TestWebhookService_Create_GeneratesSecret` (64 hex chars, event must be engagement), `TestWebhookService_WrongGroup`, `TestWebhookService_Verify_RecordsDelivery` (httptest server returning 204 → delivery status 204, webhook active; server 500 → failures 1; header `X-Signature` equals SignBody(secret, body) and body `event_type` is `ping`), `TestWebhookService_Verify_Timeout` (server sleeps > client timeout set to 50 ms in test → delivery error non-empty, status_code 0).
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement** (`DNSResolver` is the existing interface in `domain_service.go`; the http.Client in deps has `Timeout: 10s` and `CheckRedirect` limiting to 3 hops in `main.go`). **Step 4: Suite** → PASS. **Step 5: Commit** `feat(webhooks): webhook service`.

### Task 5: Webhook enqueue + dispatcher worker, handlers, routes

**Files:**
- Create: `worker/webhook_dispatcher.go` + `_test.go`, `handler/webhook.go` + `_test.go`
- Modify: `worker/click_flusher.go` (`SetEnqueuer`), `config/config.go` (+ `config_test.go`: `workers.webhooks.poll_every` default 2s, `workers.bulk.poll_every` 5s, `workers.bulk.batch_size` 50), `cmd/server/main.go`

- [ ] **Step 1: Failing tests:** `TestEnqueuer_OneJobPerClickPerActiveWebhook` (miniredis or the real local Redis DB 15: 2 clicks on a link, 2 active + 1 deactivated webhooks → 4 ZSET members; payload fields `bitlink` = short URL, `long_url`, `tags`, `group_id`, `source`), `TestDispatcher_SignsBody` (httptest server captures headers; `X-Signature` matches; `X-Event-Id` set), `TestDispatcher_RetriesWithBackoff` (server 500 → job re-added with score ≈ now+60 s and attempt 2; after attempt 5 not re-added), `TestDispatcher_Timeout` (client timeout 50 ms, server sleeps 200 ms → delivery recorded with error), `TestDispatcher_DropsDeactivated`, `TestWebhookHandler_*` (create 201 with secret, list, verify 200, deactivate/activate, deliveries, 404 wrong group, 400 private URL).
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement** per Shared contracts; wire in `main.go`: `clickFlusher.SetEnqueuer(worker.NewWebhookEnqueuer(rdb, webhookRepo, linkRepo, domainRepo, cfg.URLs.BaseURL))`, start `WebhookDispatcher`, stop on shutdown next to the flusher; register `RegisterWebhookRoutes`. **Step 4: Suite** → PASS. Restart dev backend. **Step 5: Commit** `feat(webhooks): dispatcher worker, handlers and routes`.

### Task 6: Bulk upload service, worker, handlers

**Files:**
- Create: `service/bulk_service.go` + `_test.go`, `worker/bulk_worker.go` + `_test.go`, `handler/bulk_upload.go` + `_test.go`
- Modify: `service/interfaces.go`, `service/activity_service.go` (`bulk.create`), `cmd/server/main.go`, mocks

- [ ] **Step 1: Failing tests:** `TestParseBulkCSV` (good file with 3 rows incl. quoted title with comma and tags `a|b`; wrong header → ValidationError; 1001 rows → ValidationError "at most 1000 rows"; blank line skipped; `Line` numbers are 1-based data rows), `TestBulkService_ParseLimits` (2 MiB + 1 byte → ValidationError), `TestBulkService_WrongGroup`, `TestBulkService_Results_NotFinished409`, `TestBulkWorker_RowErrorsDoNotAbort` (mock LinkSvc: row 2 returns `ErrShortCodeExists`-style error, rows 1 and 3 succeed → succeeded 2, failed 1, result CSV has 3 rows with statuses, `UpdateProgress` called once per batch), `TestBulkWorker_BatchesOf50` (120 rows → 3 progress updates), `TestBulkWorker_UnknownDomainRowError`, `TestBulkWorker_CSVSafe` (a `=cmd` title appears prefixed in the result CSV), `TestBulkHandler_Upload201`, `TestBulkHandler_Results_CSVAttachment`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement**; `POST /bulk-uploads` uses `c.FormFile("file")`, reads ≤ 2 MiB + 1 (`io.LimitReader`), filename sanitised to base name ≤ 255. **Step 4: Suite** → PASS. Restart dev backend. **Step 5: Commit** `feat(bulk): bulk upload service, worker, handlers`.

### Task 7: Frontend types, API client, hooks, activity verbs

**Files:**
- Create: `src/lib/hooks/useCampaigns.ts`, `useWebhooks.ts`, `useBulkUploads.ts` (+ `useBulkUploads.test.tsx`)
- Modify: `src/types/index.ts`, `src/lib/api.ts`, `src/lib/hooks/{index,keys}.ts`, `src/components/settings/activity.ts` (+ test: `webhook.deactivate` → "Deactivated webhook", `bulk.create` → "Created bulk upload")

**Interfaces:** Produces hooks: `useCampaigns()`, `useCampaign(id)`, `useCampaignStats(id, query)`, `useChannels()`, `useCampaignMutations()` → `{create, update, remove, addLinks, removeLink, createLinkInChannel, createChannel}`, `useLinkCampaigns(linkId)`; `useWebhooks()`, `useWebhookDeliveries(id, enabled)`, `useWebhookMutations()` → `{create, update, remove, verify, deactivate, activate}`; `useBulkUploads()` (refetchInterval 3000 while any job is queued/processing, else false), `useUploadBulkCsv()` (`api.uploadBulkCsv(file)` posts FormData), `api.bulkResultsUrl(id)` not needed — results are fetched via `api.downloadBulkResults(id)` returning a Blob and saved with the existing `src/lib/download.ts` helper. API methods mirror the endpoints in Shared contracts.

- [ ] Steps: failing hook test for the bulk polling flag → implement types/api/hooks → `vitest`, `lint`, `tsc` → commit `feat(frontend): phase 5 types, api and hooks`.

### Task 8: Campaigns list page and create/edit dialog

**Files:**
- Create: `src/components/campaigns/CampaignDialog.tsx` (+ test), `CampaignCard.tsx`, `ChannelPicker.tsx` (+ test)
- Modify: `src/app/dashboard/campaigns/page.tsx` (replace ComingSoon)

- [ ] List: `PageHeader` "Campaigns" with primary "Create campaign"; cards (name as link to detail, description, "{n} channels · {m} links · {e} engagements", created date) with ⋯ "Edit" / "Delete" (ConfirmDialog "Delete {name}?"); empty state "Create your first campaign". Dialog: Name (required), Description, Channels (`ChannelPicker`: multi-select of `useChannels()` + "Create new channel" inline input → `createChannel` then selected), Checkbox "Auto-add UTM parameters" with helper text "Adds utm_medium=bitly, utm_source={channel} and utm_campaign={name} to links created from a channel". Tests: `ChannelPicker` creates and selects; `CampaignDialog` submit payload shape `{name, description, auto_utm, channel_ids, new_channels: []}`.
- [ ] Checks → commit `feat(campaigns): list page and dialog`.

### Task 9: Campaign detail: Channels tab, Links tab, Performance

**Files:**
- Create: `src/app/dashboard/campaigns/[id]/page.tsx`, `src/components/campaigns/{ChannelCard.tsx, AddLinkToChannelDialog.tsx (+ test), CampaignLinksTable.tsx, CampaignPerformance.tsx (+ test)}`

- [ ] Header: name, description, "Edit" (reuses `CampaignDialog`), `Tabs` "Channels" | "Links". Channels tab: one `ChannelCard` per channel (name, link count, engagements, list of links with short URL + clicks + ⋯ "Remove from channel"), button "Add link to channel" → `AddLinkToChannelDialog` with two tabs "Pick existing" (searchable list from `useLinks({q})`, multi-select, calls `addLinks`) and "Create link" (destination URL, title, optional back-half; shows the note "UTM parameters will be added" when `auto_utm`; calls `createLinkInChannel`). Links tab: `CampaignLinksTable` columns Link (title + short URL), Channel, Engagements, Created. "Performance" card below the tabs: date preset select (reuse Phase 3 preset list and `rangeFor`), `TimeSeriesChart` with `variant='line'` and one line per channel (build `extraSeries`-style data: pass channel 0 as `series`, and render additional channels by composing the chart's `data` — if `TimeSeriesChart` cannot take N series, add an optional `multiSeries?: {name: string; points: SeriesPoint[]}[]` prop to it in this task with a test), and a summary table Channel · Links · Engagements. Tests: dialog payloads; performance renders one legend entry per channel.
- [ ] Checks → commit `feat(campaigns): detail page with channels, links and performance`.

### Task 10: Link touchpoints: "Add to campaign" and Campaign filter

**Files:**
- Create: `src/components/links/AddToCampaignDialog.tsx` (+ test)
- Modify: `src/components/links/LinkActionsMenu.tsx` (replace the disabled item), `FiltersDialog.tsx` (+ its test: Campaign `Select` listing `useCampaigns()` with "All campaigns"), `linksUrlState.ts` (`campaign_id` round-trips), `src/app/dashboard/links/[id]/page.tsx` (small "Campaigns" card listing `useLinkCampaigns` memberships as "{campaign} · {channel}", or "Not in a campaign")

- [ ] Dialog: Campaign select → Channel select (channels of the chosen campaign) → "Add"; calls `addLinks({campaignId, channelId, linkIds:[link.id]})`; toast "Added to {campaign}". Tests: selects cascade; payload. Filters: applied value appears in the toolbar chip count. Checks → commit `feat(links): add to campaign and campaign filter`.

### Task 11: Webhooks settings page

**Files:**
- Create: `src/components/settings/{WebhookDialog.tsx (+ test), WebhookRow.tsx, DeliveriesPanel.tsx (+ test)}`
- Modify: `src/app/dashboard/settings/webhooks/page.tsx`

- [ ] Page: header "Webhooks" + "Create webhook"; table Name · URL · Event · Status chip (Active = success, Alert = warning, Disabled = muted; `deactivated` renders "Disabled") · Last delivery (relative time or "—"); row ⋯: "Verify" (calls verify; toast "Ping delivered ({status})" or the error), "Edit", "Deactivate"/"Activate", "Delete" (confirm); clicking a row toggles `DeliveriesPanel` beneath it (last 20: status code badge, attempt, time, error). Dialog fields: Name, URL, Event (select with the single option "engagement"), Group (read-only: active group name), and after create a one-time "Secret" block with `CopyButton` + text "Use this to verify the X-Signature header (HMAC-SHA256 of the body)". Tests: dialog payload; panel renders rows and the empty text "No deliveries yet". Checks → commit `feat(settings): webhooks page`.

### Task 12: Bulk upload page and domain "Set as default for group"

**Files:**
- Create: `src/components/settings/{BulkUploadForm.tsx (+ test), BulkJobRow.tsx, bulkTemplate.ts (+ test)}`
- Modify: `src/app/dashboard/settings/bulk-upload/page.tsx`, `src/components/domains/DomainRow.tsx` (+ test)

- [ ] Bulk page: header "Bulk upload"; card with "Download template" (`bulkTemplate.ts` exports `BULK_TEMPLATE_CSV` = header line + one example row; downloaded via `download.ts` as `bulk-upload-template.csv`), file input (accept `.csv`, max 2 MB client check), "Upload" → `useUploadBulkCsv`; table of jobs File · Status chip · Progress "{processed}/{total} · {succeeded} ok · {failed} failed" (Progress bar while processing) · Created · "Download results CSV" (enabled when done/failed). Domain row: add ⋯ item "Set as default for group" (disabled with tooltip "Verify the domain first" unless `verification_status === 'ok'`), which calls `useUpdateGroup`-equivalent (`api.updateGroup(activeGroupId, {default_domain_id})`, add `useUpdateGroup` to `useGroups.ts` if missing) and toasts "Default domain updated"; the existing "Default" badge reflects it. Tests: template has the exact header `long_url,title,custom_back_half,tags,domain`; form rejects a 3 MB file; row action payload. Checks → commit `feat(settings): bulk upload page; domains default-for-group action`.

### Task 13: Docs

**Files:** Modify `README.md` (Phase 5 section: campaigns/channels and auto-UTM rule; webhook payload, signature verification snippet, retry/alert/deactivate rules, URL restrictions; bulk CSV format and limits; new `workers:` config; migration 009 under Upgrading), `backend/config.example.yaml` (or the existing example config) with the `workers:` block.

- [ ] Write, proofread against the implemented behaviour, commit `docs: phase 5 campaigns, webhooks, bulk upload`.

---

## Self-review notes

- Spec coverage: §5.9 list/dialog/detail/performance (T8, T9), add-from-link-menu (T10); §5.10 "Set as default for group" (T12); §5.11 Webhooks (T11) and Bulk upload (T12), activity actions (T2/T4/T6/T7); §6.1 tables (T1); §6.2 Campaigns/Webhooks/Bulk endpoints (T3/T5/T6); §6.3 dispatch (T5); §11.5 (T2). Links filter `campaign_id` (T1 + T10).
- Type consistency: `CampaignInput` (T2) == dialog payload (T8); `CreateLinkInChannelInput` (T2) == "Create link" tab (T9); `ApplyDeliveryResult` shared by T4 and T5; `csvSafe` from Phase 3 reused in T6.
- Parallelism: T1 → T2 → T3; T4 → T5; T6 needs T1; T7 needs only this document (contracts); T8–T12 need T7 and may run after their backend task lands for smoke, but their unit tests mock the API. Disjoint-file tasks may run in parallel in the shared checkout.

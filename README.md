# URL Shortener

A self-hosted, Bitly-style URL shortener.

- **backend/** — Go (Gin) API server (`:8081`) and redirect server (`:8080`), MariaDB + Redis.
- **frontend/** — Next.js web UI (`:3000`).

## Quick start (Docker)

```bash
cp .env.example .env   # set DB_PASSWORD / DB_ROOT_PASSWORD
# backend/config.docker.yaml: database.password must match DB_PASSWORD,
# and change jwt.secret
docker compose up -d --build
```

Open http://localhost:3000 and register an account. Short links are served
from `urls.base_url` (http://localhost:8080 by default).

Volumes:

- `backend/config.docker.yaml` → `/app/config.yaml` (read-only)
- `backend/data/` → `/app/data` (read-only; place `GeoLite2-City.mmdb` here for GeoIP)
- named volume `uploads` → `/app/uploads` (`uploads.path`, writable)

Every configuration key is documented in `backend/config.example.yaml`.

## Bitly-style UI (Phase 1)

### Features

- **Home** — quick create: paste a long URL, get a short link immediately.
- **Links list** — search and filters (tags, created date, expiration,
  custom back-half, QR, hidden/visible), bulk actions (edit tags,
  hide/unhide), hide links, CSV export.
- **Create link** — title, custom back-half, tags, UTM parameters,
  expiration date and dynamic routing rules (alternate destinations by
  device, OS or country).
- **Link details** — click analytics with a date range picker and
  period-over-period comparison (clicks over time, referrers, locations,
  devices).
- **Edit link** — change destination, title, tags; back-half history keeps
  previous back-halves listed.
- **Groups** — every link, tag and activity entry belongs to a group;
  switch the active group from the UI. A group can have a default custom
  domain that new links use unless another domain is picked.
- **API access tokens** — personal tokens (`us_...`) for scripted access.
- **Activity log** — what was created, edited, hidden or deleted, per group.
- **Custom domains** — add your own domain, verify it via DNS
  (CNAME to `urls.redirect_host`, or an A record to `urls.redirect_ip` for
  apex domains), and create links on it. Domains belong to the user; an
  optional `group_id` restricts a domain to one group (`null` = usable by
  all of the user's groups). A domain that still has links can't be
  deleted.

### QR Codes (Phase 2)

- **Every QR code owns a link** (`qr_codes.link_id`, unique). A QR created
  from a destination gets a new link with `source='qr'`, hidden from the
  Links list — or `source='link'` when "Share as a short link" is on. A QR
  can also be created for an existing Links-list link (Create QR Code).
  Deleting a link deletes its QR; deleting a QR deletes its `source='qr'`
  link (a Links-list link stays).
- **Scans** are redirects of `GET {short_url}?r=qr`; the redirect server
  records them as clicks with `source='qr'`.
  `GET /api/qr-codes/{id}/stats` is the link stats endpoint restricted to
  QR scans.
- **Endpoints** — `GET/POST /api/qr-codes`, `GET/PUT/DELETE /api/qr-codes/{id}`,
  `PATCH /api/qr-codes/bulk`, `PUT /api/qr-codes/{id}/tags`,
  `GET /api/qr-codes/export`, `GET /api/qr-codes/{id}/stats`, and
  `GET /api/qr-codes/{id}/image?format=png|svg&size=` (plain, unstyled
  render). Styled codes (pattern, colours, logo, frame) are rendered and
  downloaded in the browser.
- **Logos** — `POST /api/uploads` accepts PNG/JPEG up to 5 MB, stores the
  file under `uploads.path` (default `data/uploads`) and the API server
  serves it at `/uploads/{name}`. With Docker, `uploads.path` is
  `/app/uploads`, backed by the writable `uploads` volume.

### Analytics (Phase 3)

- **Pages** — `/dashboard/analytics` (performance overview + dashboards
  preview), `/dashboard/analytics/dashboards` (list: rename, duplicate,
  delete) and `/dashboard/analytics/dashboards/[id]` (modules, change
  visualization, reorder, filter, date range, previous-period comparison,
  Download CSV). Link details has **Compare to a link**.
- **Dashboards** are per group and store a name, `date_range`
  (`today`, `last_7_days`, `last_30_days`, `last_60_days`, `last_90_days`),
  a `filter` and a module `layout`. Custom ranges are URL-only and not saved.
  Each group gets a **Default** dashboard, created on the first list call; it
  can be renamed but deleting it returns `409`. Dashboards of another group
  return `404`.
- **Filter** — `{"type":"all"}`, `{"type":"links","ids":[...]}` or
  `{"type":"qr","ids":[...]}` (1–200 ids from the group, otherwise `400`).
  `links` counts clicks of those links except QR scans; `qr` counts the scans
  of the selected codes. Links owned by a QR code (`source='qr'`) count as QR
  items, not as links. Ids of links or QR codes deleted after the filter was
  saved are dropped when the dashboard is read (the stored row is unchanged).
- **Endpoints** (group-scoped, `/api/analytics`) — `GET /overview`,
  `GET/POST /dashboards`, `GET/PATCH/DELETE /dashboards/{id}`,
  `POST /dashboards/{id}/duplicate`, `GET /dashboards/{id}/data` and
  `GET /dashboards/{id}/export`. `/data` takes `from`, `to` and
  `compare=previous|none` (`/export` takes the same); the dashboard's stored
  filter is always used, and without both `from` and `to` the window is the
  dashboard's stored `date_range` in UTC days (`today` is today only,
  `last_N_days` ends today).
  `GET /api/links/{id}/stats?compare_link={id}` adds the comparison link's
  series to the link stats (QR stats ignore it).
- **Export** — `/export` returns a zip with `summary.csv`, `top.csv`,
  `engagements_over_time.csv`, `devices.csv`, `referrers.csv`,
  `locations.csv` and `links_and_qr_codes.csv`.

### Pages — link-in-bio (Phase 4)

- **Frontend routes** — `/dashboard/pages` (list: search, archived filter,
  Draft/Published badge, views), `/dashboard/pages/new` (claim a URL, pick a
  template) and `/dashboard/pages/[id]` (editor with **Build · Design ·
  Track** tabs). The public page is `/m/[slug]`, server-rendered; its
  server-side fetch uses the `BACKEND_URL` env var (default
  `http://localhost:8081`). The shareable URL is `{urls.base_url}/m/{slug}`;
  the redirect server answers `GET`/`HEAD /m/{slug}` with a 302 to
  `{frontend_url}/m/{slug}` (no lookup — the frontend 404s unknown slugs).
- **Add to a page** — link details (Pages card), the link actions menu, the
  Links list cards and the create link form can add a link to a page as
  a link block; Home's getting-started checklist points at Pages.
- **Slugs** — 3–40 chars, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, globally unique
  and case-insensitive (stored lowercase). Reserved: `api`, `admin`, `m`,
  `dashboard`, `login`, `register`, `static`, `assets`, `health`. `m` is
  also reserved as a **short-link back-half** (existing links named `m` keep
  working but new ones are rejected), because `/m/` is the page prefix.
- **Templates** — `POST /api/pages` takes `template` = `scratch` (default),
  `classic`, `bold` or `minimal`; it sets the initial design, and the
  non-scratch templates add a sample "Welcome" text block.
- **Design JSON** — `{"theme", "background": {"type": "solid|gradient",
  "color", "color2"}, "text_color", "button": {"style": "fill|outline",
  "radius": "square|rounded|pill", "color", "text_color"}, "font":
  "inter|serif|mono"}`; colours are `#RRGGBB`, `theme` is one of `classic`,
  `bold`, `minimal`, `dark`, `sunset`. The shape is validated and only the
  keys above are stored; unknown keys are dropped.
- **Block content JSON** — `link`: `{title (1–80), description (≤160),
  thumbnail_url}` plus `link_id` (a `source='link'` link of the same group)
  or `new_link: {original_url, title?}` to create one on the fly;
  `text`: `{heading (≤120), body (≤1000)}` (at least one); `image`:
  `{url, alt (≤120)}`; `social`: `{items: [{platform, url}]}`, 1–10 items,
  platform `instagram|x|facebook|tiktok|youtube|linkedin|email|website`.
  Deleting a link deletes its blocks.
- **Draft vs published** — edits only change the draft. `POST
  /api/pages/{id}/publish` writes a snapshot (`published_content`) and sets
  `published_at`; `has_unpublished_changes` is true unless the page is
  published and its `revision` equals `published_revision`. Every page or
  block write (and deleting a link one of its blocks uses) bumps
  `revision`; publish records the revision it snapshotted, and archiving
  does not count as a change. The editor waits for pending saves before
  publishing. The public endpoint
  serves only the snapshot (active blocks, pinned first, link blocks carry
  `short_url` with `?r=page`); a draft or never-published page is `404`.
- **Endpoints** (group-scoped, `/api/pages`) — `GET /` (`?q`,
  `?archived=on|off`), `POST /`, `GET /check-slug?slug=` (`available`,
  `reason` = `""|invalid|reserved|taken`), `GET/PUT/PATCH/DELETE /{id}`
  (`PATCH {archived}`), `POST /{id}/publish`, `POST /{id}/blocks`,
  `PUT/DELETE /{id}/blocks/{bid}`, `PUT /{id}/blocks/reorder` (`{ids}` must
  be exactly the page's block ids) and `GET /{id}/stats`. Pages and blocks
  of another group return `404`.
- **Public endpoints** (no auth) — `GET /api/public/pages/{slug}` and
  `POST /api/public/pages/{slug}/view` (`204`; optional `{"referrer"}` body,
  preferred over the `Referer` header; the IP is hashed with the click salt;
  views are not de-duplicated).
- **Stats** — `GET /api/pages/{id}/stats` takes the link-stats parameters
  (`from`, `to`, `unit`, `compare`) and returns views over time, referrers,
  devices, countries, cities and `blocks[]` with `clicks` = clicks with
  `source='page'` on the block's link in the window. Views come from
  `page_views`.
- **Links** — every link JSON object has `page_ids` (non-archived pages
  whose blocks reference it).

### Campaigns, webhooks and bulk upload (Phase 5)

#### Campaigns & channels

- **Pages** — `/dashboard/campaigns` (list, **Create campaign**, Edit,
  Delete) and `/dashboard/campaigns/[id]` (tabs for **Channels**, **Links**
  and **Performance**; **Add link to channel** adds an existing link or
  creates a new one in the channel).
- **What they are** — a campaign has a name (1–120 chars), a description
  (≤500) and **Auto-add UTM parameters**. Channels (name 1–80 chars) belong
  to the group and are shared by its campaigns; a campaign uses the subset
  attached to it. A link joins a campaign *in* a channel.
- **Membership is metadata only** — adding a link to a campaign never
  changes the link or its destination. Removing a channel from a campaign,
  deleting the campaign or deleting the link drops the membership rows;
  links are never deleted.
- **Auto-UTM** applies only to links **created from a channel** (`POST
  /api/campaigns/{id}/channels/{cid}/links`, i.e. **Add link to channel →
  Create link**) in a campaign with auto-UTM on. The destination gets
  `utm_source=<channel name>`, `utm_medium=bitly`, `utm_campaign=<campaign
  name>`, appended in exactly that order; an existing parameter with the
  same name is overwritten in place. Existing links added to a campaign,
  and links added with **Add to campaign**, are never rewritten.
- **Links list** — the Links filter dialog has a **Campaign** filter
  (`GET /api/links?campaign_id=`), and a link's ⋯ menu has **Add to
  campaign** (pick campaign and channel). `GET /api/links/{id}/campaigns`
  lists a link's memberships. One request adds at most 500 links.
- **Endpoints** (group-scoped) — `GET/POST /api/campaigns`,
  `GET/PUT/DELETE /api/campaigns/{id}` (`PUT` replaces the channel set; an
  empty `channel_ids` means none), `POST/DELETE /api/campaigns/{id}/links`,
  `POST /api/campaigns/{id}/channels/{cid}/links`, `GET
  /api/campaigns/{id}/stats` (takes the link-stats `from`/`to`/`unit`
  parameters; engagements per channel) and `GET/POST /api/channels`. Other
  groups' ids return `404`.

#### Custom domains: default for group

In **Settings → Custom domains**, a verified domain's ⋯ menu has **Set as
default for group** (disabled with a "Verify the domain first" tooltip until
verified). Links created in that group without an explicit domain — including
bulk-upload rows with an empty `domain` — then use it.

#### Webhooks

- **Page** — **Settings → Webhooks**: **Create webhook** (name, URL, event
  `engagement`), **Verify**, **Deactivate**, Edit, Delete, a status badge
  (**Active**, **Alert**, **Disabled**) and the recent deliveries.
- **Secret** — a 64-hex-character signing secret is generated on create and
  the dialog shows it **once**; copy it then. (The API's webhook objects do
  include `secret` for every member of the group, so treat group access
  accordingly.)
- **Event** — `engagement` fires for every stored click on any link of the
  webhook's group, including QR scans. Events are queued when the click
  buffer is flushed to the database, so they follow the click by a few
  seconds. Requests are `POST` with `Content-Type: application/json` and
  these headers: `X-Signature`, `X-Webhook-Id`, `X-Event-Id` (same as the
  payload's `event_id`) and `User-Agent: urlshortener-webhooks/1.0`.
- **Payload**

  ```json
  {
    "event_id": "9f1c2e7a5b0d4c3e8a6f1b2c3d4e5f60",
    "event_type": "engagement",
    "datetime": "2026-10-25T14:03:22Z",
    "bitlink": "https://sho.rt/promo",
    "long_url": "https://example.com/landing?utm_source=newsletter",
    "referrer": "https://news.example.org/",
    "country": "DE",
    "device_type": "mobile",
    "os": "iOS",
    "source": "link",
    "tags": ["promo", "q4"],
    "group_id": 3
  }
  ```

  `datetime` is the click time in UTC (RFC 3339); `bitlink` is the short URL
  (on the link's custom domain when it has one); `source` is `link`, `qr` (a QR
  scan) or `page` (a click from a link-in-bio page); `tags` is `[]`
  when the link has none.
- **Verify the signature** — `X-Signature` is the lowercase hex
  HMAC-SHA256 of the **raw request body** keyed with the secret. Compare in
  constant time against the bytes you received, before parsing:

  ```python
  import hmac, hashlib

  def valid(secret: str, raw_body: bytes, header: str) -> bool:
      expected = hmac.new(secret.encode(), raw_body, hashlib.sha256).hexdigest()
      return hmac.compare_digest(expected, header)
  ```

  ```js
  const crypto = require('crypto')
  const expected = crypto.createHmac('sha256', secret).update(rawBody).digest('hex')
  const ok = expected.length === sig.length &&
    crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(sig))
  ```
- **Verify button** — `POST /api/webhooks/{id}/verify` sends a signed ping
  `{"event_type":"ping","webhook_id":<id>,"datetime":"…"}` with
  `X-Event-Id: ping-<unix seconds>`, records the delivery and returns it. It
  counts like a real delivery for the status rules below, except that it
  never re-enables a deactivated webhook.
- **Delivery and retries** — a delivery succeeds on any `2xx` response
  within 10 s (at most 3 redirects). Anything else, including timeouts and
  connection errors, is a failure and is retried after **60 s, 5 min, 30 min
  and 2 h** — 5 attempts in total, then the event is dropped.
- **Status rules** — a success resets the webhook to **Active**. From the
  second consecutive failure the status is **Alert**. When failures have
  continued for more than 24 h since the first one, the webhook is
  **deactivated** (**Disabled**): queued retries for it are discarded and
  new events are not queued. **Activate** (`POST /api/webhooks/{id}/activate`)
  re-enables it and resets the counters; **Deactivate** pauses it manually.
- **URL restrictions** — public `http`/`https` URLs only, at most 2048
  characters, no credentials in the URL, not `localhost`. The host (an IP
  literal, or every address it resolves to) must be a public unicast
  address: private (`10/8`, `172.16/12`, `192.168/16`), loopback, link-local
  (`169.254/16`), CGNAT (`100.64/10`), unspecified, multicast, reserved and
  their IPv6 equivalents (including IPv4-mapped, NAT64 and 6to4 addresses
  that embed a non-public IPv4) are rejected with `400` ("URL must be a
  public http(s) address"). This is checked when you save the webhook, and
  again at connect time: the delivery client resolves and then refuses to
  dial any non-public address (no proxies), so DNS rebinding or a redirect to
  an internal host fails the delivery.
- **Delivery guarantees** — **at least once**: a receiver can see the same
  `event_id` more than once (for example when a response is lost after the
  endpoint processed it), so de-duplicate on `X-Event-Id`. Order is not
  guaranteed. Jobs live in the Redis sorted set `webhooks:queue`; a worker
  claims a job by removing it, so if the backend crashes between the claim
  and the retry being queued, **that event is lost**. Jobs not yet claimed
  survive a restart. The newest 50 deliveries per webhook are kept
  (`GET /api/webhooks/{id}/deliveries` returns the latest 20).

#### Bulk upload

- **Page** — **Settings → Bulk upload**: **Download template**, pick a CSV,
  upload; the job list shows status (`queued`, `processing`, `done`,
  `failed`) and progress, and **Download results CSV** once a job has
  finished.
- **CSV format** — the header must be exactly
  `long_url,title,custom_back_half,tags,domain` (case-insensitive; a UTF-8
  BOM is accepted). Only `long_url` is required; cells are trimmed, blank
  lines are skipped and short rows are padded. `tags` is separated by `|`.
  `domain` is the name of one of your **verified** custom domains; when it
  is empty the group's default domain is used, or the built-in short domain
  if the group has none.

  ```csv
  long_url,title,custom_back_half,tags,domain
  https://example.com/spring,Spring sale,spring,promo|email,
  https://example.com/docs,Docs,,,links.example.com
  https://example.com/blog,,,,
  ```
- **Limits** — 2 MiB per file and 1000 data rows (more is rejected with
  `400` at upload). Rows are created like `POST /api/links`, so the usual
  URL, back-half and tag rules apply per row; a bad row is reported in the
  results and never stops the upload.
- **Results** — `GET /api/bulk-uploads/{id}/results` (`409` until the job is
  `done` or `failed`) returns a CSV with columns
  `row,long_url,short_url,status,error`; `row` is the 1-based number among
  the data rows, `status` is `ok` or `error`, and error messages are cut to
  200 characters. Cells starting with `=`, `+`, `-` or `@` get a leading `'` so
  spreadsheets do not run them. Other endpoints: `POST /api/bulk-uploads`
  (multipart field `file`, `201`), `GET /api/bulk-uploads`, `GET
  /api/bulk-uploads/{id}`.
- **Worker behaviour** — the worker polls every `workers.bulk.poll_every`
  and updates progress every `workers.bulk.batch_size` rows. On a clean
  shutdown it stops after the current batch and marks the job `failed`
  ("interrupted by server shutdown") with the results so far — links already
  created are kept and the job is not resumed. If the process dies without a
  clean shutdown, a job stuck in `processing` for more than 10 minutes is
  put back to `queued` and **processed again from row 1**, so links created
  before the crash can be reported as errors ("that back-half is already
  taken") or, for rows without a back-half, duplicated.

#### Worker configuration

```yaml
workers:
  webhooks:
    poll_every: 2s   # how often due webhook deliveries are sent
  bulk:
    poll_every: 5s   # how often queued bulk uploads are picked up
    batch_size: 50   # rows processed between progress updates
```

All keys are optional; the values above are the defaults (non-positive
values fall back to them). Both workers run inside the backend process, and
webhook delivery needs Redis.

### Upgrading an existing install

- Database migrations run **automatically at backend startup**. Applied
  files are recorded in the `schema_migrations` table; `001_init.sql` is
  recorded as already applied when the `links` table exists.
- `002_bitly_clone.sql` and `003_links_back_half.sql` run **in place** on
  existing installs — no manual SQL is needed. Back up the database first.
- `003_links_back_half.sql` rebuilds the `links.short_code` column
  (`VARCHAR(255)`, `utf8mb4_bin`). MariaDB does this with a table-copy
  `ALTER`, so large `links` tables are briefly locked. **Short codes become
  case-sensitive** (`AbC` and `abc` are different links).
- `004_links_domain_key.sql` fails with MariaDB error 1062
  (`Duplicate entry ... for key 'uq_links_domain_key_code'`) when the
  database already holds two default-domain links with the same short code.
  Find them with
  `SELECT short_code, COUNT(*) FROM links WHERE domain_id IS NULL GROUP BY short_code HAVING COUNT(*) > 1;`,
  delete or re-code the duplicates, then restart the backend (the runner
  re-applies the pending file).
- `005_qr_codes.sql` creates the `qr_codes` and `qr_tags` tables in place.
- `006_dashboards.sql` creates the `dashboards` table in place (idempotent,
  applied by the startup migrator; no manual SQL needed).
- `007_pages.sql` creates the `pages`, `page_blocks` and `page_views`
  tables in place (idempotent, applied by the startup migrator; no manual
  SQL needed). Short code `m` is now reserved for new links.
- `008_pages_revision.sql` adds `pages.revision` and
  `pages.published_revision` (the counter behind `has_unpublished_changes`)
  in place; published pages not edited since their last publish start
  clean, the rest show unpublished changes until republished.
- `009_campaigns_webhooks_bulk.sql` creates the `campaigns`, `channels`,
  `campaign_channels`, `campaign_links`, `webhooks`, `webhook_deliveries`
  and `bulk_uploads` tables in place (idempotent, applied by the startup
  migrator; no manual SQL needed).
- With Docker, MariaDB only loads `001_init.sql` on first boot; later
  migrations always come from the backend image (`/app/migrations`).

### Registration

`server.allow_registration` (default `true`) controls whether new users can
sign up. Set it to `false` once your accounts exist.

### API tokens

Create a token in **Settings → API access tokens**; it is shown once. Use it
as a bearer token on any `/api/...` endpoint instead of a session:

```bash
curl -H "Authorization: Bearer us_xxxxxxxxxxxxxxxx" \
     http://localhost:8081/api/links
```

### Groups

Group-scoped endpoints (`/api/links`, `/api/tags`,
`/api/activity-log`, analytics) act on the group named by the `X-Group-ID`
header; without it the user's default group is used:

```bash
curl -H "Authorization: Bearer us_xxxxxxxxxxxxxxxx" \
     -H "X-Group-ID: 2" \
     http://localhost:8081/api/links
```

## Local development

```bash
# MariaDB and Redis
docker run -d --name us-mariadb -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=urlshortener \
  -e MYSQL_USER=urlshortener -e MYSQL_PASSWORD=password mariadb:10.11
docker run -d --name us-redis -p 6379:6379 redis:7-alpine

# Backend (applies migrations on startup)
cd backend
cp config.example.yaml config.yaml   # set database.password and jwt.secret
go run ./cmd/server

# Frontend
cd frontend
npm install
npm run dev                          # http://localhost:3000
```

### Tests

```bash
# Backend — DB tests need a separate database with the schema applied
# (backend/migrations/*.sql in order); they are skipped if it is unreachable.
# -p 1: packages share the test database, so run them sequentially.
cd backend
TEST_DATABASE_DSN='urlshortener:password@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27' \
  go test -p 1 ./...
go vet ./...

# Frontend
cd frontend
npm run lint
npx tsc --noEmit
npx vitest run
npm run build
```

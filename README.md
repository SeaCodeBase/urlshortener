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

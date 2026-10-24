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
- **Groups** — every link, tag, domain and activity entry belongs to a group;
  switch the active group from the UI.
- **API access tokens** — personal tokens (`us_...`) for scripted access.
- **Activity log** — what was created, edited, hidden or deleted, per group.
- **Custom domains** — add your own domain, verify it via DNS
  (CNAME to `urls.redirect_host`, or an A record to `urls.redirect_ip` for
  apex domains), and create links on it.

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

Group-scoped endpoints (`/api/links`, `/api/tags`, `/api/domains`,
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

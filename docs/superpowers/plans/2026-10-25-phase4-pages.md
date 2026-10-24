# Phase 4: Pages (link-in-bio) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Bitly's Pages feature end to end: pages model with blocks and views, list / claim-URL / editor (Build · Design · Track), publish snapshots, the public server-rendered page at `/m/[slug]` with view tracking, the redirect server's `/m/:slug` hop, and the link touchpoints ("Add to a page", Pages card, checklist).

**Architecture:** `pages` (editor works on live columns) + `page_blocks` (ordered, typed JSON content, optional owned link) + `page_views` (one row per public view). Publish copies a JSON snapshot into `pages.published_content`; the public endpoint serves only that snapshot. Link buttons on the public page point at the link's short URL with `?r=page`, so clicks flow through the existing redirect pipeline as `source='page'`; page view analytics come from `page_views`. Backend keeps repository → service → handler with mocks; frontend keeps TanStack Query hooks, the Phase 1 shell/components, `@dnd-kit` (Phase 3) for block reorder, and one shared `PagePreview` renderer used by the editor preview and the public page.

**Tech Stack:** Go 1.25, Gin, sqlx, MariaDB, testify, mockgen; Next.js 16 (app router, one server-rendered public route), React 19, TanStack Query 5, shadcn (Sheet, Tabs, Switch, Select, Dialog), `@dnd-kit/sortable`, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-24-bitly-clone-design.md` §5.1 (checklist), §5.2/§5.3/§5.4 (Add to a page, Pages card), §5.7, §6.1 (pages tables), §6.2 (Pages endpoints), §6.3, §10/§11 (4 block types, no YouTube/carousel/grid). Ledgers: `docs/superpowers/plans/2026-10-25-phase3-ledger.md` (rulings to respect: `POST /links` carries no nested sub-resources; follow-up calls instead).

## Global Constraints

- Repo root `/Users/jose/GolandProjects/urlshortener`, branch `feat/bitly-clone`, backend `backend/`, frontend `frontend/`. Commit after every task; messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` (an implementing subagent uses its own model name). Never `git add -A`; add files by path.
- Go: `cd backend && go vet ./... && TEST_DATABASE_DSN="urlshortener:urlshortener_pass@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC" go test -p 1 ./...` green. Apply the migration to BOTH local databases first: `docker exec -i us-mariadb mariadb -uroot -proot_secret_123 urlshortener < backend/migrations/007_pages.sql` and the same for `urlshortener_test`. Migrations are idempotent and follow the `database.SplitStatements` authoring rules. `go generate ./...` after any interface change; `go mod vendor` after any `go get`.
- Frontend: `cd frontend && npx vitest run && npm run lint && npx tsc --noEmit && npm run build` green.
- No billing/upgrade/AI/integrations UI. Copy fixed by spec §5.7 must match exactly: "Pages that turn visits into engagements", "Create your first page", "Claim your URL", "Create your page", "Start from scratch", "Preview", "Publish changes", "Share", "Build", "Design", "Track", "+ Add", "Link", "Text", "Image", "Social icons", "Draft", "Published", "Views", "Edit", "Hide", "Delete".
- Visual tokens: Phase 1 tokens in `frontend/src/app/globals.css`; cards `rounded-xl bg-card p-6`; charts current `#2A5BD7`, previous `#A5B8E8`.
- Dates: UTC calendar days (`YYYY-MM-DD`), inclusive; `NormalizeStatsQuery` limits apply; the Phase 3 `rangeFor` UTC convention applies to any preset.
- Error JSON `{"error": "..."}`; every `/pages` route is JWT/API-token protected and group-scoped (`X-Group-ID`); `/public/pages/*` routes are unauthenticated and never expose drafts.
- Slug rules: 3–40 chars, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` (no leading/trailing hyphen), globally unique (case-insensitive, stored lowercase), reserved slugs rejected: `api`, `admin`, `m`, `dashboard`, `login`, `register`, `static`, `assets`, `health`.
- Page URLs: `{BaseURL}/m/{slug}` is the shareable URL (redirect server hops to `{FrontendURL}/m/{slug}`); the frontend renders the page at `/m/[slug]`.

## Review Focus

1. The public endpoint must never serve a draft: a page with `status='draft'` or `published_content IS NULL` → 404; after Publish, later edits to blocks/design must not appear until the next Publish — Task 3 (`TestPageService_GetPublished_*`), Task 5 (`TestPublicPageHandler_DraftIs404`).
2. A page of another group (or another user) must be 404 for every `/pages/:id*` route, and a block must belong to the page in the URL (`/pages/:id/blocks/:bid` with a foreign bid → 404) — Task 3 (`TestPageService_*_WrongGroup`, `TestPageService_Block_ForeignBlock404`).
3. A link block may only reference a link of the same group with `source='link'`; deleting that link removes the block (FK cascade) and the published snapshot keeps rendering (button to a dead short link is acceptable until republish) — Task 1 (`TestPageRepo_BlockCascadesOnLinkDelete`), Task 3 (`TestPageService_AddLinkBlock_ForeignLink`).
4. Slug claims are case-insensitive, unique, and reserved words are rejected; a race on the same slug ends with one 201 and one 409 — Task 1 (`TestPageRepo_SlugUniqueCaseInsensitive`), Task 3 (`TestPageService_Create_SlugRules`).
5. `POST /public/pages/:slug/view` must be cheap and safe: 404 for unpublished, body ignored, referrer/UA truncated to column widths, IP hashed with the same salt as clicks, never a 500 on GeoIP failure — Task 4 (`TestPageService_RecordView_*`), Task 5 (`TestPublicPageHandler_View`).
6. Reorder must accept exactly the page's block ids as a permutation (missing or foreign id → 400) — Task 3 (`TestPageService_Reorder_Validation`).

---

## File map (backend)

- Create `backend/migrations/007_pages.sql`; `internal/model/page.go`; `internal/repository/page_repo.go` (+`_test.go`), `internal/repository/page_view_repo.go` (+`_test.go`); `internal/service/page_service.go` (+`_test.go`), `internal/service/page_public_service.go` (+`_test.go`; public + views + stats); `internal/handler/page.go` (+`_test.go`), `internal/handler/page_public.go` (+`_test.go`).
- Modify `repository/interfaces.go`, `repository/link_repo.go` (page membership join for `page_ids`), `service/interfaces.go`, `service/activity_service.go` (page actions), `handler/link_presenter.go` + `handler/link.go` (`page_ids`), `handler/redirect.go` (`/m/:slug` hop), `service/shortcode_service.go` or the reserved-code list (`m` reserved), `internal/testutil/db.go` (truncate `page_views`, `page_blocks`, `pages`), `cmd/server/main.go`, `README.md`.

## File map (frontend)

- Create `src/lib/hooks/usePages.ts`, `usePageStats.ts`; `src/components/pages/{designPresets.ts, PagePreview.tsx, PageCard.tsx, PageActionsMenu.tsx, ClaimUrlForm.tsx, TemplatePicker.tsx, EditorTopBar.tsx, BuildTab.tsx, BlockList.tsx, BlockRow.tsx, AddBlockMenu.tsx, BlockSheet.tsx, LinkBlockForm.tsx, TextBlockForm.tsx, ImageBlockForm.tsx, SocialBlockForm.tsx, DesignTab.tsx, TrackTab.tsx, PublicPageView.tsx, ViewBeacon.tsx}` (+ tests named per task); `src/components/links/AddToPageDialog.tsx`; pages `src/app/dashboard/pages/page.tsx` (replace placeholder), `src/app/dashboard/pages/new/page.tsx`, `src/app/dashboard/pages/[id]/page.tsx`, `src/app/m/[slug]/page.tsx` (server component) + `src/app/m/[slug]/not-found.tsx`.
- Modify `src/types/index.ts`, `src/lib/api.ts`, `src/lib/hooks/{index,keys}.ts`, `src/app/dashboard/links/[id]/page.tsx` (Pages card), `src/components/links/{LinkActionsMenu,LinkCard,LinkForm}.tsx`, `src/components/home/GettingStarted.tsx`, `package.json` (no new deps expected).

---

## Shared contracts (every task reads this)

**Migration `007_pages.sql`:**

```sql
CREATE TABLE IF NOT EXISTS pages (
    id                BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id           BIGINT UNSIGNED NOT NULL,
    group_id          BIGINT UNSIGNED NOT NULL,
    slug              VARCHAR(40) NOT NULL,
    title             VARCHAR(255) NOT NULL DEFAULT '',
    description       VARCHAR(500) NOT NULL DEFAULT '',
    avatar_url        VARCHAR(512) NOT NULL DEFAULT '',
    design            JSON NOT NULL,
    status            ENUM('draft','published') NOT NULL DEFAULT 'draft',
    published_content JSON NULL,
    published_at      TIMESTAMP NULL,
    archived          TINYINT(1) NOT NULL DEFAULT 0,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_pages_slug (slug),
    INDEX idx_pages_group (group_id, created_at),
    CONSTRAINT fk_pages_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_pages_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS page_blocks (
    id         BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    page_id    BIGINT UNSIGNED NOT NULL,
    type       ENUM('link','text','image','social') NOT NULL,
    position   INT NOT NULL DEFAULT 0,
    content    JSON NOT NULL,
    is_active  TINYINT(1) NOT NULL DEFAULT 1,
    is_pinned  TINYINT(1) NOT NULL DEFAULT 0,
    link_id    BIGINT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_page_blocks_page (page_id, position),
    INDEX idx_page_blocks_link (link_id),
    CONSTRAINT fk_page_blocks_page FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE,
    CONSTRAINT fk_page_blocks_link FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS page_views (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    page_id     BIGINT UNSIGNED NOT NULL,
    viewed_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_hash     VARCHAR(64) NOT NULL DEFAULT '',
    referrer    VARCHAR(512) NOT NULL DEFAULT '',
    country     VARCHAR(2) NOT NULL DEFAULT '',
    city        VARCHAR(100) NOT NULL DEFAULT '',
    device_type VARCHAR(20) NOT NULL DEFAULT '',
    browser     VARCHAR(50) NOT NULL DEFAULT '',
    INDEX idx_page_views_page_time (page_id, viewed_at),
    CONSTRAINT fk_page_views_page FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE
);
```

`slug` column uses the table's utf8mb4 general (case-insensitive) collation; the service lowercases before insert anyway.

**Design JSON (`pages.design`):**
```json
{"theme":"classic","background":{"type":"solid","color":"#F4F6FA","color2":"#FFFFFF"},"text_color":"#273144",
 "button":{"style":"fill","radius":"rounded","color":"#2A5BD7","text_color":"#FFFFFF"},"font":"inter"}
```
`theme` ∈ classic|bold|minimal|dark|sunset (5 presets, Task 6 defines their values; the backend stores whatever valid JSON matches this shape); `background.type` ∈ solid|gradient; `button.style` ∈ fill|outline; `button.radius` ∈ square|rounded|pill; `font` ∈ inter|serif|mono; colours `#RRGGBB`. Backend validates shape + enums + hex.

**Block content JSON by type:**
- `link`: `{"title":"…","description":"…","thumbnail_url":"…"}` — `title` 1–80 required; description ≤160; thumbnail http(s) URL or "". `link_id` required (block column), must be a same-group `source='link'` link.
- `text`: `{"heading":"…","body":"…"}` — at least one non-empty; heading ≤120, body ≤1000.
- `image`: `{"url":"…","alt":"…"}` — url required http(s) (or `/uploads/...` from the uploads endpoint), alt ≤120.
- `social`: `{"items":[{"platform":"instagram","url":"https://…"}]}` — 1–10 items; platform ∈ instagram|x|facebook|tiktok|youtube|linkedin|email|website; url http(s) (or `mailto:` for email).

**Page JSON** (`GET /pages/:id`, list items without `blocks`):
```json
{"id":1,"group_id":7,"slug":"jose","title":"Jose","description":"","avatar_url":"","design":{…},
 "status":"draft","published_at":null,"has_unpublished_changes":true,"archived":false,
 "url":"http://localhost:8080/m/jose","views":12,"created_at":"…","updated_at":"…",
 "blocks":[{"id":5,"type":"link","position":0,"content":{…},"is_active":true,"is_pinned":false,"link_id":9,
            "link":{"id":9,"title":"Menu","short_url":"http://localhost:8080/abc","original_url":"https://…"}}]}
```
`has_unpublished_changes` = `status='draft'` OR `updated_at > published_at` OR any block `updated_at > published_at` (computed in the service). `views` = all-time count. List `{"pages":[…],"total":n}` sorted `created_at` desc; filters `?q=` (title/slug substring) and `?archived=on|off` (default off).

**Published snapshot (`published_content`) and public JSON** (`GET /public/pages/:slug`):
```json
{"slug":"jose","title":"Jose","description":"…","avatar_url":"…","design":{…},"published_at":"…",
 "blocks":[{"id":5,"type":"link","content":{…},"is_pinned":false,"short_url":"http://localhost:8080/abc?r=page"}]}
```
Only `is_active` blocks, pinned first (stable by position), link blocks carry `short_url` with `?r=page` appended; no ids of links, no user/group ids.

**Stats JSON** (`GET /pages/:id/stats?from&to&unit&compare`): `{"from","to","unit","total_views_all_time","total_views","prev_total_views","change_pct","series":[{"date","clicks"}],"prev_series":[],"referrers":[ReferrerStat],"devices":[KeyStat],"countries":[CountryStat],"cities":[CityStat],"blocks":[{"block_id","title","link_id","short_url","clicks"}]}` — views come from `page_views`; `blocks[].clicks` = clicks with `source='page'` on the block's link in the window (from `ClickRepository.CountRange`). Reuse `SeriesPoint`/`ReferrerStat`/`KeyStat`/`CountryStat`/`CityStat` and the Phase 3 helpers.

**Endpoints** (auth + group unless noted):
- `GET /pages?q&archived`, `POST /pages {slug,title?,template?}` → 201 (`template` ∈ scratch|classic|bold|minimal → initial `design` preset + for templates two sample blocks: a text block "Welcome" and no link blocks), `GET /pages/check-slug?slug=` → `{"slug":"jose","available":true,"reason":""}` (reason ∈ "", "invalid", "reserved", "taken")
- `GET /pages/:id`, `PUT /pages/:id {title?,description?,avatar_url?,design?}`, `DELETE /pages/:id` → 204, `PATCH /pages/:id {archived:bool}` → 200, `POST /pages/:id/publish` → 200 page JSON (snapshot written, status published, published_at now)
- `POST /pages/:id/blocks {type,content,link_id?|new_link:{original_url,title?}}` → 201 block; `PUT /pages/:id/blocks/:bid {content?,is_active?,is_pinned?}`; `DELETE /pages/:id/blocks/:bid` → 204; `PUT /pages/:id/blocks/reorder {ids:[…]}` → 200 `{"blocks":[…]}`
- `GET /pages/:id/stats?…`
- Public (no auth, no group): `GET /public/pages/:slug` → published JSON | 404; `POST /public/pages/:slug/view` → 204 (always 204 for published; 404 unpublished)
- Redirect server: `GET /m/:slug` → 302 `{FrontendURL}/m/{slug}` (no lookup; the frontend 404s unknown slugs).
- Links: every link JSON object gains `page_ids: [1,3]` (pages whose blocks reference it, not archived).

**`new_link` on block create:** the service creates a normal `source='link'` link in the page's group through `LinkService.Create` (title from `new_link.title` or the block title) and uses its id. Ruling: page-created links are ordinary links visible in the Links list (Bitly behaviour); the click source `page` comes only from `?r=page`.

---

### Task 1: Migration, models, PageRepository

**Files:** Create `backend/migrations/007_pages.sql`, `internal/model/page.go`, `internal/repository/page_repo.go`, `internal/repository/page_repo_test.go`; modify `repository/interfaces.go`, `internal/testutil/db.go`, `service/shortcode_service.go` (or wherever reserved codes live: add `m`).

**Interfaces:** Produces `model.Page{ID, UserID, GroupID uint64; Slug, Title, Description, AvatarURL string; Design model.JSONMap; Status string; PublishedContent model.JSONMap (nil when NULL — check how `model/nullable.go` handles NULL JSON); PublishedAt *time.Time; Archived bool; CreatedAt, UpdatedAt}`, `model.PageBlock{ID, PageID uint64; Type string; Position int; Content model.JSONMap; IsActive, IsPinned bool; LinkID *uint64; CreatedAt, UpdatedAt}`, `model.PageView{…}` and
```go
type PageRepository interface {
    Create(ctx, p *model.Page) error                              // ErrPageSlugTaken on duplicate
    GetByID(ctx, id uint64) (*model.Page, error)                  // ErrPageNotFound
    GetBySlug(ctx, slug string) (*model.Page, error)              // ErrPageNotFound
    SlugExists(ctx, slug string) (bool, error)
    ListByGroup(ctx, groupID uint64, f PageFilter) ([]PageWithViews, error) // PageFilter{Query string; Archived bool}; PageWithViews{model.Page; Views int64}
    Update(ctx, p *model.Page) error                              // title, description, avatar_url, design; ErrPageNotFound
    SetArchived(ctx, id uint64, archived bool) error
    Publish(ctx, id uint64, content model.JSONMap, at time.Time) error // status, published_content, published_at
    Delete(ctx, id uint64) error
    ListBlocks(ctx, pageID uint64) ([]model.PageBlock, error)     // ORDER BY position, id
    GetBlock(ctx, id uint64) (*model.PageBlock, error)            // ErrBlockNotFound
    CreateBlock(ctx, b *model.PageBlock) error                    // position = max+1
    UpdateBlock(ctx, b *model.PageBlock) error                    // content, is_active, is_pinned
    DeleteBlock(ctx, id uint64) error
    ReorderBlocks(ctx, pageID uint64, ids []uint64) error         // one transaction; position = index
    PagesForLinks(ctx, linkIDs []uint64) (map[uint64][]uint64, error) // link id → page ids (archived pages excluded), distinct, sorted
    MaxBlockUpdatedAt(ctx, pageID uint64) (*time.Time, error)
}
```
Errors `ErrPageNotFound`, `ErrBlockNotFound`, `ErrPageSlugTaken` in `repository`.

- [ ] Write the migration (Shared contracts) and apply to both databases.
- [ ] Failing tests: `TestPageRepo_CreateGetSlug` (lowercase slug round trip, `SlugExists`), `TestPageRepo_SlugUniqueCaseInsensitive` (`Jose` after `jose` → `ErrPageSlugTaken`), `TestPageRepo_Blocks_CRUD_Reorder` (positions follow ids order; foreign id → error), `TestPageRepo_BlockCascadesOnLinkDelete`, `TestPageRepo_PagesForLinks_ExcludesArchived`, `TestPageRepo_Publish_SetsSnapshot`, `TestPageRepo_ListByGroup_FilterAndViews`.
- [ ] Implement; mockgen line; `go generate ./...`; add `m` to the reserved short codes (find the existing list; test it).
- [ ] `go test -p 1 ./internal/repository/...` PASS; commit `feat(pages): pages, blocks and views tables, model and repository`.

### Task 2: PageViewRepository

**Files:** Create `internal/repository/page_view_repo.go` (+`_test.go`); modify `repository/interfaces.go`.

**Interfaces:** Produces
```go
type PageViewRepository interface {
    Insert(ctx, v *model.PageView) error
    CountAllTime(ctx, pageID uint64) (int64, error)
    CountRange(ctx, pageID uint64, from, to time.Time) (int64, error)
    Series(ctx, pageID uint64, from, to time.Time, unit string) ([]Bucket, error)   // zero-filled like clicks
    Breakdown(ctx, pageID uint64, from, to time.Time, facet string, limit int) ([]Row, error) // referrer|device|country|city|browser
    CountsForPages(ctx, pageIDs []uint64) (map[uint64]int64, error)
}
```
Share `halfOpen`, `seriesUnits`, the referrer/device/country/city column expressions with `click_repo.go` (extract the maps if needed, no duplication).

- [ ] Failing tests `TestPageViewRepo_SeriesZeroFilled`, `TestPageViewRepo_BreakdownUnknownBuckets` ("Direct" for empty referrer, "Unknown" for empty country/device), `TestPageViewRepo_CountsForPages`.
- [ ] Implement; mockgen; `go generate ./...`; tests PASS; commit `feat(pages): page view repository`.

### Task 3: PageService (CRUD, slug, blocks, reorder, publish)

**Files:** Create `internal/service/page_service.go` (+`_test.go`); modify `service/interfaces.go`, `service/activity_service.go` (actions `page.create|update|publish|hide|unhide|delete`, `EntityPage = "page"`).

**Interfaces:** Consumes Task 1 repo, `LinkRepository.GetByIDs`, `LinkService.Create(ctx, userID, CreateLinkInput{… GroupID, Source: "link"})` (check its exact signature), `DomainRepository` + `BaseURL` for short URLs (as `AnalyticsService` does), `ActivityService.Log`, `ValidationError` (Phase 3). Produces
```go
type PageService interface {
    CheckSlug(ctx, slug string) (SlugCheck, error)                    // {Slug, Available bool, Reason string}
    Create(ctx, userID, groupID uint64, in CreatePageInput) (*PageView, error) // in{Slug, Title, Template string}; ErrPageSlugTaken → 409
    List(ctx, groupID uint64, f repository.PageFilter) ([]PageView, error)
    Get(ctx, groupID, id uint64) (*PageView, error)                   // with blocks + link summaries; ErrPageNotFound when group mismatch
    Update(ctx, userID, groupID, id uint64, in UpdatePageInput) (*PageView, error) // pointers; validates design
    SetArchived(ctx, userID, groupID, id uint64, archived bool) (*PageView, error)
    Delete(ctx, userID, groupID, id uint64) error
    Publish(ctx, userID, groupID, id uint64) (*PageView, error)       // builds snapshot per Shared contracts
    AddBlock(ctx, userID, groupID, pageID uint64, in BlockInput) (*BlockView, error)   // BlockInput{Type; Content map[string]any; LinkID *uint64; NewLink *NewLinkInput}
    UpdateBlock(ctx, userID, groupID, pageID, blockID uint64, in BlockPatch) (*BlockView, error)
    DeleteBlock(ctx, userID, groupID, pageID, blockID uint64) error
    Reorder(ctx, userID, groupID, pageID uint64, ids []uint64) ([]BlockView, error)
    PageIDsForLinks(ctx, linkIDs []uint64) (map[uint64][]uint64, error)
}
```
`PageView`/`BlockView` carry the JSON tags of Shared contracts (`url` built from `BaseURL + "/m/" + slug`). `ValidateDesign(map[string]any) error` and `ValidateBlockContent(type string, content map[string]any) error` are exported package functions (handlers and tests use them). `DefaultDesign(template string) model.JSONMap` returns the preset for scratch/classic/bold/minimal (values from the Task 6 contract below; keep them in one Go map).

- [ ] Failing tests (mocks): `TestPageService_Create_SlugRules` (invalid, reserved, taken→ErrPageSlugTaken, uppercase lowercased, template seeds a text block), `TestPageService_Get_WrongGroup`, `TestPageService_AddLinkBlock_ForeignLink` (other group / qr-owned link → ValidationError), `TestPageService_AddLinkBlock_NewLink` (LinkService.Create called with the page's group and `Source:"link"`), `TestPageService_Block_ForeignBlock404`, `TestPageService_Reorder_Validation` (missing id, foreign id, duplicate → ValidationError), `TestPageService_Publish_Snapshot` (active only, pinned first, `?r=page` short urls, no link ids), `TestPageService_HasUnpublishedChanges`, `TestPageService_ValidateDesign` (enum/hex table), `TestPageService_ValidateBlockContent` (table per type).
- [ ] Implement; mockgen; `go generate ./...`; `go test -p 1 ./internal/service/...` PASS; commit `feat(pages): page service with slug rules, blocks, reorder and publish snapshots`.

### Task 4: Public page service: GetPublished, RecordView, Stats

**Files:** Create `internal/service/page_public_service.go` (+`_test.go`); modify `service/interfaces.go`.

**Interfaces:** Consumes Tasks 1–2 repos, `ClickRepository.CountRange(ids, from, to, "page")`, `util.ParseUserAgent`, `util.LookupIP`, the Phase 3 helpers (`loadBreakdowns`, `changePct`, `toSeries`, `NormalizeStatsQuery`). Produces
```go
type PagePublicService interface {
    GetPublished(ctx, slug string) (map[string]any, error)   // the snapshot (+ "slug"); ErrPageNotFound for missing/unpublished/archived
    RecordView(ctx, slug string, v ViewInput) error          // ViewInput{IP, UserAgent, Referrer string}; ErrPageNotFound when unpublished; truncates referrer 512, browser 50, device 20, city 100
    Stats(ctx, groupID, pageID uint64, q StatsQuery) (*PageStats, error) // *StatsQueryError / ErrPageNotFound
}
```
`NewPagePublicService(pageRepo, viewRepo, clickRepo, ipSalt string)` (hash IP exactly like `RedirectHandler.hashIP` — move that helper to `util` if needed and reuse it).

- [ ] Failing tests: `TestPagePublicService_GetPublished_DraftIs404`, `_ArchivedIs404`, `_ServesSnapshotNotLive` (live title changed after publish → snapshot title returned), `TestPagePublicService_RecordView_Truncates`, `_HashesIP`, `TestPagePublicService_Stats_BlocksClicks` (two link blocks → CountRange per link with source page; totals/series/breakdowns from view repo).
- [ ] Implement; mockgen; tests PASS; commit `feat(pages): public page, view tracking and page stats service`.

### Task 5: Handlers, routes, redirect hop, link `page_ids`

**Files:** Create `internal/handler/page.go` (+`_test.go`), `internal/handler/page_public.go` (+`_test.go`); modify `handler/redirect.go` (+ test), `handler/link_presenter.go`, `handler/link.go` (+ tests), `repository/link_repo.go` only if `page_ids` needs a join (prefer `PageService.PageIDsForLinks` in the presenter), `cmd/server/main.go`.

**Interfaces:** Routes per Shared contracts. Error mapping: `*ValidationError` → 400, `ErrPageSlugTaken` → 409 `{"error":"slug is already taken"}`, `ErrPageNotFound`/`ErrBlockNotFound` → 404, `*StatsQueryError` → 400. `POST /public/pages/:slug/view` reads `c.ClientIP()`, `User-Agent`, `Referer`; responds 204 with no body. Redirect server: `redirectRouter.GET("/m/:slug", redirectHandler.PageHop)` → 302 to `FrontendURL + "/m/" + slug` (reject slugs failing the slug regex with 404). Link presenter adds `page_ids` (empty array when none) to every link response (list, get, create, update, bulk) via one `PageIDsForLinks` call per response.

- [ ] Failing handler tests: `TestPageHandler_CreateConflict409`, `_CheckSlug`, `_Get_OtherGroup404`, `_Blocks_CRUD`, `_Reorder400`, `_Publish`, `_Stats_BadQuery400`, `TestPublicPageHandler_DraftIs404`, `TestPublicPageHandler_View` (204, passes IP/UA/Referer), `TestRedirectHandler_PageHop` (302 Location; bad slug 404), `TestLinkHandler_PageIDsPresent`.
- [ ] Implement; wire `main.go` (services after link service; public routes outside auth; redirect hop); `go vet && go test -p 1 ./...` PASS.
- [ ] Live smoke (restart the dev backend on 8081/8080): register throwaway user → `POST /pages` → add text + link block (new_link) → `POST publish` → `GET /public/pages/:slug` (snapshot with `?r=page` short url) → `POST view` 204 → `GET /pages/:id/stats` shows 1 view → `curl -I localhost:8080/m/<slug>` 302 → `GET /links/:id` has `page_ids`. Put commands + outputs in the report.
- [ ] Commit `feat(api): pages endpoints, public page and view routes, redirect hop, link page_ids`.

### Task 6: Frontend types, API, hooks, design presets, PagePreview

**Files:** Create `src/lib/hooks/usePages.ts`, `src/lib/hooks/usePageStats.ts`, `src/components/pages/designPresets.ts` (+`.test.ts`), `src/components/pages/PagePreview.tsx` (+`.test.tsx`); modify `src/types/index.ts`, `src/lib/api.ts`, `src/lib/hooks/{index,keys}.ts`.

**Interfaces:** Types mirror Shared contracts (`Page`, `PageBlock`, `BlockType`, `BlockContent` union, `PageDesign`, `PublishedPage`, `PageStats`, `SlugCheck`, `CreatePageInput`, `BlockInput`, `BlockPatch`, `PageFilters`); `api`: `listPages(filters)`, `getPage(id)`, `createPage(input)`, `updatePage(id, patch)`, `setPageArchived(id, archived)`, `deletePage(id)`, `publishPage(id)`, `checkSlug(slug)`, `addBlock(pageId, input)`, `updateBlock(pageId, bid, patch)`, `deleteBlock(pageId, bid)`, `reorderBlocks(pageId, ids)`, `getPageStats(id, query)`, `getPublicPage(slug)` (used client-side only for tests), `recordPageView(slug)`; hooks `usePages(filters)`, `usePage(id)`, `usePageMutations()` → `{create, update, setArchived, remove, publish, addBlock, updateBlock, deleteBlock, reorder}` invalidating `['pages']`, `['page', id]` and `['link']` prefixes; `usePageStats(id, query)`; `useSlugCheck(slug)` (debounced 300 ms, enabled when the slug matches the regex); keys `pages(groupId, filters)`, `page(id)`, `pageStats(id, query)`, `slugCheck(slug)`.
`designPresets.ts`: `DESIGN_PRESETS: Record<'classic'|'bold'|'minimal'|'dark'|'sunset', PageDesign>` — classic (canvas `#F4F6FA`, text `#273144`, fill `#2A5BD7`/`#FFFFFF`, rounded, inter), bold (solid `#111827`, text `#F9FAFB`, fill `#F59E0B`/`#111827`, pill, inter), minimal (solid `#FFFFFF`, text `#111827`, outline `#111827`/`#111827`, square, serif), dark (gradient `#0F172A`→`#1E293B`, text `#E2E8F0`, fill `#38BDF8`/`#0F172A`, rounded, mono), sunset (gradient `#F97316`→`#EC4899`, text `#FFFFFF`, fill `#FFFFFF`/`#9A3412`, pill, inter); `SLUG_RE`, `RESERVED_SLUGS`, `SOCIAL_PLATFORMS` (label + lucide icon per platform), `pageUrl(baseUrl, slug)`.
`PagePreview({page: {title, description, avatar_url, design, blocks}, mode: 'editor'|'public', baseHref?})`: renders avatar/title/description, blocks in order (editor mode shows all blocks with inactive ones at 50% opacity; public mode receives already-filtered blocks); link buttons are `<a>` to `short_url` (public) or `#` (editor); styles from `design` via inline CSS variables (`--page-bg`, `--page-text`, `--btn-bg`, `--btn-text`, radius map square 4px / rounded 12px / pill 999px; font map inter → `var(--font-inter)`, serif → Georgia, mono → ui-monospace). The same component is imported by the public server route (must be a pure component without hooks that need the client; mark `'use client'` only on the beacon, not on the preview).

- [ ] Failing tests `designPresets.test.ts` (5 presets validate against the shape; `SLUG_RE` cases), `PagePreview.test.tsx` (renders title, applies `--btn-bg`, public mode links to `short_url`, editor mode dims inactive blocks, social icons render per platform).
- [ ] Implement; `npx vitest run && npm run lint && npx tsc --noEmit` PASS; commit `feat(frontend): pages types, api, hooks, design presets and page preview`.

### Task 7: Pages list and claim-URL flow

**Files:** Modify `src/app/dashboard/pages/page.tsx`; create `src/app/dashboard/pages/new/page.tsx`, `src/components/pages/{PageCard,PageActionsMenu,ClaimUrlForm,TemplatePicker}.tsx`, tests `PageCard.test.tsx`, `ClaimUrlForm.test.tsx`.

**Produces:** `/dashboard/pages`: `PageHeader` "Pages" + primary "Create page"; empty state hero "Pages that turn visits into engagements" with "Create your first page" (→ `/dashboard/pages/new`); search "Search pages" + "Show: Active | Hidden" (`?archived=on`); cards: avatar initial/image, title (link to editor), URL `{base}/m/{slug}` with CopyButton, "{n} views", status chip "Draft"/"Published" (+ "Unpublished changes" dot when `has_unpublished_changes` on a published page), actions "Edit" (→ editor), "Share" (`ShareDialog` with the page URL), ⋯ "Hide"/"Show", "Delete" (confirm "Delete page?" / "This permanently deletes the page and its blocks. Links stay." / "Delete"). Error/loading/empty states like the dashboards list.
`/dashboard/pages/new`: step 1 "Claim your URL": prefix `{host}/m/` + slug input (lowercases as you type, 3–40, `[a-z0-9-]`), live `useSlugCheck` → "Available" / "Already taken" / "Invalid" / "Reserved"; button "Create your page" (disabled until available). Step 2 `TemplatePicker`: "Start from scratch" tile + three template tiles (Classic, Bold, Minimal) rendering a mini `PagePreview` of the preset with sample blocks; choosing one calls `create({slug, template})` and routes to `/dashboard/pages/{id}`.

- [ ] Failing tests: `PageCard.test.tsx` (status chip, views, menu actions call `setArchived`/`remove`), `ClaimUrlForm.test.tsx` (lowercasing, disabled until available, create called with slug).
- [ ] Implement; full frontend checks PASS; commit `feat(frontend): pages list, claim URL and template picker`.

### Task 8: Editor shell + Build tab (blocks)

**Files:** Create `src/app/dashboard/pages/[id]/page.tsx`, `src/components/pages/{EditorTopBar,BuildTab,BlockList,BlockRow,AddBlockMenu,BlockSheet,LinkBlockForm,TextBlockForm,ImageBlockForm,SocialBlockForm}.tsx`, tests `BlockList.test.tsx`, `BlockSheet.test.tsx`, `LinkBlockForm.test.tsx`.

**Produces:** Editor route: left/centre editor column + right phone-frame `PagePreview` (sticky, 375 px wide, hidden under `lg`; the "Preview" button opens the same preview in a Dialog). `EditorTopBar`: back "← Pages", editable title (inline, saves via `update({title})`), status chip, buttons "Preview", "Share" (`ShareDialog`), "Publish changes" (primary; disabled when `!has_unpublished_changes`; on success toast "Page published"), ⋯ menu "View page" (opens `page.url`), "Hide"/"Show", "Delete". Tabs "Build" | "Design" | "Track" (`?tab=`).
Build tab: `BlockList` with `@dnd-kit/sortable` (drag handle `aria-label="Reorder block"`, keyboard sensor), each `BlockRow` showing type icon, title summary (link: content.title + short_url; text: heading/body excerpt; image: url; social: n platforms), toggles "Active" (`Switch`) and pin (`Pin` icon button, `aria-pressed`), "Edit" (opens `BlockSheet`), "Delete" (confirm). Reorder persists with `reorder(ids)` (optimistic, revert + toast on failure). "+ Add" dropdown: Link, Text, Image, Social icons → opens `BlockSheet` in create mode.
`BlockSheet` (shadcn Sheet, right side): per-type form; Link form: segmented "Existing link" (link picker: search via `useLinks({q})`, radio list, excludes qr-owned automatically since the list only has `source='link'`) | "New destination" (URL input + optional title), then "Button title" (required, prefilled from the link title), description, thumbnail URL; Text form: heading, body (textarea); Image form: URL + "Upload" (uses the Phase 2 upload API `api.uploadImage`, then sets the URL), alt; Social form: list rows (platform select + URL) with add/remove (max 10). "Save"/"Cancel"; validation mirrors the backend rules with inline messages; API errors toast.

- [ ] Failing tests: `BlockList.test.tsx` (renders rows with summaries; toggling Active calls `updateBlock(pageId, bid, {is_active:false})`; delete confirm calls `deleteBlock`), `BlockSheet.test.tsx` (text form validation "Add a heading or body"; save calls `addBlock` with the right content), `LinkBlockForm.test.tsx` (existing-link mode submits `link_id` + title; new-destination mode submits `new_link`).
- [ ] Implement; checks PASS; commit `feat(frontend): page editor shell, preview and Build tab with blocks`.

### Task 9: Design tab and Track tab

**Files:** Create `src/components/pages/{DesignTab,TrackTab}.tsx`, tests `DesignTab.test.tsx`, `TrackTab.test.tsx`.

**Produces:** Design tab: "Profile" card (avatar: URL input + "Upload" via the uploads API, preview; display name = title; bio = description), "Theme" card: 5 preset tiles (mini `PagePreview`) → sets the whole design; "Background": Solid | Gradient segmented + colour inputs (`<input type="color">` + hex text); "Text colour"; "Buttons": style Fill | Outline, radius Square | Rounded | Pill, button colour + text colour; "Font": Inter | Serif | Mono. Every control updates the page immediately through `update({design})` debounced 400 ms (the preview re-renders from the optimistic local design). Invalid hex shows an inline error and is not sent.
Track tab: controls (date range button with presets/Custom via `DateRangeDialog`, comparison select) sharing the `LinkAnalytics` URL-state helpers (`parseAnalyticsUrlState`, `effectiveRange`, `unitFor`, `rangeLabel`, UTC `rangeFor` convention), scorecards "Total views" (all time) and "Views ({range})" with change badge, "Views over time" `TimeSeriesChart`, "Link clicks" table (block title · short url · clicks; `EmptyState` "No link blocks yet"), "Views by referrer" / "Views by device" (`DeviceBarChart`) / "Views by location" (Countries | Cities tabs, `BreakdownTable`), CSV download of the loaded stats (sections like `buildStatsCsv`; add `buildPageStatsCsv` in `src/lib/csv.ts` with a test).

- [ ] Failing tests: `DesignTab.test.tsx` (preset tile applies the preset; invalid hex not sent), `TrackTab.test.tsx` (renders totals and block clicks rows from a stats fixture; CSV builder test in `csv.test.ts`).
- [ ] Implement; checks PASS; commit `feat(frontend): page editor Design and Track tabs`.

### Task 10: Public page `/m/[slug]`

**Files:** Create `src/app/m/[slug]/page.tsx` (server component), `src/app/m/[slug]/not-found.tsx`, `src/components/pages/PublicPageView.tsx`, `src/components/pages/ViewBeacon.tsx` (+`.test.tsx`).

**Produces:** Server component fetches `${process.env.BACKEND_URL || 'http://localhost:8081'}/api/public/pages/${slug}` with `cache: 'no-store'`; 404 → `notFound()`; renders `PublicPageView` (full-height canvas using the design, centred 480 px column, `PagePreview mode="public"`), `generateMetadata` (title = page title, description), and `ViewBeacon` (client; `useEffect` → `api.recordPageView(slug)` once per mount, `navigator.sendBeacon` when available, swallow errors). No app shell, no auth; `robots` index allowed. Link buttons open in the same tab. A small footer "Made with URL Shortener" linking to `/`.

- [ ] Failing test `ViewBeacon.test.tsx` (posts once per mount; errors swallowed) and a `PublicPageView.test.tsx` (renders title and buttons from a published fixture).
- [ ] Implement; `npm run build` must show `ƒ /m/[slug]`; checks PASS; commit `feat(frontend): public link-in-bio page with view tracking`.

### Task 11: Link touchpoints and checklist

**Files:** Modify `src/app/dashboard/links/[id]/page.tsx` (`PagesCard`), `src/components/links/{LinkActionsMenu,LinkCard,LinkForm}.tsx` (+ tests), `src/components/home/GettingStarted.tsx` (+ test); create `src/components/links/AddToPageDialog.tsx` (+`.test.tsx`).

**Produces:** `AddToPageDialog({open, onOpenChange, link})`: title "Add to a page", page `Select` (active pages of the group; empty → text "You don't have any pages yet" + link "Create a page"), "Button title" input (prefilled with the link title or host), info "Make sure you publish your page's updates after adding this link", buttons Cancel / "Add to page" → `addBlock(pageId, {type:'link', link_id, content:{title}})`, toast "Added to {page}". Wired from the ⋯ menu "Add to a page" (LinkActionsMenu/LinkCard/details header) and from the details `PagesCard` ("+ Add to a page"; lists pages containing the link from `link.page_ids` resolved through `usePages()` with links to their editors). `LinkForm` "Add to a page" switch becomes enabled: page select + "Button title"; after link creation the form issues the follow-up `addBlock` call (non-atomic, toast on failure — same ruling as the QR follow-up). Checklist step 4 "Create a page" auto-checks when `usePages()` returns ≥1 page.

- [ ] Failing tests: `AddToPageDialog.test.tsx` (submits `addBlock` with the chosen page and title; empty state), `GettingStarted` test addition (page step done when a page exists), `LinkForm` test addition (switch enabled; follow-up call after create).
- [ ] Implement; checks PASS; commit `feat(frontend): add links to pages from link pages and the create form; checklist page step`.

### Task 12: Docs

**Files:** Modify `README.md` (Phase 4 section: pages endpoints, public routes, `/m/:slug` hop, migration 007 note, design/block JSON summary, `page_ids`), `backend/config.example.yaml` only if a key was added (none expected).

- [ ] Verify every listed route in `main.go`; commit `docs: phase 4 pages`.

---

## Self-review notes

- Spec coverage: §5.7 list/new/editor (Build/Design/Track)/public/draft-vs-published (T1–T10), §6.1 tables (T1), §6.2 endpoints incl. check-slug/reorder/stats/public (T5), §6.3 redirect hop (T5), §5.1 checklist + §5.2/§5.3/§5.4 touchpoints (T11). Out of scope by spec §10: YouTube/carousel/grid blocks. Rulings: page-created links are ordinary `source='link'` links; link deletion cascades its blocks; `POST /links` carries no `page_id` (follow-up block call); 5 presets with fixed values; views are not de-duplicated.
- Type consistency: `BlockInput{Type, Content, LinkID, NewLink}` is the one block-create shape across service, handler and frontend; `PageDesign` shape identical in Go validation and `designPresets.ts`; stats reuse Phase 3 stat types.

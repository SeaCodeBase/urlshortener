# Phase 3: Analytics — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver Bitly's Analytics area end to end: the Analytics overview page, group dashboards (CRUD, per-group "Default" dashboard, module data, filter by links/QR codes, visualization switching, drag reorder, CSV zip export) and the "Compare to a link" control on link details.

**Architecture:** A `dashboards` row per group stores `date_range`, `filter` JSON and `layout` JSON; every module's data comes from one `GET /analytics/dashboards/:id/data` call computed by a new `AnalyticsService` over a `ClickScope` (whole group, or an explicit link set) added to `ClickRepository`. Links and QR codes are one population: clicks with `source='qr'` on a link that owns a QR code are attributed to that QR code item, everything else to the link. Frontend keeps TanStack Query hooks, the Phase 1 chart components, and adds doughnut/bar charts, a sortable module grid (`@dnd-kit`) and dashboard pages copied from the Links/QR page patterns.

**Tech Stack:** Go 1.25, Gin, sqlx, MariaDB, `archive/zip` (stdlib), testify, mockgen; Next.js 16, React 19, TanStack Query 5, shadcn, recharts 3, `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities`, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-24-bitly-clone-design.md` §5.1 (Today's stats), §5.4 ("Compare to a link"), §5.8, §6.2 (Analytics endpoints), §6.4, §7. Phase 1 ledger `docs/superpowers/plans/2026-10-24-phase1-ledger.md`; Phase 2 ledger `docs/superpowers/plans/2026-10-25-phase2-ledger.md`.

## Global Constraints

- Repo root `/Users/jose/GolandProjects/urlshortener`, branch `feat/bitly-clone`, backend in `backend/`, frontend in `frontend/`. Commit after every task; messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` (an implementing subagent uses its own model name). Never `git add -A`; add the task's files by path.
- Go tests: `cd backend && TEST_DATABASE_DSN="urlshortener:urlshortener_pass@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC" go test -p 1 ./...` must stay green. Apply the new migration to BOTH databases first: `docker exec -i us-mariadb mariadb -uroot -proot_secret_123 urlshortener < backend/migrations/006_dashboards.sql` and the same for `urlshortener_test`. Migration files are idempotent (`IF NOT EXISTS`) and follow the authoring rules in the comment on `database.SplitStatements`.
- `backend/vendor/` exists (untracked) and vendor mode is on: after `go get`, run `go mod tidy && go mod vendor`. Regenerate mocks after changing any interface: `cd backend && go generate ./...`.
- Frontend checks: `cd frontend && npx vitest run && npm run lint && npx tsc --noEmit && npm run build` must pass.
- No billing/upgrade/AI/integrations UI (spec §1). Copy fixed by spec §5.8 must match exactly: "Analytics", "Dashboards", "+ Create dashboard", "View all →", "Performance overview", "TOTAL ENGAGEMENTS · THIS WEEK", "Top performing", "Sort by: Date created", "Add filters", "Create new dashboard", "Modified on", "Created by", "Showing data for all links and QR Codes", "Rename", "Duplicate", "Delete", "← Dashboards", "Download CSV", "Previous period", "No comparison", "Filter", "Apply filters", "Change visualization", module titles "Top day by engagements", "Top location by engagements", "Engagements over time", "Engagements by device", "Engagements by referrer", "Engagements by location", "Links & QR Codes by engagements"; §5.4 "Compare to a link".
- Visual tokens are the Phase 1 tokens in `frontend/src/app/globals.css`; cards `rounded-xl bg-card p-6`; chart colours current `#2A5BD7`, previous `#A5B8E8`, doughnut `DOUGHNUT_PALETTE` from `components/charts/palette.ts`.
- All dates are UTC calendar days (`YYYY-MM-DD`), inclusive, exactly like `service.StatsQuery`; "previous period" is the same length ending the day before `from`. Ranges obey `NormalizeStatsQuery` (≤366 days, hourly ≤2 days).
- Error JSON stays `{"error": "..."}`. Every analytics route is JWT/API-token protected and group-scoped (`X-Group-ID`, `middleware.GetGroupID`).
- Links with `source='qr'` are QR code items, never link items, in every analytics list (spec §11.3).

## Review Focus

1. A dashboard of another group (or another user's group) must be 404 for list/get/data/export/update/delete — Task 4 (`TestDashboardService_*_WrongGroup`), Task 5 (`TestDashboardHandler_Get_OtherGroup404`).
2. The "Default" dashboard must exist for a brand-new group on the first list call and must never be deleted (409) — Task 4 (`TestDashboardService_List_CreatesDefault`, `TestDashboardService_Delete_DefaultConflict`).
3. A filter with more than 200 ids, an id from another group, or a QR id in a links filter must be rejected with 400, never silently narrowed — Task 4 (`TestDashboardService_Update_FilterValidation`).
4. Module data must attribute QR scans of a link that owns a QR code to the QR item and keep its plain clicks on the link item; a link without a QR keeps every source — Task 3 (`TestAnalyticsService_Items_SplitsQrScans`).
5. `compare_link` on `GET /links/:id/stats` must 404 for a link the user does not own and must ignore itself (`compare_link == id` → 400) — Task 6 (`TestStatsService_CompareLink_*`).
6. Export CSV cells must pass `csvSafe` (formula injection) and the zip must contain exactly the seven module files — Task 4 (`TestAnalyticsService_ExportZip`).

---

## File map (backend)

- Create `backend/migrations/006_dashboards.sql`; `internal/model/dashboard.go`; `internal/repository/dashboard_repo.go` (+ `_test.go`); `internal/service/analytics_service.go` (+ `_test.go`), `internal/service/dashboard_service.go` (+ `_test.go`); `internal/handler/dashboard.go` (+ `_test.go`).
- Modify `repository/interfaces.go` (ClickScope methods, DashboardRepository), `repository/click_repo.go` (+ `_test.go`), `service/interfaces.go`, `service/stats_service.go` (compare link), `handler/stats.go` (`compare_link`), `handler/analytics.go` (`Overview`), `internal/testutil/db.go` (truncate `dashboards`), `cmd/server/main.go`, `README.md`.

## File map (frontend)

- Create `src/types` additions (in `src/types/index.ts`), `src/lib/hooks/useAnalytics.ts`, `src/lib/hooks/useDashboards.ts`; `src/components/charts/DoughnutChart.tsx`, `src/components/charts/BarBreakdownChart.tsx`; `src/components/analytics/{OverviewDashboards,PerformanceOverview,TopPerforming,DashboardRow,DashboardActionsMenu,DashboardNameDialog,DashboardControls,DashboardFilterDialog,ModuleCard,ModuleGrid,modules.ts,dashboardCsv.ts}.tsx|ts`; `src/components/links/CompareLinkDialog.tsx`; pages `src/app/dashboard/analytics/page.tsx` (replace placeholder), `src/app/dashboard/analytics/dashboards/page.tsx`, `src/app/dashboard/analytics/dashboards/[id]/page.tsx`.
- Modify `src/lib/api.ts`, `src/lib/hooks/{index,keys}.ts`, `src/components/links/LinkAnalytics.tsx` (compare to a link), `src/components/charts/TimeSeriesChart.tsx` (optional third series), `src/components/home/TodayStats.tsx` (chips link to `/dashboard/analytics`), `package.json`.

---

## Shared contracts (every task reads this)

**Migration `006_dashboards.sql`:**

```sql
CREATE TABLE IF NOT EXISTS dashboards (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    group_id    BIGINT UNSIGNED NOT NULL,
    created_by  BIGINT UNSIGNED NOT NULL,
    name        VARCHAR(255) NOT NULL,
    is_default  TINYINT(1) NOT NULL DEFAULT 0,
    date_range  VARCHAR(20) NOT NULL DEFAULT 'last_7_days',
    filter      JSON NOT NULL,
    layout      JSON NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_dashboards_group (group_id, created_at),
    CONSTRAINT fk_dashboards_group FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
    CONSTRAINT fk_dashboards_user FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
);
```

(Check the real name of the groups table in `002_bitly_clone.sql` and use it.)

**`date_range` values:** `today`, `last_7_days` (default), `last_30_days`, `last_60_days`, `last_90_days`. Custom ranges are URL-only and never persisted (ruling).

**`filter` JSON:** `{"type":"all"}` | `{"type":"links","ids":[1,2]}` | `{"type":"qr","ids":[5]}`; `ids` has 1–200 entries that belong to the group (`links` with `source='link'` for `links`; `qr_codes` ids for `qr`). Default `{"type":"all"}`.

**`layout` JSON:** `{"modules":[{"key":"top_day","viz":"scorecard"},{"key":"top_location","viz":"scorecard"},{"key":"over_time","viz":"line"},{"key":"devices","viz":"doughnut"},{"key":"referrers","viz":"bar"},{"key":"locations","viz":"table"},{"key":"items","viz":"table"}]}` — that exact order is the default. Allowed `viz` per key: `top_day` scorecard; `top_location` scorecard; `over_time` line|bar|table; `devices` doughnut|bar|table; `referrers` bar|doughnut|table; `locations` table|bar; `items` table|bar. A layout must contain each of the seven keys exactly once.

**Scope semantics** (`repository.ClickScope{GroupID uint64; LinkIDs []uint64; Source string}`): `LinkIDs == nil` → every link of the group (`link_id IN (SELECT id FROM links WHERE group_id = ?)`); non-nil → `link_id IN (?)` (empty slice → no rows). `Source`: `""` all, `link`, `qr`, `page`, or `not_qr` (`source <> 'qr'`). A `links` filter becomes `{LinkIDs: ids, Source: "not_qr"}`; a `qr` filter becomes `{LinkIDs: linkIDsOfThoseQRs, Source: "qr"}`; `all` becomes `{GroupID: g}`.

**Dashboard data JSON** (`GET /analytics/dashboards/:id/data?from&to&compare=previous|none`), unit is always `day` except ranges ≤ 2 days (`hour`), the same rule as `NormalizeStatsQuery`:

```json
{"from":"2026-10-18","to":"2026-10-24","unit":"day","compare":true,
 "total":120,"prev_total":100,"change_pct":20,
 "top_day":{"date":"2026-10-20","count":40,"prev_count":12,"change_pct":233.3},
 "top_location":{"code":"US","name":"United States","count":55},
 "series":[{"date":"2026-10-18","clicks":3}],"prev_series":[],
 "devices":[{"key":"mobile","clicks":70,"percentage":58.3,"prev_clicks":50}],
 "referrers":[{"referrer":"Direct","clicks":90,"percentage":75,"prev_clicks":0}],
 "countries":[{"code":"US","name":"United States","clicks":55,"percentage":45.8,"prev_clicks":40}],
 "cities":[{"name":"Austin","country":"US","clicks":20,"percentage":16.7,"prev_clicks":2}],
 "items":[{"type":"qr","id":5,"link_id":9,"title":"Menu","short_url":"http://localhost:8080/abc","clicks":60,"percentage":50,"prev_clicks":30}]}
```

`top_day` is the series bucket with the most clicks (first on ties; `null` when total is 0); `top_location` is the first country row (`null` when none); `change_pct` fields are `null` when the previous value is 0 or no comparison. Breakdowns use limit 200. `items` is every (link or QR) with clicks in the range, most first, limit 200.

**Overview JSON** (`GET /analytics/overview`): `{"this_week":{"from","to","total","series":[...]},"last_week":{"from","to","total","series":[...]},"change_pct":12.5|null,"top_performing":[item…5]}` — this week = last 7 days ending today, last week = the 7 days before (ruling: Bitly's "this week" is replicated as a rolling 7-day window), top performing = items over the last 30 days, top 5.

**Dashboard JSON:** `{"id","group_id","name","is_default","date_range","filter":{…},"layout":{…},"created_by":{"id","name"},"created_at","updated_at"}`; list `{"dashboards":[…],"total"}` sorted by `created_at` desc (`?sort=created|modified|name`).

**Compare link** (`GET /links/:id/stats?compare_link=ID`): `LinkStatsV2` gains `compare_link: {"id","title","short_url","total_range","series":[...]}|null`, series over the same window/unit/source.

---

### Task 1: Migration, model and DashboardRepository

**Files:**
- Create: `backend/migrations/006_dashboards.sql`, `backend/internal/model/dashboard.go`, `backend/internal/repository/dashboard_repo.go`, `backend/internal/repository/dashboard_repo_test.go`
- Modify: `backend/internal/repository/interfaces.go`, `backend/internal/testutil/db.go` (truncate `dashboards` before `groups`), `backend/internal/repository/migration_test.go` if it enumerates files

**Interfaces:**
- Produces `model.Dashboard{ID, GroupID, CreatedBy uint64; Name string; IsDefault bool; DateRange string; Filter model.JSONMap; Layout model.JSONMap; CreatedAt, UpdatedAt time.Time}` with `db` tags matching the columns.
- Produces `repository.DashboardRepository`:
  ```go
  type DashboardRepository interface {
      Create(ctx context.Context, d *model.Dashboard) error                       // sets ID, timestamps
      GetByID(ctx context.Context, id uint64) (*model.Dashboard, error)           // ErrDashboardNotFound
      ListByGroup(ctx context.Context, groupID uint64, sort string) ([]model.Dashboard, error) // sort: created|modified|name
      CountByGroup(ctx context.Context, groupID uint64) (int64, error)
      Update(ctx context.Context, d *model.Dashboard) error                       // name, date_range, filter, layout; ErrDashboardNotFound
      Delete(ctx context.Context, id uint64) error                                // ErrDashboardNotFound
      EnsureDefault(ctx context.Context, groupID, userID uint64, layout model.JSONMap) (*model.Dashboard, error) // returns existing is_default row or inserts "Default"
      CreatorNames(ctx context.Context, userIDs []uint64) (map[uint64]string, error)
  }
  ```
  `ErrDashboardNotFound = errors.New("dashboard not found")` in `repository`. `EnsureDefault` must be safe under concurrent calls: `INSERT … WHERE NOT EXISTS (SELECT 1 FROM dashboards WHERE group_id=? AND is_default=1)` then re-select (ruling: no unique index on `(group_id,is_default)` because MariaDB would reject multiple non-default rows).

- [ ] **Step 1: Write the migration** with the SQL in Shared contracts; apply it to both databases (Global Constraints).
- [ ] **Step 2: Write failing repository tests** in `dashboard_repo_test.go` using `testutil` like `qr_repo_test.go`: `TestDashboardRepo_CreateGetListDelete` (create two, list sort created desc then name asc, delete → `ErrDashboardNotFound` on get), `TestDashboardRepo_EnsureDefault_Idempotent` (two calls return the same ID; `CountByGroup` 1), `TestDashboardRepo_Update_PersistsJSON` (filter/layout round-trip through `model.JSONMap`).
- [ ] **Step 3: Run tests, expect compile failure.**
- [ ] **Step 4: Implement model, interface, repository**; add `//go:generate mockgen` line next to the other repository mocks and run `go generate ./...`.
- [ ] **Step 5: Run `go test -p 1 ./internal/repository/...`, expect PASS.**
- [ ] **Step 6: Commit** `feat(analytics): dashboards table, model and repository`.

### Task 2: ClickRepository scope queries

**Files:**
- Modify: `backend/internal/repository/interfaces.go`, `backend/internal/repository/click_repo.go`, `backend/internal/repository/click_repo_test.go`

**Interfaces:**
- Produces, on `ClickRepository`:
  ```go
  type ClickScope struct { GroupID uint64; LinkIDs []uint64; Source string } // see Shared contracts
  CountScope(ctx, scope ClickScope, from, to time.Time) (int64, error)
  SeriesScope(ctx, scope ClickScope, from, to time.Time, unit string) ([]Bucket, error)   // zero-filled like Series
  BreakdownScope(ctx, scope ClickScope, from, to time.Time, facet string, limit int) ([]Row, error)
  ItemsScope(ctx, scope ClickScope, from, to time.Time, limit int) ([]ItemCount, error)  // GROUP BY link_id, source ORDER BY cnt DESC
  type ItemCount struct { LinkID uint64 `db:"link_id"`; Source string `db:"source"`; Count int64 `db:"cnt"` }
  ```
  Implementation: a `scopeWhere(scope, from, to) (string, []any, error)` helper used by all four, sharing `halfOpen`, the `seriesUnits` and `breakdownColumns` tables with the existing methods (refactor the existing bodies to call shared helpers rather than duplicating SQL).

- [ ] **Step 1: Write failing tests** `TestClickRepo_Scope_GroupVsIDs` (clicks on 3 links in 2 groups; group scope counts only its links; id scope counts the listed ids; `Source:"not_qr"` excludes qr rows; empty non-nil `LinkIDs` → 0 and empty series buckets still zero-filled), `TestClickRepo_ItemsScope_GroupsByLinkAndSource` (a link with 2 link clicks + 3 qr scans yields two rows, qr first).
- [ ] **Step 2: Run, expect compile failure.**
- [ ] **Step 3: Implement**, `go generate ./...`.
- [ ] **Step 4: Run `go test -p 1 ./internal/repository/...`, expect PASS.**
- [ ] **Step 5: Commit** `feat(analytics): click scope queries (group or link set, source filter, items)`.

### Task 3: AnalyticsService (overview + dashboard data + export)

**Files:**
- Create: `backend/internal/service/analytics_service.go`, `backend/internal/service/analytics_service_test.go`
- Modify: `backend/internal/service/interfaces.go` (add `AnalyticsService` + mockgen line)

**Interfaces:**
- Consumes Task 2 scope methods, `repository.LinkRepository.GetByIDs`, `repository.QRRepository.GetByIDs`/`QRCodeIDsForLinks`, `getCountryName`, `csvSafe` (same package), `NormalizeStatsQuery`.
- Produces:
  ```go
  type AnalyticsService interface {
      Overview(ctx context.Context, groupID uint64) (*Overview, error)
      DashboardData(ctx context.Context, groupID uint64, filter DashboardFilter, q StatsQuery) (*DashboardData, error) // *StatsQueryError on bad q
      ExportZip(ctx context.Context, groupID uint64, name string, filter DashboardFilter, q StatsQuery) ([]byte, error)
  }
  type DashboardFilter struct { Type string `json:"type"`; IDs []uint64 `json:"ids,omitempty"` }
  type AnalyticsItem struct { Type string `json:"type"`; ID uint64 `json:"id"`; LinkID uint64 `json:"link_id"`; Title string `json:"title"`; ShortURL string `json:"short_url"`; Clicks int64 `json:"clicks"`; Percentage float64 `json:"percentage"`; PrevClicks int64 `json:"prev_clicks"` }
  type TopDay struct { Date string `json:"date"`; Count int64 `json:"count"`; PrevCount int64 `json:"prev_count"`; ChangePct *float64 `json:"change_pct"` }
  type TopLocation struct { Code, Name string; Count int64 }
  type DashboardData struct { … exactly the JSON in Shared contracts … }
  type OverviewWeek struct { From, To string; Total int64; Series []SeriesPoint }
  type Overview struct { ThisWeek, LastWeek OverviewWeek; ChangePct *float64; TopPerforming []AnalyticsItem }
  ```
  `NewAnalyticsService(clickRepo, linkRepo, qrRepo)` with `SetClock`. `scopeFor(groupID, filter) (repository.ClickScope, error)` maps the filter per Shared contracts (qr ids → their link ids via `qrRepo.GetByIDs`). Items: rows from `ItemsScope`; for each `(link_id, source)` row look up the link (`GetByIDs`) and `QRCodeIDsForLinks`; `source=='qr' && link has QR` → item `{type:"qr", id: qrID}`; otherwise merge into the link item `{type:"link", id: linkID}`; a link whose `Source=='qr'` (QR-owned link) is always the QR item; sort by clicks desc, apply `limit`, percentages against `total`; `prev_clicks` from the same query over the previous window keyed by `(type,id)`. Short URL built the same way `link_presenter.go` does (reuse its helper; export it if needed). `ExportZip` writes `summary.csv`, `engagements_over_time.csv`, `devices.csv`, `referrers.csv`, `locations.csv` (countries then a blank line and cities), `links_and_qr_codes.csv`, `top.csv` (top day + top location) through `csvSafe` on every string cell, using `archive/zip` into a `bytes.Buffer`.

- [ ] **Step 1: Write failing tests** with repository mocks: `TestAnalyticsService_DashboardData_ComputesTopDayAndChange` (series 3 buckets → top_day is the max; `change_pct` 20 for 120 vs 100; null when prev 0), `TestAnalyticsService_Items_SplitsQrScans` (Review Focus 4), `TestAnalyticsService_ScopeFor_Filters` (`all` → group scope; `links` → ids + `not_qr`; `qr` → mapped link ids + `qr`), `TestAnalyticsService_Overview_Windows` (clock 2026-10-24 → this week 10-18..10-24, last week 10-11..10-17), `TestAnalyticsService_ExportZip` (seven entries; a title starting with `=` is prefixed `'`).
- [ ] **Step 2: Run, expect compile failure.**
- [ ] **Step 3: Implement**, `go generate ./...`.
- [ ] **Step 4: Run `go test -p 1 ./internal/service/...`, expect PASS.**
- [ ] **Step 5: Commit** `feat(analytics): analytics service (overview, dashboard data, csv zip export)`.

### Task 4: DashboardService

**Files:**
- Create: `backend/internal/service/dashboard_service.go`, `backend/internal/service/dashboard_service_test.go`
- Modify: `backend/internal/service/interfaces.go` (+ mockgen), `backend/internal/service/activity_service.go` if it has an action allow-list (actions `dashboard.create`, `dashboard.update`, `dashboard.delete`, `dashboard.duplicate`)

**Interfaces:**
- Consumes Task 1 repository, Task 3 `DashboardFilter`, `repository.LinkRepository.GetByIDs`, `repository.QRRepository.GetByIDs`, `ActivityService.Log`, `UserRepository` for creator names (via `DashboardRepository.CreatorNames`).
- Produces:
  ```go
  type DashboardService interface {
      List(ctx context.Context, userID, groupID uint64, sort string) ([]DashboardView, error)   // EnsureDefault first
      Get(ctx context.Context, groupID, id uint64) (*DashboardView, error)                        // ErrDashboardNotFound when not in group
      Create(ctx context.Context, userID, groupID uint64, name string) (*DashboardView, error)    // default layout/filter/date_range; name trimmed, 1–255 chars else *ValidationError
      Update(ctx context.Context, userID, groupID, id uint64, patch DashboardPatch) (*DashboardView, error)
      Duplicate(ctx context.Context, userID, groupID, id uint64) (*DashboardView, error)         // "Copy of {name}", is_default false
      Delete(ctx context.Context, userID, groupID, id uint64) error                              // ErrDashboardDefault (409) for is_default
  }
  type DashboardPatch struct { Name *string; DateRange *string; Filter *DashboardFilter; Layout *DashboardLayout }
  type DashboardLayout struct { Modules []DashboardModule `json:"modules"` }
  type DashboardModule struct { Key string `json:"key"`; Viz string `json:"viz"` }
  type DashboardView struct { ID, GroupID uint64; Name string; IsDefault bool; DateRange string; Filter DashboardFilter; Layout DashboardLayout; CreatedBy struct{ID uint64; Name string}; CreatedAt, UpdatedAt time.Time } // json tags per Shared contracts
  var DefaultLayout = DashboardLayout{…seven modules in order…}
  var AllowedViz = map[string][]string{…}
  func ValidateLayout(l DashboardLayout) error; func (s *…) validateFilter(ctx, groupID, f DashboardFilter) error
  ```
  `ErrDashboardDefault = errors.New("the default dashboard cannot be deleted")`. `validateFilter`: type ∈ all|links|qr; ids 1–200 unique; every id exists, belongs to the group, and (links) has `source='link'`; else `*ValidationError{Msg}` (reuse the service package's existing validation error type if one exists — check `link_service.go`; otherwise add `ValidationError`). Date range ∈ the five values.

- [ ] **Step 1: Write failing tests** (mocks): `TestDashboardService_List_CreatesDefault`, `TestDashboardService_Get_WrongGroup` (repo returns group 9, caller group 7 → `ErrDashboardNotFound`), `TestDashboardService_Update_FilterValidation` (201 ids → error; foreign id → error; QR id under `links` → error; valid → repo.Update with merged fields + activity log), `TestDashboardService_Update_LayoutValidation` (missing key, duplicate key, bad viz), `TestDashboardService_Delete_DefaultConflict`, `TestDashboardService_Duplicate_NameAndFlags`.
- [ ] **Step 2: Run, expect compile failure.**
- [ ] **Step 3: Implement**, `go generate ./...`.
- [ ] **Step 4: Run `go test -p 1 ./internal/service/...`, expect PASS.**
- [ ] **Step 5: Commit** `feat(analytics): dashboard service with default dashboard, filter and layout validation`.

### Task 5: Handlers and routes

**Files:**
- Create: `backend/internal/handler/dashboard.go`, `backend/internal/handler/dashboard_test.go`
- Modify: `backend/internal/handler/analytics.go` (+ `analytics_test.go`), `backend/cmd/server/main.go`

**Interfaces:**
- Consumes Tasks 3–4 services; `parseStatsQuery` from `handler/stats.go` (export or reuse in package); `middleware.GetUserID/GetGroupID`.
- Produces routes under `api.Group("/analytics")` (auth + group middleware):
  - `GET /analytics/overview` → `analyticsHandler.Overview` (200 Overview JSON)
  - `GET /analytics/dashboards?sort=` → `{"dashboards":[…],"total":n}`
  - `POST /analytics/dashboards {name}` → 201 dashboard
  - `GET /analytics/dashboards/:id` → 200 | 404
  - `PATCH /analytics/dashboards/:id {name?,date_range?,filter?,layout?}` → 200 | 400 validation | 404
  - `POST /analytics/dashboards/:id/duplicate` → 201
  - `DELETE /analytics/dashboards/:id` → 204 | 404 | 409 default
  - `GET /analytics/dashboards/:id/data?from&to&compare&unit` → 200 DashboardData | 400 | 404 (the dashboard's stored filter is used; no filter override by query)
  - `GET /analytics/dashboards/:id/export?from&to&compare` → `application/zip`, `Content-Disposition: attachment; filename="{slug(name)}-{from}-{to}.zip"`

- [ ] **Step 1: Write failing handler tests** in the style of `analytics_test.go` (gomock services, `withGroup`, `doJSON`): `TestDashboardHandler_ListCreate`, `TestDashboardHandler_Get_OtherGroup404`, `TestDashboardHandler_Patch_Validation400`, `TestDashboardHandler_Delete_Default409`, `TestDashboardHandler_Data_PassesStoredFilter` (service `Get` returns filter `links [3]`; expect `DashboardData` called with that filter and a normalized query from `?from=2026-10-18&to=2026-10-24&compare=previous`), `TestDashboardHandler_Export_Headers`, `TestAnalyticsHandler_Overview`.
- [ ] **Step 2: Run, expect compile failure.**
- [ ] **Step 3: Implement handlers; wire services and routes in `main.go`** (construct `analyticsService`, `dashboardService` after `statsService`; keep `/analytics/today`).
- [ ] **Step 4: Run `go vet ./... && go test -p 1 ./...` (full suite), expect PASS.** Then start the backend and curl: create a dashboard, `GET …/data`, `GET …/export` → zip with 7 entries (`unzip -l`).
- [ ] **Step 5: Commit** `feat(api): analytics overview and dashboard endpoints`.

### Task 6: "Compare to a link" on link stats (backend)

**Files:**
- Modify: `backend/internal/service/stats_service.go` (+ `_test.go`), `backend/internal/handler/stats.go` (+ `_test.go`)

**Interfaces:**
- Produces `StatsQuery.CompareLinkID uint64` (0 = none); `LinkStatsV2.CompareLink *CompareLinkStats` with `CompareLinkStats{ID uint64; Title string; ShortURL string; TotalRange int64; Series []SeriesPoint}` (json per Shared contracts; `compare_link: null` when unset). Handler reads `?compare_link=ID` (non-numeric → 400 "compare_link must be a link id"; equal to `:id` → 400 "compare_link must differ from the link"). Service: the compare link must exist and be owned by `userID` else `ErrNotLinkOwner`→404 like the main link; the QR stats path (`qrHandler.Stats`) ignores `compare_link`.

- [ ] **Step 1: Write failing tests** `TestStatsService_CompareLink_AddsSeries`, `TestStatsService_CompareLink_OtherOwner404`, `TestStatsHandler_CompareLink_SameID400`.
- [ ] **Step 2: Run, expect FAIL.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run `go test -p 1 ./internal/service/... ./internal/handler/...`, expect PASS.**
- [ ] **Step 5: Commit** `feat(stats): compare a link's engagements with another link`.

### Task 7: Frontend types, API client, hooks, chart components

**Files:**
- Modify: `frontend/src/types/index.ts`, `frontend/src/lib/api.ts`, `frontend/src/lib/hooks/keys.ts`, `frontend/src/lib/hooks/index.ts`, `frontend/src/components/charts/TimeSeriesChart.tsx`, `frontend/package.json`
- Create: `frontend/src/lib/hooks/useAnalytics.ts`, `frontend/src/lib/hooks/useDashboards.ts`, `frontend/src/components/charts/DoughnutChart.tsx`, `frontend/src/components/charts/BarBreakdownChart.tsx`, `frontend/src/components/charts/DoughnutChart.test.tsx`, `frontend/src/components/analytics/modules.ts`, `frontend/src/components/analytics/modules.test.ts`

**Interfaces:**
- Produces types mirroring Shared contracts: `Dashboard`, `DashboardFilter`, `DashboardLayout`, `DashboardModule`, `ModuleKey`, `ModuleViz`, `DashboardData`, `AnalyticsItem`, `AnalyticsOverview`, `DashboardPatch`, `DashboardSort = 'created'|'modified'|'name'`, `DashboardDateRange`; `LinkStatsQuery.compare_link?: number`; `LinkStatsV2.compare_link: CompareLinkStats | null`.
- `api`: `getAnalyticsOverview()`, `listDashboards(sort)`, `getDashboard(id)`, `createDashboard(name)`, `updateDashboard(id, patch)`, `duplicateDashboard(id)`, `deleteDashboard(id)`, `getDashboardData(id, {from,to,compare,unit})`, `exportDashboard(id, {from,to,compare}): Promise<Blob>`. Add `/api/analytics` paths to the proxy allowlist in `src/app/api/[...path]/route.ts` if it is path-based (check).
- Hooks: `useAnalyticsOverview()`, `useDashboards(sort)`, `useDashboard(id)`, `useDashboardData(id, query)`, `useDashboardMutations()` → `{create, update, duplicate, remove}` invalidating `['dashboards']` and `['dashboard', id]`; keys `dashboards(groupId, sort)`, `dashboard(id)`, `dashboardData(id, query)`, `overview(groupId)`.
- `modules.ts`: `MODULES: Record<ModuleKey, {title: string; allowed: ModuleViz[]}>` with the seven titles from Global Constraints and allowed viz per Shared contracts; `DEFAULT_LAYOUT`; `DATE_RANGE_OPTIONS: {value: DashboardDateRange; label: 'Today'|'Last 7 days'|…}[]`; `rangeFor(dateRange, now): {from,to}` (YYYY-MM-DD UTC-safe via `format`).
- `DoughnutChart({rows: {label, value}[], height?})` recharts `PieChart` with `innerRadius` 60%, legend table on the right (label · value · %), palette `DOUGHNUT_PALETTE`, `NoData` when empty. `BarBreakdownChart({rows: {label, value, prev?}[], compare?, height?})` horizontal bars, current `#2A5BD7`, previous `#A5B8E8`.
- `TimeSeriesChart` gains optional `extraSeries?: { name: string; points: SeriesPoint[] }` drawn as a solid `#47B5A1` line (used by Task 10) and `variant?: 'line'|'bar'` (bar uses `BarChart` with the same colours).

- [ ] **Step 1: `npm i @dnd-kit/core @dnd-kit/sortable @dnd-kit/utilities`** (pin the versions npm resolves).
- [ ] **Step 2: Write failing tests** `modules.test.ts` (`rangeFor('last_7_days', 2026-10-24)` → 10-18..10-24; `DEFAULT_LAYOUT` has 7 unique keys; `MODULES.devices.allowed` equals `['doughnut','bar','table']`) and `DoughnutChart.test.tsx` (renders legend rows with percentages; `NoData` for empty).
- [ ] **Step 3: Implement types, api, hooks, charts, modules.**
- [ ] **Step 4: `npx vitest run && npx tsc --noEmit && npm run lint`, expect PASS.**
- [ ] **Step 5: Commit** `feat(frontend): analytics types, api, hooks, doughnut/bar charts, module registry`.

### Task 8: Analytics overview page

**Files:**
- Modify: `frontend/src/app/dashboard/analytics/page.tsx`, `frontend/src/components/home/TodayStats.tsx`
- Create: `frontend/src/components/analytics/OverviewDashboards.tsx`, `PerformanceOverview.tsx`, `TopPerforming.tsx`, `DashboardNameDialog.tsx`, `OverviewDashboards.test.tsx`

**Interfaces:**
- Consumes Task 7 hooks and `StatCard`, `TimeSeriesChart`, `Favicon`, `formatNumber`, `formatChangePct`.
- Produces page `/dashboard/analytics` (spec §5.8 Overview): `PageHeader` title "Analytics"; section "Dashboards": grid of tiles — first tile "+ Create dashboard" (opens `DashboardNameDialog`: input "Dashboard name", buttons Cancel / "Create dashboard"; on success `router.push` to the new dashboard), then one tile per dashboard (name, "Modified on {MMM d, yyyy}"), at most 3 tiles + link "View all →" to `/dashboard/analytics/dashboards`; section "Performance overview": card with "TOTAL ENGAGEMENTS · THIS WEEK" label, big number, ▲/▼ `change_pct` badge with text "vs last week", `TimeSeriesChart` with series = this week, `prevSeries` = last week (legend "This week" / "Last week", `seriesName` props); section "Top performing": list of 5 rows (type icon link/QR, favicon, title → link to `/dashboard/links/{link_id}` or `/dashboard/qr-codes/{id}`, short url, "{n} engagements"), `EmptyState` "No engagements in the last 30 days" when empty.
- `TodayStats`: the chip row links to `/dashboard/analytics` (already) — ensure the card's heading and chips keep working; no new behaviour.

- [ ] **Step 1: Write failing test** `OverviewDashboards.test.tsx`: renders "+ Create dashboard", 3 tiles for 4 dashboards and "View all →"; clicking create opens the dialog and calls the mutation with the typed name.
- [ ] **Step 2: Implement.**
- [ ] **Step 3: `npx vitest run && npx tsc --noEmit && npm run lint`, expect PASS.**
- [ ] **Step 4: Commit** `feat(frontend): analytics overview page`.

### Task 9: All dashboards page

**Files:**
- Create: `frontend/src/app/dashboard/analytics/dashboards/page.tsx`, `frontend/src/components/analytics/DashboardRow.tsx`, `DashboardActionsMenu.tsx`, `DashboardRow.test.tsx`

**Interfaces:**
- Consumes Task 7 hooks/mutations, Task 8 `DashboardNameDialog` (reuse with `mode: 'create'|'rename'`), `ConfirmDialog`, `LinksToolbar`-style toolbar (do not reuse `LinksToolbar`; a small local toolbar).
- Produces `/dashboard/analytics/dashboards` (spec §5.8 All dashboards): header "Dashboards" with back link to `/dashboard/analytics`; toolbar: "Sort by: Date created ▾" select (Date created | Date modified | Name → `?sort=`), "Add filters" button (disabled, tooltip "Coming in a later phase" — ruling: dashboard list filters are out of scope), primary "Create new dashboard"; rows: avatar initial of creator, name (link to the dashboard), "Modified on {date}", "Created by {name}", date range label, filter summary ("Showing data for all links and QR Codes" | "Showing data for {n} links" | "Showing data for {n} QR Codes"), ⋯ menu Rename / Duplicate / Delete (Delete disabled with tooltip "The default dashboard cannot be deleted" when `is_default`; confirm dialog "Delete dashboard?" / "This permanently deletes the dashboard." / "Delete").

- [ ] **Step 1: Write failing test** `DashboardRow.test.tsx`: filter summary strings for the three filter types; Delete disabled for default.
- [ ] **Step 2: Implement.**
- [ ] **Step 3: Checks pass; commit** `feat(frontend): all dashboards page with sort, rename, duplicate, delete`.

### Task 10: Dashboard page (controls, modules, reorder, visualization, filter, export)

**Files:**
- Create: `frontend/src/app/dashboard/analytics/dashboards/[id]/page.tsx`, `frontend/src/components/analytics/DashboardControls.tsx`, `DashboardFilterDialog.tsx`, `ModuleCard.tsx`, `ModuleGrid.tsx`, `dashboardCsv.ts`, `dashboardCsv.test.ts`, `ModuleCard.test.tsx`, `DashboardFilterDialog.test.tsx`

**Interfaces:**
- Consumes Task 7 everything, `DateRangeDialog` (`maxDays={366}`), `useLinks`/`useQrCodes` for the filter picker, `downloadBlob`, `csvRow`/`csvCell`.
- Produces `/dashboard/analytics/dashboards/[id]` (spec §5.8 Dashboard):
  - Header: "← Dashboards" link; editable title (click → inline input, Enter/blur saves via `update({name})`, Escape cancels; toast "Dashboard renamed"); right: "Download CSV" button → `api.exportDashboard` → `downloadBlob('{name}-{from}-{to}.zip')`.
  - Controls row: date range button labelled with `rangeLabel` (presets from `DATE_RANGE_OPTIONS` + "Custom…" opening `DateRangeDialog`; a preset persists `date_range` through `update`, custom is URL-only `?from&to`), comparison select "Previous period" | "No comparison" (`?compare=`), "Filter" button opening `DashboardFilterDialog`, summary text "Showing data for all links and QR Codes" / "… {n} links" / "… {n} QR Codes".
  - `DashboardFilterDialog`: title "Filter", segmented Type "Links" | "QR Codes" | "All", searchable checklist (uses `useLinks({q})` / `useQrCodes({q})`, page size 50, "Load more"), counter "{n}/200 selected", disabled checkboxes past 200, buttons "Cancel" / "Apply filters" → `update({filter})`.
  - `ModuleGrid`: two-column CSS grid of `ModuleCard`s ordered by `layout.modules`, `@dnd-kit/sortable` with a drag handle (`GripVertical` icon, `aria-label="Reorder {title}"`), drop persists `update({layout})` (optimistic order); keyboard sensor enabled.
  - `ModuleCard`: title, right: CSV icon (`aria-label="Download {title} CSV"` → `dashboardCsv.moduleCsv(key, data)` → `downloadBlob('{slug}.csv')`), ⋯ menu "Change visualization" submenu listing `MODULES[key].allowed` with the current one checked → `update({layout})`. Renders per `(key, viz)`: scorecards (`StatCard` with date / flag+country and "▲ x% vs previous period" when comparing), line/bar (`TimeSeriesChart variant`), doughnut (`DoughnutChart`), bar breakdown (`BarBreakdownChart`), table (`BreakdownTable` 10 per page, "vs prev." column when comparing). `items` table columns Type (icon) · Link (title + short url, linking to the link/QR page) · Engagements · % · vs prev; its CSV icon exports all rows.
  - Loading: skeleton grid; error: `EmptyState` with Retry; 404 → "Dashboard not found" with link back.
  - `dashboardCsv.ts`: `moduleCsv(key: ModuleKey, data: DashboardData, compare: boolean): string` (headers per module; `Engagements`, `%`, `Previous` when compare).

- [ ] **Step 1: Write failing tests**: `dashboardCsv.test.ts` (devices CSV header `Device,Engagements,%` and rows; items CSV includes Type and Link columns), `ModuleCard.test.tsx` (devices module offers Doughnut/Bar/Table in its menu; top_day scorecard shows "No data" when `top_day` is null), `DashboardFilterDialog.test.tsx` (selecting 2 links and Apply calls `onApply({type:'links', ids:[…]})`; the 201st checkbox is disabled).
- [ ] **Step 2: Implement.**
- [ ] **Step 3: `npx vitest run && npm run lint && npx tsc --noEmit && npm run build`, expect PASS.**
- [ ] **Step 4: Commit** `feat(frontend): dashboard page with modules, reorder, visualization switch, filter and export`.

### Task 11: "Compare to a link" on link details (frontend)

**Files:**
- Create: `frontend/src/components/links/CompareLinkDialog.tsx`, `CompareLinkDialog.test.tsx`
- Modify: `frontend/src/components/links/LinkAnalytics.tsx` (+ `.test.tsx`), `frontend/src/lib/hooks/useLinkStats.ts` if the query type needs `compare_link`

**Interfaces:**
- Consumes Task 6 API (`compare_link`), Task 7 `TimeSeriesChart.extraSeries`, `useLinks({q})`.
- Produces: in `LinkAnalytics` for `subject.kind === 'link'` only, a "Compare to a link" button next to the comparison select; URL state `?compare_link=ID`; `CompareLinkDialog` (title "Compare to a link", search input "Search links", radio list of the group's links excluding the current one, "Compare" button); when set, a chip "{compare title} ×" replaces the button, the chart shows the second link as `extraSeries` named by its title, and the "Engagements ({range})" scorecard row gains a third card "{compare title}" with its `total_range`. The CSV export adds a "Compared link" section (date, clicks).

- [ ] **Step 1: Write failing tests**: `CompareLinkDialog.test.tsx` (excludes the current link; Compare returns the chosen id), `LinkAnalytics.test.tsx` addition (with `compare_link` in the URL the hook is called with `compare_link` and the chip renders).
- [ ] **Step 2: Implement.**
- [ ] **Step 3: Checks pass; commit** `feat(frontend): compare a link's analytics with another link`.

### Task 12: Docs

**Files:**
- Modify: `README.md` (Phase 3 section: analytics endpoints, dashboards, export, `compare_link`; migration 006 note), `backend/config.example.yaml` only if a config key was added (none expected)

- [ ] **Step 1: Write the README section; verify every listed route exists in `main.go`.**
- [ ] **Step 2: Commit** `docs: phase 3 analytics`.

---

## Self-review notes

- Spec coverage: §5.8 overview (T8), all dashboards (T9), dashboard page incl. modules/viz/reorder/filter/CSV (T10), default dashboard (T1/T4), §5.4 compare to a link (T6/T11), §6.2 endpoints (T5), §6.4 queries (T2), §5.1 Today's stats already shipped in Phase 1 (T8 touch only). Out of scope by ruling: PDF export, dashboard-list "Add filters" (disabled), persisting custom ranges.
- Type consistency: `DashboardFilter{Type, IDs}` is the one filter shape across repo JSON, service, handler and frontend; `AnalyticsItem` is reused by overview and dashboard data; module keys/viz strings are identical in `AllowedViz` (Go) and `MODULES` (TS).

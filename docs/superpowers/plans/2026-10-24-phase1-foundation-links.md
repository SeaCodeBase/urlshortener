# Phase 1: Foundation, Links, Home, Settings — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the Bitly-style application shell, the complete Links feature set (list/filters/bulk/hide/export/create/details/edit/back-half/UTM/expiration/routing), Home quick create, global search, and the Settings pages (profile, account, groups, API tokens, activity log), on top of a migrated data model with groups, tags and click sources.

**Architecture:** Backend stays Go/Gin layered as repository → service → handler with `go.uber.org/mock` mocks for unit tests and MariaDB for repository tests. A new migration adds groups/tags/archived/sources. Group scoping comes from an `X-Group-ID` header resolved by middleware. Frontend stays Next.js 16 app router + shadcn + Zustand, adds TanStack Query for server state, and gets a new `AppShell` with the Bitly visual system.

**Tech Stack:** Go 1.25, Gin, sqlx, MariaDB 10.11, Redis 7, go-redis, testify, mockgen; Next.js 16, React 19, Tailwind 4, shadcn/ui (new-york), TanStack Query 5, Zustand 5, recharts 3, lucide-react, sonner, date-fns, react-day-picker (via shadcn calendar), Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-24-bitly-clone-design.md` (sections 3, 4, 5.1–5.5, 5.11–5.13, 6, 7, 8 phase 1)

## Global Constraints

- Repo root: `/Users/jose/GolandProjects/urlshortener`; backend in `backend/`, frontend in `frontend/`. Work on branch `feat/bitly-clone`.
- Go tests: `cd backend && TEST_DATABASE_DSN="urlshortener:urlshortener_pass@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC" go test -p 1 ./...` must stay green (`-p 1` is required: packages share the test DB). Apply new migrations to BOTH `urlshortener` and `urlshortener_test` databases with `docker exec -i us-mariadb mariadb -uroot -proot_secret_123 <db> < backend/migrations/<file>.sql` before running repository tests.
- Regenerate mocks after changing any interface: `cd backend && go generate ./...` (mockgen is installed in `~/go/bin`).
- Frontend checks: `cd frontend && npm run lint && npx tsc --noEmit && npm run build` must pass; `npx vitest run` must pass.
- No billing/upgrade/AI/integrations UI anywhere (spec §1 exclusions).
- Colours and copy are fixed by spec §3 and §5; labels must match exactly (e.g. "Create link", "Filter by created date", "Add filters", "Show: Active", "You've reached the end of your links", "Your link is ready!").
- Primary colour `#2A5BD7`, canvas `#F4F6FA`, navy text `#031F39`, "Create new" `#0F2E8A`, logo orange `#EE6123`.
- Back-half rules: 1–255 chars, regex `^[A-Za-z0-9_-]+$`, case-sensitive, unique per domain (NULL domain = default).
- API responses keep the existing `{"error": "..."}` error shape; list responses keep `{links,total,page,total_pages}`.
- Commit after every task with a conventional-commit message ending in `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.

## Review Focus

1. A link with `archived=1` must still redirect (hide ≠ disable) — test in Task 4 (redirect service) and Task 11 (list excludes it by default).
2. Deleting a link that has been redirected (`redirect_count>0`) or has a custom back-half must return 409 with the spec message — Task 7.
3. Custom back-half uniqueness is per domain: the same back-half on two different domains is allowed, and on the same domain is a 409 — Task 7.
4. `X-Group-ID` for a group the user does not own must be rejected with 403, and a missing header must fall back to the default group — Task 3.
5. Expired links: redirect goes to the domain's `not_found_redirect` when set, otherwise the HTML error page with HTTP 410 — Task 4.

---

## File map (backend)

- Create `backend/migrations/002_bitly_clone.sql` — schema changes for this phase only (groups, users cols, links cols, link_tags, clicks cols, dynamic_routing_rules, api_tokens, activity_log, domains cols).
- Create `backend/internal/model/group.go`, `routing_rule.go`, `api_token.go`, `activity.go`; modify `model/link.go`, `model/user.go`, `model/click.go`, `model/domain.go`.
- Create `backend/internal/repository/group_repo.go`, `tag_repo.go`, `routing_rule_repo.go`, `api_token_repo.go`, `activity_repo.go`; modify `link_repo.go`, `click_repo.go`, `user_repo.go`, `domain_repo.go`, `interfaces.go` (+ mocks).
- Create `backend/internal/service/group_service.go`, `tag_service.go`, `routing_service.go`, `token_service.go`, `activity_service.go`, `title_fetcher.go`, `utm.go`; modify `link_service.go`, `redirect_service.go`, `stats_service.go`, `auth_service.go`, `interfaces.go`.
- Create `backend/internal/handler/group.go`, `tag.go`, `token.go`, `activity.go`, `search.go`; modify `link.go`, `stats.go`, `redirect.go`, `auth.go`, `domain.go`.
- Create `backend/internal/middleware/group.go`; modify `middleware/auth.go` (API tokens).
- Modify `backend/internal/config/config.go` (urls.frontend_url, urls.redirect_host, urls.redirect_ip, ui.favicon_service), `config.example.yaml`, `config.docker.yaml`.
- Modify `backend/cmd/server/main.go` (wiring, routes), `backend/internal/worker/click_flusher.go` (os, source), `backend/internal/util/useragent.go` (OS).
- Create `backend/internal/handler/templates/error.html` (embedded).

## File map (frontend)

- Modify `src/app/globals.css` (tokens), `src/app/layout.tsx` (Inter font, QueryProvider), `src/lib/api.ts` (new endpoints + group header), `src/types/index.ts`, `src/stores/auth.ts` (activeGroupId).
- Create `src/lib/query.tsx` (QueryClientProvider), `src/lib/format.ts` (dates/numbers), `src/lib/hooks/*.ts` (useLinks, useLink, useTags, useGroups, useDomains, useStats, useTokens, useActivity).
- Create `src/components/layout/AppShell.tsx`, `Sidebar.tsx` (rewrite), `TopBar.tsx` (replaces TopNav), `GroupSwitcher.tsx`, `GlobalSearch.tsx`, `UserMenu.tsx`, `CreateNewMenu.tsx`.
- Create `src/components/common/PageHeader.tsx`, `EmptyState.tsx`, `ConfirmDialog.tsx`, `StatCard.tsx`, `CopyButton.tsx`, `Favicon.tsx`.
- Create `src/components/links/LinksToolbar.tsx` (rewrite), `LinkCard.tsx` (rewrite), `LinkCardCompact.tsx`, `LinkActionsMenu.tsx`, `FiltersDialog.tsx`, `DateRangeDialog.tsx`, `BulkTagDialog.tsx`, `TagPicker.tsx`, `TagEditor.tsx`, `ShareDialog.tsx`, `ShortLinkField.tsx`, `UtmBuilder.tsx`, `ExpirationField.tsx`, `RoutingRulesEditor.tsx`, `LinkForm.tsx`, `LinkReadyDialog.tsx`.
- Create `src/components/charts/TimeSeriesChart.tsx`, `BreakdownTable.tsx`, `DeviceBarChart.tsx`; keep existing charts.
- Create/modify pages: `src/app/dashboard/page.tsx` (Home), `links/page.tsx`, `links/create/page.tsx`, `links/[id]/page.tsx` (details; replaces `stats`), `links/[id]/edit/page.tsx`, `settings/layout.tsx`, `settings/profile/page.tsx`, `settings/account/page.tsx`, `settings/groups/page.tsx`, `settings/groups/[id]/page.tsx`, `settings/api/page.tsx`, `settings/activity-log/page.tsx`, `(auth)/login/page.tsx`, `(auth)/register/page.tsx`, `(auth)/layout.tsx`.
- Add shadcn: `switch select dropdown-menu popover calendar tabs badge tooltip sheet command separator skeleton progress radio-group textarea avatar alert-dialog scroll-area`.
- Add deps: `@tanstack/react-query` (present), `date-fns`, `react-day-picker`; dev: `vitest @vitejs/plugin-react @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom`.
- Create `frontend/vitest.config.ts`, `frontend/src/test/setup.ts`.

---

### Task 1: Migration 002 and models

**Files:**
- Create: `backend/migrations/002_bitly_clone.sql`
- Create: `backend/internal/model/group.go`, `backend/internal/model/routing_rule.go`, `backend/internal/model/api_token.go`, `backend/internal/model/activity.go`
- Modify: `backend/internal/model/link.go`, `model/user.go`, `model/click.go`, `model/domain.go`
- Test: `backend/internal/repository/migration_test.go`

**Interfaces:**
- Produces: `model.Group{ID, UserID, Name, DefaultDomainID *uint64, CreatedAt, UpdatedAt}`; `model.Link` gains `GroupID uint64 db:"group_id"`, `Archived bool db:"archived"`, `HasCustomBackHalf bool db:"has_custom_back_half"`, `RedirectCount int db:"redirect_count"`, `ParentLinkID *uint64 db:"parent_link_id"`, `Source string db:"source"`; `model.LinkWithStats` gains `Tags []string json:"tags"`, `QRCodeID *uint64`, `FaviconURL string`; `model.User` gains `DefaultGroupID *uint64`, `AccountName *string`, `HomeChecklistDismissed bool`; `model.Click` gains `Source string`, `OS string`; `model.Domain` gains `GroupID *uint64, VerificationToken string, VerificationStatus string, VerificationError *string, VerifiedAt NullTime, LastCheckedAt NullTime, RootRedirect *string, NotFoundRedirect *string`; `model.RoutingRule{ID, LinkID, Position int, LongURL string, DeviceMatch []string, OSMatch []string, CountryMatch []string, CountryExclude []string}` with JSON columns via `model.StringList` (implements `sql.Scanner`/`driver.Valuer`); `model.APIToken{ID, UserID, Name, TokenHash, TokenPrefix, LastUsedAt NullTime, CreatedAt, RevokedAt NullTime}`; `model.ActivityEntry{ID, UserID, GroupID, Action, EntityType string, EntityID uint64, Metadata model.JSONMap, CreatedAt}`.

- [ ] **Step 1: Write the migration** `002_bitly_clone.sql` exactly implementing spec §6.1 for: `groups`, `users` (+default_group_id, account_name, home_checklist_dismissed), `links` (+group_id NOT NULL DEFAULT 0, archived, has_custom_back_half, redirect_count, parent_link_id, source ENUM('link','qr','page')), `link_tags`, `clicks` (+source, os), `dynamic_routing_rules`, `api_tokens`, `activity_log`, `domains` (+group_id, verification_*, root_redirect, not_found_redirect; existing rows set `verification_status='ok'`). Backfill: insert one group per user named `SUBSTRING_INDEX(email,'@',1)`, set `users.default_group_id`, set `links.group_id`, then `ALTER TABLE links ADD FOREIGN KEY (group_id) REFERENCES groups(id)`. Add `INDEX idx_links_group_created (group_id, created_at)`, `INDEX idx_links_group_archived (group_id, archived)`.
- [ ] **Step 2: Write `migration_test.go`** (package `repository_test`): `TestMigration002_Backfill` — insert a user and a link via raw SQL into `urlshortener_test`, assert `SELECT COUNT(*) FROM groups WHERE user_id=?` is 1 after the migration has been applied (the test assumes the migration already ran; it asserts the columns exist by selecting `group_id, archived, source` from links).
- [ ] **Step 3: Apply migration** to both databases with the `docker exec` command from Global Constraints; run test. Expected: PASS.
- [ ] **Step 4: Update models** as listed in Interfaces, including `StringList` and `JSONMap` helper types in `backend/internal/model/json.go` with unit tests `TestStringList_ScanValue` (round-trips `["a","b"]`, NULL → empty slice).
- [ ] **Step 5: Update `testutil.SetupTestDB`** to also clean `link_tags, dynamic_routing_rules, api_tokens, activity_log, groups, domains` (order respects FKs).
- [ ] **Step 6: `go build ./... && go test -p 1 ./internal/model/... ./internal/repository/...`** Expected: PASS.
- [ ] **Step 7: Commit** `feat(db): migration 002 with groups, tags, archived links, click sources`.

### Task 2: Group repository & service, user default group on register

**Files:**
- Create: `backend/internal/repository/group_repo.go`, `backend/internal/service/group_service.go`
- Modify: `repository/interfaces.go`, `service/interfaces.go`, `repository/user_repo.go`, `service/auth_service.go`
- Test: `repository/group_repo_test.go`, `service/group_service_test.go`, `service/auth_service_unit_test.go`

**Interfaces:**
- Produces: `GroupRepository{Create(ctx,*Group) error; GetByID(ctx,id)(*Group,error); ListByUserID(ctx,userID)([]Group,error); Update(ctx,*Group) error; CountByUserID(ctx,userID)(int,error)}`; `GroupService{Create(ctx,userID,name string)(*Group,error); List(ctx,userID)([]Group,error); Get(ctx,userID,groupID)(*Group,error); Update(ctx,userID,groupID,UpdateGroupInput{Name *string; DefaultDomainID *uint64; ClearDefaultDomain bool})(*Group,error); ResolveActive(ctx,userID uint64, headerGroupID *uint64)(*Group,error)}`; errors `ErrGroupNotFound`, `ErrNotGroupOwner`. `UserRepository` gains `UpdateDefaultGroup(ctx,userID,groupID) error`, `UpdateAccount(ctx,userID uint64, accountName *string, dismissed *bool) error`. `AuthService.Register` creates the group and sets `default_group_id`.

- [ ] **Step 1: Tests first** — `TestGroupRepository_CreateAndList`, `TestGroupService_ResolveActive_HeaderOwned`, `TestGroupService_ResolveActive_HeaderForeign` (expects `ErrNotGroupOwner`), `TestGroupService_ResolveActive_NoHeaderUsesDefault`, `TestAuthServiceImpl_Register_CreatesDefaultGroup` (mock expects `GroupRepository.Create` and `UpdateDefaultGroup`).
- [ ] **Step 2: Run** `go test -p 1 ./internal/repository/ ./internal/service/ -run 'Group|Register'` Expected: FAIL (undefined).
- [ ] **Step 3: Implement** repository + service; `AuthService` constructor gains `groupRepo repository.GroupRepository` parameter (update all callers incl. tests and `main.go`); regenerate mocks.
- [ ] **Step 4: Run** same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(groups): group repository/service and default group on register`.

### Task 3: Group middleware and API-token auth

**Files:**
- Create: `backend/internal/middleware/group.go`, `backend/internal/repository/api_token_repo.go`, `backend/internal/service/token_service.go`, `backend/internal/handler/token.go`
- Modify: `middleware/auth.go`, `repository/interfaces.go`, `service/interfaces.go`, `cmd/server/main.go`
- Test: `middleware/group_test.go`, `middleware/auth_test.go`, `service/token_service_test.go`, `handler/token_test.go`

**Interfaces:**
- Produces: `middleware.GroupMiddleware(groupSvc service.GroupService) gin.HandlerFunc` sets `c.Set("group_id", uint64)`; `middleware.GetGroupID(c) uint64`. `AuthMiddleware(authService, tokenSvc service.TokenService)`: if bearer token starts with `us_` it is validated via `tokenSvc.Validate(ctx, raw) (userID uint64, err)` (SHA-256 hash lookup, revoked → 401, updates `last_used_at` at most once per minute), else JWT as today. `TokenService{Create(ctx,userID,name,password string)(raw string, *APIToken, error); List(ctx,userID)([]APIToken,error); Revoke(ctx,userID,id) error; Validate(ctx,raw)(uint64,error)}`; raw token format `us_` + 40 base62 chars; `TokenPrefix` = first 8 chars after `us_`. Handler routes: `GET /api/api-tokens`, `POST /api/api-tokens {name,password}` → 201 `{token, id, name, prefix, created_at}` (token only here), `DELETE /api/api-tokens/:id` → 204.

- [ ] **Step 1: Tests** — `TestGroupMiddleware_Header403` (foreign group → 403 `{"error":"group not found"}`), `TestGroupMiddleware_DefaultWhenMissing`, `TestAuthMiddleware_APIToken_Valid`, `TestAuthMiddleware_APIToken_Revoked401`, `TestTokenService_Create_WrongPassword` (returns `service.ErrInvalidCredentials`), `TestTokenService_Validate_Unknown401`, `TestTokenHandler_Create_ReturnsRawOnce`.
- [ ] **Step 2: Run, expect FAIL.**
- [ ] **Step 3: Implement**; wire in `main.go`: `api.Use(authMiddleware)` groups get `middleware.GroupMiddleware(groupService)` after auth for `/links`, `/tags`, `/domains`, `/search`, `/activity-log`, `/groups` (groups route itself does not require the header).
- [ ] **Step 4: Run, expect PASS. `go vet ./...` clean.**
- [ ] **Step 5: Commit** `feat(auth): group scoping middleware and API access tokens`.

### Task 4: Redirect server v2 (archived, sources, routing rules, error page, root/404 redirects, OS)

**Files:**
- Create: `backend/internal/repository/routing_rule_repo.go`, `backend/internal/service/routing_service.go`, `backend/internal/handler/templates/error.html`, `backend/internal/handler/errorpage.go`
- Modify: `service/redirect_service.go`, `handler/redirect.go`, `service/click_service.go`, `worker/click_flusher.go`, `util/useragent.go`, `repository/click_repo.go` (BatchInsert adds source, os), `cmd/server/main.go`
- Test: `service/redirect_service_test.go`, `service/routing_service_test.go`, `handler/redirect_test.go`, `util/useragent_test.go`

**Interfaces:**
- Produces: `RedirectService.Resolve(ctx, host, code string) (*ResolvedLink, error)` where `ResolvedLink{LinkID uint64; LongURL string; DomainID *uint64; Rules []model.RoutingRule; ExpiredRedirect string}`; errors `ErrLinkNotFound`, `ErrLinkExpired` (with `ResolvedLink.ExpiredRedirect` populated when the domain has `not_found_redirect`), `ErrLinkInactive`. Cache value JSON now includes `archived`, `rules`, `expires_at`, `not_found_redirect`. `RoutingService.Pick(rules []model.RoutingRule, req RoutingRequest{DeviceType, OS, Country string}) (url string, matched bool)` — first rule where all non-empty lists match (`country_exclude` must not contain the country). `util.ParseUserAgent` returns `UAResult{Browser, DeviceType, OS}` with `DeviceType ∈ desktop|mobile|tablet|unknown` (tablet via `ua.Mobile()` + "iPad|Tablet" substring) and `OS` from `ua.OSInfo().Name`. `ClickEvent` gains `Source string` (`link|qr|page`, from query `r=qr|page`) and the flusher writes `OS`. Redirect handler: `?r=` parsed; `GET /` on a host with `root_redirect` → 302; expired → 302 to `ExpiredRedirect` if set else HTML `error.html` with status 410 and heading "This link has expired"; not found → HTML 404 "Link not found". `RedirectService.InvalidateCache(ctx, host, code)` unchanged; add `InvalidateLink(ctx, link *model.Link, domainName string)` helper used by handlers.

- [ ] **Step 1: Tests** — `TestRedirectService_ArchivedStillRedirects`, `TestRedirectService_ExpiredUsesDomain404`, `TestRoutingService_Pick_FirstMatchWins`, `TestRoutingService_Pick_CountryExclude`, `TestRoutingService_Pick_NoMatchReturnsFalse`, `TestRedirectHandler_SourceQR` (asserts recorded `ClickEvent.Source=="qr"` through a fake clickRecorder interface), `TestRedirectHandler_RootRedirect`, `TestParseUserAgent_iPadIsTablet`, `TestParseUserAgent_OS`.
- [ ] **Step 2: Run, expect FAIL.**
- [ ] **Step 3: Implement.** Introduce `handler.ClickRecorder` interface `{RecordClick(ctx, service.ClickEvent) error}` so the handler test can fake it. Embed `error.html` with `//go:embed`. Error page style: white card on `#F4F6FA`, navy heading, blue link "Go to homepage" → `urls.frontend_url`.
- [ ] **Step 4: Run, expect PASS.**
- [ ] **Step 5: Commit** `feat(redirect): routing rules, click sources, OS, error page, domain redirects`.

### Task 5: Config additions

**Files:**
- Modify: `backend/internal/config/config.go`, `config.example.yaml`, `config.docker.yaml`, `config_test.go`

**Interfaces:**
- Produces: `cfg.URLs.FrontendURL` (default `http://localhost:3000`), `cfg.URLs.RedirectHost` (default = host of BaseURL), `cfg.URLs.RedirectIP` (default ""), `cfg.UI.FaviconService bool` (default true), `cfg.Uploads.Path` (default `data/uploads`).

- [ ] **Step 1: Test** `TestApplyDefaults_URLs` asserting the defaults above.
- [ ] **Step 2: Implement and update both yaml files with comments.**
- [ ] **Step 3: Run `go test ./internal/config/`, PASS. Commit** `feat(config): frontend_url, redirect_host/ip, ui and uploads settings`.

### Task 6: Tags, activity log, title fetcher, UTM helper

**Files:**
- Create: `repository/tag_repo.go`, `repository/activity_repo.go`, `service/tag_service.go`, `service/activity_service.go`, `service/title_fetcher.go`, `service/utm.go`, `handler/tag.go`, `handler/activity.go`
- Test: `repository/tag_repo_test.go`, `service/utm_test.go`, `service/title_fetcher_test.go` (httptest server), `handler/activity_test.go`

**Interfaces:**
- Produces: `TagRepository{SetLinkTags(ctx, linkID, tags []string) error; GetLinkTags(ctx, linkID)([]string,error); GetTagsForLinks(ctx, ids []uint64)(map[uint64][]string,error); ListByGroup(ctx, groupID)([]string,error); AddTagsToLinks(ctx, ids, tags) error; RemoveTagsFromLinks(ctx, ids, tags) error}`. Tags are trimmed, max 100 chars, deduplicated case-sensitively. `ActivityRepository{Insert(ctx,*ActivityEntry) error; List(ctx, groupID uint64, from, to *time.Time, limit, offset int)([]ActivityEntry,int64,error)}`; `ActivityService.Log(ctx, userID, groupID uint64, action, entityType string, entityID uint64, meta map[string]any)` (never returns error; logs on failure). `FetchTitle(ctx, rawURL string) string` — GET with 3 s timeout, 512 KB cap, parses `<title>`, falls back to hostname; blocks private IP ranges (returns hostname). `ApplyUTM(rawURL string, p UTMParams{Source,Medium,Campaign,Term,Content string; Custom []KV}) (string, error)` — appends/overwrites query params, preserves fragment. Routes: `GET /api/tags?type=link` → `{tags:[...]}`; `GET /api/activity-log?from&to&page&limit` → `{entries,total,page,total_pages}`.

- [ ] **Step 1: Tests** — `TestTagRepository_SetAndGet`, `TestTagRepository_ListByGroup_Distinct`, `TestApplyUTM_AppendsAndPreservesFragment`, `TestApplyUTM_OverwritesExisting`, `TestFetchTitle_ParsesTitle`, `TestFetchTitle_FallbackHostnameOnTimeout`, `TestFetchTitle_BlocksPrivateIP`, `TestActivityHandler_List`.
- [ ] **Step 2: FAIL → implement → PASS → commit** `feat: tags, activity log, title fetcher, UTM builder`.

### Task 7: Link service v2 (create with tags/UTM/expiration/routing/title, list filters, hide, bulk, back-half edit, delete rules, export)

**Files:**
- Modify: `repository/link_repo.go`, `repository/interfaces.go`, `service/link_service.go`, `service/interfaces.go`, `handler/link.go`, `cmd/server/main.go`
- Test: `repository/link_repo_test.go` (new), `service/link_service_test.go` (new, mocks), `handler/link_test.go` (new)

**Interfaces:**
- `LinkRepository` gains: `ListByGroup(ctx, groupID uint64, f LinkFilter, limit, offset int) ([]model.LinkWithStats, int64, error)` where `LinkFilter{Query string; Tags []string; Archived *bool; CustomBackHalf *bool; Expiration string /*expired|expiring|none|""*/; HasQR *bool; CampaignID *uint64; CreatedAfter, CreatedBefore *time.Time; Sort string /*created|clicks*/}` (total_clicks from `link_stats_daily` SUM + `clicks` fallback: use `(SELECT COUNT(*) FROM clicks c WHERE c.link_id=l.id)` for now); `SetArchived(ctx, ids []uint64, archived bool) error`; `IncrementRedirectCount(ctx, id) error`; `CountByGroup`.
- `LinkService`: `CreateLinkInput` gains `Tags []string; UTM *UTMParams; ExpiresAt; RoutingRules []RoutingRuleInput; GroupID uint64`; `Create` fetches title when empty (via `TitleFetcher` func field, injectable), sets `HasCustomBackHalf = input.CustomCode != ""`, `LongURLHost`. `List(ctx, groupID, params ListLinksParams{Filter LinkFilter; Page, Limit})`. `Update` with `OriginalURL` change → `IncrementRedirectCount` + cache invalidation. `SetArchived(ctx, userID, groupID, ids, archived)`. `BulkTags(ctx, groupID, ids, add, remove)`. `EditBackHalf(ctx, userID, linkID, domainID *uint64, backHalf string) (*model.Link, error)` creates the new link with `ParentLinkID`, copies title/tags/expiry/routing. `Delete` returns `ErrLinkNotDeletable` when `RedirectCount>0 || HasCustomBackHalf` → handler 409 `{"error":"This link can't be deleted. You can hide it instead."}`. `Export(ctx, groupID, filter) ([][]string, error)` columns `Link, Custom Link, Date created, Title, Destination URL, Engagements, Status`.
- Handler routes (all under auth+group): `GET /links` (query params per spec §6.2 incl. `archived=on|off`, default off), `POST /links`, `GET /links/:id` (includes `tags`, `total_clicks`, `routing_rules`, `history` = chain of parent links `[{id, short_url}]`), `PUT /links/:id`, `DELETE /links/:id`, `PATCH /links/bulk {action:"archive"|"unarchive"|"edit_tags", ids, add_tags, remove_tags}`, `POST /links/:id/back-half {domain_id, back_half}` → 201 new link, `PUT /links/:id/tags {tags}`, `GET /links/export?…` (text/csv attachment `links.csv`). Response link object adds `tags`, `archived`, `total_clicks`, `has_custom_back_half`, `redirect_count`, `favicon_url` (`https://www.google.com/s2/favicons?domain={host}&sz=32` when `ui.favicon_service`), `group_id`.

- [ ] **Step 1: Repository tests** — `TestLinkRepository_ListByGroup_FilterArchivedDefault`, `_FilterTags`, `_FilterCustomBackHalf`, `_FilterExpiration`, `_Query` (matches title/url/code/tag), `_SortClicks`, `TestLinkRepository_SetArchived`.
- [ ] **Step 2: Service tests (mocks)** — `TestLinkService_Create_FetchesTitle`, `_AppliesUTM`, `_SavesTagsAndRules`, `TestLinkService_Delete_NotDeletableAfterRedirect`, `_NotDeletableCustomBackHalf`, `TestLinkService_EditBackHalf_CreatesChild`, `TestLinkService_Update_DestinationIncrementsRedirect`.
- [ ] **Step 3: Handler tests** — `TestLinkHandler_Bulk_Archive`, `TestLinkHandler_Export_CSVHeader`, `TestLinkHandler_Delete_409`.
- [ ] **Step 4: Run, FAIL → implement → PASS.** Keep `ListByUserID` for compatibility but no longer used by handlers.
- [ ] **Step 5: Commit** `feat(links): filters, tags, hide, bulk, back-half edit, delete rules, export`.

### Task 8: Stats v2 (range, compare, hourly, breakdowns incl. OS/browser, sources)

**Files:**
- Modify: `repository/click_repo.go`, `repository/interfaces.go`, `service/stats_service.go`, `handler/stats.go`
- Test: `repository/click_repo_test.go`, `service/stats_service_test.go`

**Interfaces:**
- `ClickRepository` gains `CountRange(ctx, linkIDs []uint64, from, to time.Time, source string) (int64, error)`, `Series(ctx, linkIDs, from, to, unit string /*day|hour*/, source) ([]Bucket{Date string; Count int64}, error)` (zero-filled), `Breakdown(ctx, linkIDs, from, to, facet string /*country|city|referrer|device|browser|os*/, limit int, source) ([]Row{Key, Key2 string; Count int64}, error)`, `TotalAllTime(ctx, linkIDs, source)`.
- `StatsService.GetLinkStats(ctx, userID, linkID uint64, q StatsQuery{From, To time.Time; Unit string; Compare bool; Source string}) (*LinkStatsV2, error)` returning `{total_all_time, total_range, prev_total_range, change_pct *float64, series[], prev_series[], countries[{code,name,clicks,percentage,prev_clicks}], cities[], referrers[], devices[], browsers[], os[]}`. Route `GET /links/:id/stats?from=YYYY-MM-DD&to=YYYY-MM-DD&unit=day|hour&compare=previous|none&source=all|link|qr|page` (defaults: last 30 days, day, none, all). Previous period = same length ending the day before `from`.

- [ ] **Step 1: Tests** — `TestClickRepository_Series_ZeroFills`, `TestClickRepository_Breakdown_Device`, `TestStatsService_Compare_PreviousWindow` (mock repo; asserts prev window dates and change_pct), `TestStatsService_Defaults30Days`.
- [ ] **Step 2: FAIL → implement → PASS → commit** `feat(stats): date-range, comparison, hourly series and new breakdowns`.

### Task 9: Groups/search/profile/account/domains-verify handlers + wiring

**Files:**
- Create: `handler/group.go`, `handler/search.go`
- Modify: `handler/auth.go`, `handler/domain.go`, `cmd/server/main.go`
- Test: `handler/group_test.go`, `handler/search_test.go`, `handler/auth_test.go`

**Interfaces:**
- Routes: `GET/POST /api/groups`, `GET/PUT /api/groups/:id` (`{name, default_domain_id}`); `GET /api/search?q=` → `{links: n, qr_codes: 0}`; `PUT /api/auth/me` accepts `{display_name, account_name, default_group_id, home_checklist_dismissed}`; `PUT /api/auth/email {password, new_email}`; `GET /api/auth/me` returns `default_group_id, account_name, home_checklist_dismissed, created_at`; `GET /api/domains` returns verification fields; `POST /api/domains` generates `verification_token` and sets `pending`; `POST /api/domains/:id/verify` performs DNS TXT lookup of `_urlshortener.{domain}` expecting `verify={token}` OR CNAME to `urls.redirect_host` (net.Resolver, injectable) and sets `ok`/`error`; `PUT /api/domains/:id {root_redirect, not_found_redirect, group_id}`.
- Activity logging calls added in link/domain/group/token handlers for actions: `link.create, link.update, link.hide, link.unhide, link.delete, link.back_half, domain.create, domain.verify, domain.delete, group.create, group.update, token.create, token.revoke`.

- [ ] **Step 1: Tests** — `TestGroupHandler_Update_DefaultDomainMustBeVerified` (400), `TestSearchHandler_Counts`, `TestAuthHandler_UpdateEmail_WrongPassword401`, `TestDomainHandler_Verify_TXT` (fake resolver).
- [ ] **Step 2: FAIL → implement → PASS → `go test -p 1 ./...` all green → commit** `feat(api): groups, search, profile/account updates, domain verification`.

### Task 10: Frontend foundation (tokens, fonts, query provider, shadcn components, vitest)

**Files:**
- Modify: `frontend/src/app/globals.css`, `src/app/layout.tsx`, `package.json`
- Create: `src/lib/query.tsx`, `src/lib/format.ts`, `vitest.config.ts`, `src/test/setup.ts`, `src/lib/format.test.ts`

- [ ] **Step 1:** `npx shadcn@latest add switch select dropdown-menu popover calendar tabs badge tooltip sheet command separator skeleton progress radio-group textarea avatar alert-dialog scroll-area --yes`; `npm i date-fns`; `npm i -D vitest @vitejs/plugin-react @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom`.
- [ ] **Step 2:** `vitest.config.ts` (jsdom, setup file, alias `@`), `package.json` script `"test": "vitest run"`.
- [ ] **Step 3: Test** `format.test.ts`: `formatDate('2026-10-24T09:37:00Z') === 'Oct 24, 2026'`, `formatDateTime(...) === 'October 24, 2026 5:37 PM GMT+8'` style via `Intl` with tz option, `formatNumber(1240) === '1,240'`, `pct(1240, 3260) === '38%'`.
- [ ] **Step 4:** Implement `format.ts`; tokens in `globals.css`: `--primary: #2A5BD7; --primary-foreground: #fff; --background: #F4F6FA; --foreground: #031F39; --muted-foreground: #5C6B7A; --border: #E3E6EB; --card: #fff; --ring: #2A5BD7; --brand: #EE6123; --sidebar-primary: #0F2E8A; --radius: 0.5rem; --destructive: #D93025`; Inter via `next/font/google` replacing Geist; `QueryProvider` wrapping children in `layout.tsx` (staleTime 30s).
- [ ] **Step 5:** `npx vitest run && npm run lint && npx tsc --noEmit` PASS. Commit `feat(frontend): Bitly design tokens, Inter, TanStack Query, vitest`.

### Task 11: API client, types and hooks

**Files:**
- Modify: `src/lib/api.ts`, `src/types/index.ts`, `src/stores/auth.ts`
- Create: `src/lib/hooks/useLinks.ts`, `useLink.ts`, `useTags.ts`, `useGroups.ts`, `useDomains.ts`, `useLinkStats.ts`, `useTokens.ts`, `useActivity.ts`, `useSearch.ts`
- Test: `src/lib/api.test.ts`

**Interfaces:**
- `api` adds: `getLinks(params: LinkListParams)`, `createLink(CreateLinkInput)`, `updateLink`, `deleteLink`, `bulkLinks({action, ids, add_tags?, remove_tags?})`, `editBackHalf(id,{domain_id,back_half})`, `setLinkTags(id,tags)`, `exportLinks(params): Promise<Blob>`, `getLinkStats(id, {from,to,unit,compare,source})`, `getTags(type)`, `getGroups/createGroup/updateGroup`, `getDomains/createDomain/verifyDomain/updateDomain/deleteDomain`, `getTokens/createToken/revokeToken`, `getActivity`, `search(q)`, `updateMe`, `updateEmail`. Every request sends `X-Group-ID` from `useAuthStore.getState().activeGroupId` when set. Types mirror the backend JSON (`Link` adds `tags, archived, total_clicks, has_custom_back_half, redirect_count, favicon_url, group_id, routing_rules?, history?`).
- `useAuthStore` adds `activeGroupId: number|null`, `groups: Group[]`, `setActiveGroup(id)`, loads groups in `checkAuth` and defaults to `user.default_group_id`.
- Hooks are thin TanStack wrappers with query keys `['links', groupId, params]`, `['link', id]`, `['tags', groupId]`, etc., plus mutations that invalidate `['links']`.

- [ ] **Step 1: Test** `api.test.ts` with a mocked `fetch`: sends `Authorization` and `X-Group-ID`; `exportLinks` returns a Blob; error body → `Error(message)`.
- [ ] **Step 2: Implement → PASS → commit** `feat(frontend): api client v2, types, query hooks, active group`.

### Task 12: AppShell (sidebar, top bar, group switcher, global search, user menu, create-new menu)

**Files:**
- Create: `src/components/layout/AppShell.tsx`, `Sidebar.tsx` (rewrite), `TopBar.tsx`, `GroupSwitcher.tsx`, `GlobalSearch.tsx`, `UserMenu.tsx`, `CreateNewMenu.tsx`; delete `TopNav.tsx`
- Modify: `src/app/dashboard/layout.tsx`, `src/components/layout/index.ts`
- Test: `src/components/layout/Sidebar.test.tsx`, `GlobalSearch.test.tsx`

- [ ] **Step 1: Tests** — Sidebar renders items in order `Home, Links, QR Codes, Pages, Analytics, Campaigns, Custom domains, Settings`, marks `/dashboard/links/123` active for Links; collapse button toggles `aria-expanded`; GlobalSearch: typing "test" shows text `Search for "test" in:` with buttons `Links` and `QR Codes`, Enter navigates to `/dashboard/links?q=test` (mock `useRouter`).
- [ ] **Step 2: Implement** per spec §3: sidebar 240px/collapsed 64px (state in `localStorage` key `sidebar-collapsed`), off-canvas below `lg` with hamburger in top bar; "Create new" button opens `CreateNewMenu` (DropdownMenu: Link → `/dashboard/links/create`, QR Code → `/dashboard/qr-codes/create`, Page → `/dashboard/pages/new`); keyboard shortcuts `l` and `q` (ignored when typing in inputs). Top bar: `GroupSwitcher` (popover list of groups, "Search groups…", check mark on active), search icon → `GlobalSearch`, help icon → `/docs` link (external `_blank`), `UserMenu` (avatar initial; dropdown: name/email, account name row with copy, items Support `https://github.com/SeaCodeBase/urlshortener/issues`, API documentation `/api-docs` placeholder route, Terms (disabled), separator, Sign out).
- [ ] **Step 3: PASS; lint/tsc; commit** `feat(frontend): Bitly-style app shell`.

### Task 13: Shared pieces — PageHeader, EmptyState, ConfirmDialog, CopyButton, Favicon, StatCard, TagPicker, TagEditor, ShareDialog, DateRangeDialog, FiltersDialog

**Files:** create each under `src/components/common/` or `src/components/links/`; tests `TagPicker.test.tsx`, `DateRangeDialog.test.tsx`, `FiltersDialog.test.tsx`.

**Interfaces:**
- `TagPicker({value: string[]; onChange; suggestions: string[]; placeholder='Select tags'})` — Command-based combobox with chips, "Create '{x}'" option on Enter.
- `TagEditor({linkId, tags})` — inline: shows chips or "No tags" + "+ Add tag" button that opens TagPicker popover and saves via `setLinkTags`.
- `ShareDialog({shortUrl, title})` — short link + Copy; buttons X (`https://twitter.com/intent/tweet?url=`), Facebook, LinkedIn, WhatsApp, Email (`mailto:?body=`).
- `DateRangeDialog({open, value:{from,to}|null, onApply, onClear})` — title "Filter by created date" (prop `title`), two inputs, `Calendar` with `numberOfMonths={2}` `mode="range"`, preset chips `Last hour, Today, Last 7 days, Last 30 days, Last 60 days, Last 90 days`, buttons Cancel/Apply.
- `FiltersDialog({open, value: LinkFilters, tags, onApply})` — title "Filters", fields per spec §5.2, footer `Clear all filters | Cancel | Apply`.
- `ConfirmDialog({open, title, description, confirmLabel, destructive, onConfirm})` via AlertDialog.
- `EmptyState({title, description, action?})`, `StatCard({label, value, change?: {pct, label}, spark?: number[]})`, `CopyButton({text, label?})` with toast "Copied to clipboard", `Favicon({url, size})`.

- [ ] **Step 1: Tests** — TagPicker creates a new tag on Enter and calls onChange; DateRangeDialog preset "Last 7 days" sets from = today-6 and Apply calls onApply with both dates; FiltersDialog "Clear all filters" resets to defaults and Apply passes `{tags:[], linkType:'all', expiration:null, hasQr:'any'}`.
- [ ] **Step 2: Implement → PASS → commit** `feat(frontend): shared dialogs, tag picker, share dialog, stat card`.

### Task 14: Links list page

**Files:**
- Modify: `src/app/dashboard/links/page.tsx`, `src/components/links/LinksToolbar.tsx`, `LinkCard.tsx`
- Create: `LinkCardCompact.tsx`, `LinkActionsMenu.tsx`, `BulkTagDialog.tsx`, `LinksList.tsx`
- Test: `src/components/links/LinkCard.test.tsx`, `LinksToolbar.test.tsx`

- [ ] **Step 1: Tests** — LinkCard renders title, short url without protocol, destination, `"{n} engagements"`, "No tags" + "Add tag", hidden badge when archived; actions menu lists `View link details, Create QR Code, Add to a page, Add to campaign, Hide link, Delete` (and `Show link` when archived; Delete disabled with tooltip text when `redirect_count>0 || has_custom_back_half`). Toolbar: shows "0 selected", disables Hide/Tag with no selection, renders `Show: Active` select with `Hidden` option, view radios labelled `Compact view, Default view, Grid view`.
- [ ] **Step 2: Implement** per spec §5.2: URL state (`q`, `show=active|hidden`, `tags`, `type`, `exp`, `qr`, `from`, `to`) via `useSearchParams`; infinite scroll with `useInfiniteQuery` (page size 20) + sentinel "You've reached the end of your links"; select-all checkbox; bulk Hide → ConfirmDialog "Hide {n} links?"/"Hide links"; bulk Tag → BulkTagDialog; Export → download CSV blob; "Add to a page"/"Add to campaign" items navigate to `#` placeholders disabled until later phases (render disabled with tooltip "Coming soon" — allowed only within this phase; later phases wire them). View preference in `localStorage` `links-view`. Skeleton loaders. Empty states: no links → "No links yet" + "Create link"; no results → "No results found / Try adjusting your search, filters, or try searching hidden links instead." with link to `show=hidden`.
- [ ] **Step 3: PASS → lint/tsc → commit** `feat(frontend): Bitly-style links list with filters, bulk actions and views`.

### Task 15: Create link page and Home quick create

**Files:**
- Create: `src/components/links/LinkForm.tsx`, `ShortLinkField.tsx`, `UtmBuilder.tsx`, `ExpirationField.tsx`, `RoutingRulesEditor.tsx`, `LinkReadyDialog.tsx`, `src/components/home/QuickCreate.tsx`, `src/components/home/GettingStarted.tsx`, `src/components/home/TodayStats.tsx`
- Modify: `src/app/dashboard/links/create/page.tsx`, `src/app/dashboard/page.tsx`
- Test: `UtmBuilder.test.tsx`, `ShortLinkField.test.tsx`, `QuickCreate.test.tsx`

**Interfaces:**
- `ShortLinkField({domains, domainId, onDomainChange, backHalf, onBackHalfChange, error})` with "Generate" producing 7 chars `[a-zA-Z0-9]`.
- `UtmBuilder({value: UTM, onChange, destination})` renders Source/Medium/Campaign/Term/Content + custom params + preview computed by `buildUtmPreview(destination, utm)` (pure, exported, tested).
- `LinkForm({mode:'create', defaultDomains})` sections/labels exactly per spec §5.3; submit → `createLink`; on success copy to clipboard, toast "Link created and copied", `router.push('/dashboard/links/'+id)`.
- `QuickCreate` per §5.1 (Short link mode only in this phase; QR Code toggle present but switches to a "Create a QR Code" link to `/dashboard/qr-codes/create` until Phase 2); success opens `LinkReadyDialog` ("Your link is ready!", short link box, `View link details`, `Copy link`).
- `GettingStarted` reads links count & total clicks from `useLinks` and `/api/analytics/overview` is not yet available → use `getLinks({limit:1})` total and sum of `total_clicks` on first page; items `Shorten a URL`, `Collect 10 clicks or scans` (progress), `Create a QR Code`, `Create a page`; dismiss "×" → `updateMe({home_checklist_dismissed:true})`.
- `TodayStats` shows "ENGAGEMENTS {n}" using `getLinkStats` is per-link only; for this phase call `GET /links?limit=100&sort=clicks` is wrong — instead backend Task 8 exposes nothing group-level yet, so render the card with data from a new tiny endpoint added here: `GET /api/analytics/today?range=today|7d|30d` → `{engagements}` (add to backend `handler/stats.go` + `ClickRepository.CountRangeByGroup`). Include that backend change in this task with test `TestStatsHandler_Today`.

- [ ] **Step 1: Tests** — `buildUtmPreview('https://a.com/p?x=1#f', {source:'s',medium:'m'}) === 'https://a.com/p?x=1&utm_source=s&utm_medium=m#f'`; ShortLinkField Generate yields 7 alphanumerics and rejects `"bad code!"` with message "Only letters, numbers, - and _ are allowed"; QuickCreate submits with Enter and shows "Your link is ready!".
- [ ] **Step 2: Implement → PASS → commit** `feat(frontend): create link form, UTM/expiration/routing editors, home quick create`.

### Task 16: Link details and edit pages

**Files:**
- Create: `src/app/dashboard/links/[id]/page.tsx`, `src/components/links/LinkDetailsHeader.tsx`, `LinkAnalytics.tsx`, `src/components/charts/TimeSeriesChart.tsx`, `BreakdownTable.tsx`, `DeviceBarChart.tsx`
- Modify: `src/app/dashboard/links/[id]/edit/page.tsx`; delete `links/[id]/stats/page.tsx` (add redirect from `/stats` to the details page)
- Test: `BreakdownTable.test.tsx`, `LinkAnalytics.test.tsx`

- [ ] **Step 1: Tests** — BreakdownTable paginates 10/page and shows `%` as `38%`; LinkAnalytics renders date button "Last 30 days", comparison select default "No comparison", and modules titled `Engagements over time, Locations, Referrers, Devices, Browsers, Operating systems`; when `compare=previous` the change badge shows `▲ +21.2%`.
- [ ] **Step 2: Implement** per spec §5.4 and §5.5 (edit page: banner, "Edit back-half", "Redirect", Title, Tags, Cancel/Save; back-half change calls `editBackHalf` and navigates to the new link). Details ⋯ menu = `LinkActionsMenu`. History list under Details when `history.length>0`. Hidden banner with "Show link".
- [ ] **Step 3: PASS → commit** `feat(frontend): link details with analytics and edit page`.

### Task 17: Settings pages

**Files:**
- Create: `src/app/dashboard/settings/layout.tsx`, `profile/page.tsx`, `account/page.tsx`, `groups/page.tsx`, `groups/[id]/page.tsx`, `api/page.tsx`, `activity-log/page.tsx`; `src/components/settings/SettingsNav.tsx`, `PasskeysSection.tsx` (moved from existing settings page), `TokenCreateDialog.tsx`
- Modify: `src/app/dashboard/settings/page.tsx` → redirect to `/dashboard/settings/profile`
- Test: `SettingsNav.test.tsx`, `TokenCreateDialog.test.tsx`

- [ ] **Step 1: Tests** — nav renders groups `Personal settings: Profile`, `Account settings: Account details, Groups, Webhooks, Bulk upload, Activity log`, `Developer settings: API` (Webhooks/Bulk upload link to routes that show an EmptyState "Coming in a later phase" page until Phase 5); TokenCreateDialog shows the raw token once with Copy and the text "Make sure to copy your token now. You won't be able to see it again."
- [ ] **Step 2: Implement** per spec §5.11 (Profile: display name, email + "Change email" dialog, change password, Passkeys section; Account: account name, member since, notification email read-only; Groups: list + "Add new group" + detail with Default domain select of verified domains; API: tokens table + create/revoke; Activity log: table with date filter and pagination).
- [ ] **Step 3: PASS → commit** `feat(frontend): settings pages (profile, account, groups, api tokens, activity log)`.

### Task 18: Auth pages restyle + Custom domains page (list/add/verify/settings)

**Files:**
- Modify: `src/app/(auth)/layout.tsx`, `login/page.tsx`, `register/page.tsx`, `src/app/dashboard/domains/page.tsx` → move to `src/app/dashboard/custom-domains/page.tsx` (+ redirect from old path), delete `domains/create/page.tsx`
- Create: `src/components/domains/AddDomainDialog.tsx`, `DomainRow.tsx`
- Test: `AddDomainDialog.test.tsx`

- [ ] **Step 1: Test** — stepper shows DNS instructions containing `CNAME` → `{redirect_host}` and `TXT _urlshortener.{domain}` with the token after step 1; Verify button calls `verifyDomain`.
- [ ] **Step 2: Implement** per spec §5.10 and §5.13 (auth two-column layout; headings "Log in and start sharing" / "Create your account"; passkey panel preserved). Config exposure: add `GET /api/meta` → `{base_url, redirect_host, redirect_ip, favicon_service}` (backend, with handler test `TestMetaHandler`).
- [ ] **Step 3: PASS → commit** `feat(frontend): auth pages restyle and custom domains page`.

### Task 19: Docker/compose/config docs + end-to-end smoke

**Files:**
- Modify: `docker-compose.yml`, `backend/config.docker.yaml`, `backend/config.example.yaml`, `README.md`, `frontend/Dockerfile` (unchanged unless needed), `.env.example`

- [ ] **Step 1:** Add `urls.frontend_url`, `urls.redirect_host` to docker config; README section "Bitly-style UI" with screenshots placeholder list and the migration note (`002_bitly_clone.sql` runs automatically on fresh installs; for existing installs run it manually).
- [ ] **Step 2: Smoke** — run backend (`cd backend && go run ./cmd/server`) and frontend (`npm run dev`); in a browser: register → Home quick create → Links list → details → edit title → hide → Show: Hidden → Show link → create token → use token with curl `GET /api/links` → activity log shows entries. Record results in the task notes.
- [ ] **Step 3: Full checks** — `go test -p 1 ./...`, `go vet ./...`, `npm run lint`, `npx tsc --noEmit`, `npx vitest run`, `npm run build`. Commit `chore: phase 1 config, docs and smoke`.

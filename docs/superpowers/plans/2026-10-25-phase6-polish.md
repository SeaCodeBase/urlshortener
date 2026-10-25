# Phase 6: Polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close spec §8 phase 6: keyboard shortcuts (L, Q), an empty-state audit with the last placeholder removed, graceful shutdown and proxy trust on the backend, deployment files brought up to date, a batch of parked UI minors from earlier phases, and the final README.

**Architecture:** No new features. Backend: `main.go` gains signal handling with `http.Server.Shutdown` for both routers and ordered worker stops; `server.trusted_proxies` config feeds `gin.Engine.SetTrustedProxies`. Frontend: one `useGlobalShortcuts` hook mounted in `AppShell`; a shared `StatusChip`; small fixes in existing components. Deployment: Dockerfile/compose/config.docker.yaml aligned with the Phase 2–5 config keys.

**Tech Stack:** unchanged (Go 1.25/Gin; Next.js 16/React 19/Vitest).

**Spec:** `docs/superpowers/specs/2026-10-24-bitly-clone-design.md` §3 (compose shape), §5.2 (views — already built in Phase 1: Compact/Default/Grid toggle in `LinksToolbar`), §8 item 6, §9 (per-phase e2e smoke). Already satisfied before this phase and only verified here: responsive sidebar (`AppShell` Sheet below `lg`), redirect-server error page (`handler/errorpage.go`), compact/grid views.

## Global Constraints

- Repo root `/Users/jose/GolandProjects/urlshortener`, branch `feat/bitly-clone`. Commit after every task; messages end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>` (an implementing subagent uses its own model name). Never `git add -A`; add files by path. `backend/vendor/` stays untracked.
- Go: `cd backend && go vet ./... && TEST_DATABASE_DSN="urlshortener:urlshortener_pass@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC" go test -p 1 ./...` green. Frontend: `cd frontend && npx vitest run && npm run lint && npx tsc --noEmit && npm run build` green. No new dependencies.
- No billing/upgrade/AI/integrations UI. Existing copy and visual tokens unchanged.
- Shortcuts must never fire while the user is typing (input, textarea, select, contenteditable), while any dialog/sheet/menu is open, or with a modifier key held.

## Review Focus

1. A shortcut firing inside a text field would corrupt user input — Task 2 (`useGlobalShortcuts.test.tsx`: typing "l" in an input does not navigate; with a dialog open does not navigate).
2. A shutdown that drops in-flight requests or leaves a bulk job half-written — Task 1 (`TestServer_GracefulShutdownOrder` with fake workers asserting stop order and that the HTTP servers stop accepting before workers stop).
3. Trusting every proxy lets a client spoof its IP into click geo/ip_hash — Task 1 (`TestTrustedProxies_*`: with no trusted proxies, `X-Forwarded-For` is ignored; with `127.0.0.1` trusted, it is honoured only from that peer).
4. Docker image must start with the Phase 2–5 config: `uploads.path` writable, `workers` keys parsed, port 8081 exposed — Task 4 (`docker compose config` validation step and a `go run` of the binary against `config.docker.yaml` syntax via `config.LoadFromYAML` test).
5. Removing the last placeholder must not leave a route 404 — Task 3 (test that `ComingSoon` no longer exists and every sidebar/settings nav href resolves to a page file).

---

### Task 1: Backend — graceful shutdown, trusted proxies, deployment config

**Files:**
- Modify: `backend/cmd/server/main.go`, `backend/internal/config/config.go` (+ `config_test.go`), `backend/internal/worker/click_flusher.go` (only if `Stop` is not already synchronous), `backend/Dockerfile` (`EXPOSE 8080 8081`), `backend/config.docker.yaml` (+ `workers:` block, `server.trusted_proxies: []`), `backend/config.example.yaml` (`server.trusted_proxies`), `docker-compose.yml` (no functional change expected; verify healthchecks and the uploads volume; add `stop_grace_period: 30s` to backend).
- Create: `backend/cmd/server/shutdown.go` + `shutdown_test.go` (a small `runServers(ctx, redirectSrv, apiSrv *http.Server, stops []func())` helper so the order is unit-testable without sockets).

**Interfaces:**
- Produces: `config.ServerConfig.TrustedProxies []string` (`yaml:"trusted_proxies"`; empty → `engine.SetTrustedProxies(nil)` which makes Gin ignore `X-Forwarded-For`); `shutdownTimeout = 20 * time.Second`.

- [ ] **Step 1: Failing tests.** `TestConfig_TrustedProxiesDefaultEmpty`; `TestTrustedProxies_IgnoredWhenNoneConfigured` (gin test engine with `SetTrustedProxies(nil)`, request from 10.0.0.9 with `X-Forwarded-For: 1.2.3.4` → `c.ClientIP()` is 10.0.0.9); `TestTrustedProxies_HonouredFromTrustedPeer`; `TestRunServers_ShutdownOrder` (fake servers via `httptest.NewUnstartedServer`? simpler: pass `shutdown func(ctx) error` closures; cancel ctx → assert order: API shutdown, redirect shutdown, then stops in the given order; total under timeout).
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** `main.go`: `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`; build `&http.Server{Addr, Handler: router}` for both; start with `ListenAndServe` in goroutines (log + `os.Exit(1)` on non-`ErrServerClosed`); on `<-ctx.Done()`: `Shutdown` both with `shutdownTimeout`, then stop workers in order: click flusher (final flush, which also enqueues), webhook dispatcher (waits for in-flight), bulk worker (stops between batches). Apply `SetTrustedProxies(cfg.Server.TrustedProxies)` to both engines (nil when empty). Dockerfile `EXPOSE 8080 8081`. `config.docker.yaml`: `trusted_proxies: []` with a comment ("list your reverse proxy IPs/CIDRs; empty = X-Forwarded-For ignored") and the `workers:` block copied from the example. compose: `stop_grace_period: 30s` on backend.
- [ ] **Step 4: Verify** suite green; `docker compose config -q` exits 0; `go build ./...`; restart the dev backend and confirm `kill -TERM <pid>` logs "shutting down" and exits within 20 s.
- [ ] **Step 5: Commit** `feat(server): graceful shutdown, trusted proxies, docker config`.

### Task 2: Keyboard shortcuts L and Q

**Files:**
- Create: `frontend/src/lib/hooks/useGlobalShortcuts.ts` + `useGlobalShortcuts.test.tsx`
- Modify: `frontend/src/components/layout/AppShell.tsx` (mount the hook), `frontend/src/components/layout/CreateNewMenu.tsx` (Page row gets no shortcut; keep L/Q hints), `README.md` (one "Keyboard shortcuts" line under Local development or Features).

**Interfaces:** `useGlobalShortcuts(): void` — `keydown` on `window`: key `l`/`L` → `router.push('/dashboard/links/create')`, `q`/`Q` → `/dashboard/qr-codes/create`; ignored when `event.metaKey || ctrlKey || altKey`, when `event.defaultPrevented`, when the target or an ancestor is `input, textarea, select, [contenteditable=""], [contenteditable="true"]`, when `document.querySelector('[role="dialog"], [role="menu"], [role="listbox"], [data-state="open"][role="alertdialog"]')` finds an open overlay, or when `event.repeat`.

- [ ] Tests first (navigate on `l`/`q`; no navigation while typing in an input/textarea/contenteditable; no navigation with a `[role=dialog]` present; no navigation with ctrl/meta held) → implement → `vitest`, lint, tsc → commit `feat(frontend): keyboard shortcuts L and Q`.

### Task 3: Empty-state audit, placeholder removal, parked UI minors

**Files:**
- Delete: `frontend/src/components/common/ComingSoon.tsx` (confirm no imports remain).
- Create: `frontend/src/components/common/StatusChip.tsx` (+ test) — `StatusChip({tone: 'success'|'warning'|'info'|'muted'|'destructive', children})` with the class sets currently repeated in `domains/DomainRow.tsx`, `settings/WebhookRow.tsx`, `settings/BulkJobRow.tsx`, `settings/DeliveriesPanel.tsx`; replace those usages.
- Modify: `frontend/src/components/links/FiltersDialog.tsx` (campaign Select renders a fallback item "Campaign #{id}" when the current value is not in the list); `frontend/src/components/domains/DomainRow.tsx` (hide "Set as default for group" when `domain.group_id != null && domain.group_id !== activeGroupId`); `frontend/src/components/campaigns/ChannelCard.tsx` (link count from `channel.links.length` while stats load; header label "{n} links · {e} engagements in range"); `frontend/src/components/analytics/*` only if the audit finds a list without an empty state; `frontend/src/components/layout/Sidebar.test.tsx` or a new `routes.test.ts` asserting every nav href in `Sidebar.tsx` and `SettingsNav.tsx` has a matching `src/app/dashboard/**/page.tsx`.
- Audit list (each must show the spec's centred title + grey subtitle + action when empty): Links, QR Codes, Pages, Analytics dashboards, Campaigns, Custom domains, Webhooks, Bulk upload, Activity log, API tokens, Groups. Fix any that render a bare table/“No results”.

- [ ] Tests first for StatusChip, Filters fallback, DomainRow hide rule, ChannelCard fallback, route-coverage test → implement → checks → commit `refactor(frontend): shared status chip, empty-state audit, parked UI fixes`.

### Task 4: Docs and deployment verification

**Files:** Modify `README.md` (top: one-paragraph feature overview listing all areas with links to the phase sections; "Keyboard shortcuts"; "Deployment" notes: `trusted_proxies`, `stop_grace_period`, `workers`, ports 8080/8081/3000; "Upgrading" unchanged), `docs/superpowers/specs/2026-10-24-bitly-clone-design.md` §8 (append a one-line status note per phase: done + ledger path).

- [ ] Write; run `docker compose config -q`; commit `docs: phase 6 polish, deployment notes`.

### Task 5 (controller, not a subagent): end-to-end smoke per spec §9

Register a throwaway user → create link → list (Compact/Grid toggle) → details → edit → hide → QR → page → dashboard → campaign → shortcut `L` → mobile viewport sidebar sheet → redirect error page for an expired link. Record the result in the ledger.

---

## Self-review notes

- Spec coverage: §8.6 compact/grid (verified existing), shortcuts (T2), empty states (T3), responsive sidebar (verified), error page (verified), docs/README (T4), compose + Dockerfiles (T1). Parked security item (proxy trust) folded into T1; graceful shutdown into T1 (needed by the Phase 5 bulk worker semantics).
- Parallelism: T1 (backend + deploy files) ∥ T2 ∥ T3 (frontend, disjoint files: T2 touches hooks/AppShell/CreateNewMenu; T3 touches common/links/domains/campaigns/settings). T4 after T1–T3 (README mentions all).

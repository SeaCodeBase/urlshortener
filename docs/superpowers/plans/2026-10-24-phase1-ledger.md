# SDD ledger — plan: docs/superpowers/plans/2026-10-24-phase1-foundation-links.md
Spec: docs/superpowers/specs/2026-10-24-bitly-clone-design.md (read).
Branch: feat/bitly-clone (in-place checkout; Ruling: no worktree — user is away and nothing else edits this checkout — cost if wrong: concurrent edits collide).

## Pre-flight conflict scan
| Pair / task | Produces vs consumes | Finding |
|---|---|---|
| T1 ↔ T2 | T1 model.Group, users.default_group_id; T2 GroupRepository uses them | consistent |
| T2 ↔ T3 | T2 GroupService.ResolveActive; T3 GroupMiddleware consumes | consistent |
| T3 ↔ T9 | T3 middleware wired to route groups; T9 adds /groups routes without group header | consistent (groups route excluded) |
| T4 ↔ T7 | T4 RedirectService.InvalidateLink; T7 Update uses invalidation | consistent |
| T4 ↔ T8 | T4 adds clicks.source/os; T8 breakdown facets include os, source filter | consistent |
| T6 ↔ T7 | T6 TagRepository, FetchTitle, ApplyUTM; T7 Create consumes | consistent |
| T7 ↔ T11/T14/T16 | link JSON fields tags/total_clicks/favicon_url/history; frontend types mirror | consistent |
| T8 ↔ T15/T16 | stats v2 shape; T15 adds /analytics/today (backend change inside a frontend task) | Ruling: allowed, small endpoint; cost if wrong: minor split |
| T9 ↔ T18 | T18 adds GET /api/meta (backend) | consistent |
| T10 ↔ T12.. | tokens, QueryProvider | consistent |
| T13 ↔ T14/T15/T16 | shared components props | consistent |
| T1 self | migration + runner + models + testutil | agrees |
| T7 self | ListByGroup excludes source<>link per §11.3 | agrees |
Review rubric conflicts: none found.
Models: implementers opus (multi-file integration), reviewers opus (per user), final review opus.
Task 1: dispatched (BASE ba7db64, implementer opus)
Task 1: complete (commits ba7db64..4049d37, review approved)
Task 1: Ruling: fk_links_group ON DELETE CASCADE accepted — deleting a group deletes its links; group delete is not exposed in UI this phase — cost if wrong: data loss on group delete.
Task 1: carried into Task 2 — LinkRepository.Create must write group_id; GroupService.ResolveActive lazily creates a default group when users.default_group_id IS NULL; add authoring-rules comment to SplitStatements.
Task 1: minor (deferred): no cross-process migration lock; testutil swallows DELETE errors; VerificationToken serialized in JSON.
Task 2: dispatched (BASE 4049d37, implementer opus)
Task 2: review needs fixes (race in lazy default group; Update ignores domain ownership). Fix round 1 dispatched to original implementer (FIX_BASE a13e2d5).
Task 2: fix round 1/5 (3 addressed, 0 open; commits a13e2d5..bdd7f25)
Task 2: complete (commits 4049d37..bdd7f25, review clean after 1 fix round)
Task 2: Ruling: domain scoped to another group may still be set as default domain — deferred to Task 9 (domain group_id semantics) — cost if wrong: group uses a domain assigned elsewhere.
Task 2: carried: LinkService.Create must set GroupID (Task 7). NewGroupService(groupRepo, userRepo, domainRepo) signature.
Task 3: dispatched (BASE bdd7f25, implementer opus)
Task 3: complete (commits bdd7f25..3100580, review approved)
Task 3: minor (deferred): user-not-found during token create → 500 not 401; DB failure in Validate → 401; duplicate ErrAPITokenNotFound per package.
Task 4: dispatched (BASE 3100580, implementer opus)
Task 4: complete (commits 3100580..2bf374d, review approved)
Task 4: Ruling: unknown code on a custom domain with not_found_redirect → 302 there (matches Bitly) — cost if wrong: 404 page not shown on branded domains.
Task 4: minor (deferred): stripPort duplicated / IPv6; ExpiredRedirect naming; JSON 500 vs HTML; domain not_found_redirect edits stale ≤1h; non-base Host cache keys; Delete must call InvalidateLink (→Task 7); GET /m/:slug deferred to Phase 4.
Task 5+6: dispatched together (BASE 2bf374d, implementer opus) — Ruling: batched, small config task plus helpers; reviewed as one unit.
Task 5+6: complete (commits 2bf374d..3f367b8, review approved)
Task 5+6: minor (deferred): activity page overflow (cap page); duplicated clamping; INSERT IGNORE hides FK errors; placeholder limit in bulk tag insert; extra re-read in activity Insert; a few unblocked IP ranges; compose mounts data ro (uploads path).
Task 7: dispatched (BASE 3f367b8, implementer opus) — carried: set GroupID on create; call InvalidateLink on delete/update; ownership checks before bulk tag repo calls; context.WithoutCancel for activity Log; exclude source<>link.
Task 7: review approved-with-fixes (CSV injection Important; plus old-domain cache key, verified-domain check, clear title/expiration, migration 003 test). Fix round 1 dispatched (FIX_BASE 0d9e9dc).
Task 7: fix round 1/5 (5 addressed, 0 open; commits 0d9e9dc..822d526)
Task 7: complete (commits 3f367b8..822d526, review clean after 1 fix round)
Task 7: Ruling: migration 003 (short_code VARCHAR(255) utf8mb4_bin) accepted — needed for case-sensitive 255-char back-halves; table-copy ALTER on prod needs a release note — cost if wrong: brief lock on links table during upgrade; old mixed-case links become case-sensitive.
Task 7: minor (deferred): single-link ops user-scoped vs list group-scoped (teammate links); loadDomainMap by user; Export repeats COUNT per page and buffers rows; maxBulkIDs const placement; whitespace-only title clears title; frontend must send clear_expiration (Task 16).
Task 8: dispatched (BASE 822d526, implementer opus) — includes /api/analytics/today endpoint pulled forward from Task 15.
Task 8: complete (commits 822d526..ed0297e, review approved)
Task 8: minor (deferred): pin DSN session time_zone to UTC (db.go) → fold into Task 9; link_stats_daily unused (no source col); prev window top-50 cap; sequential queries.
Task 9: dispatched (BASE ed0297e, implementer opus) — includes GET /api/meta (from Task 18) and DSN time_zone fix.
Task 10: dispatched (BASE 53135e0, implementer opus) — Ruling: frontend-only task started while Task 9 is under read-only review; file sets are disjoint — cost if wrong: rebase conflicts (none expected).
Task 9: review needs fixes (Critical: reserved hosts claimable as custom domain → open redirect; unverified domains honoured by redirect service). Fix round 1 dispatched (FIX_BASE 53135e0).
Task 10: complete (commits 53135e0..7e984af, review approved)
Task 10: minor (deferred): formatDate test tz-dependent (set TZ=UTC in vitest setup → Task 11); inter.variable on <html>; secondary bg-white; has-[>svg]:px-3; mixed radix packages.
Task 11: dispatched (BASE 7e984af, implementer opus) — frontend only; Task 9 backend fix still in flight.
Task 9: fix round 1 implemented (commit b3075c8); re-review dispatched.
Task 9: fix round 1/5 (3 addressed, 1 residual Important: reserved rows backfilled ok; commits 53135e0..b3075c8). Fix round 2 dispatched (FIX_BASE b3075c8).
Task 9: fix round 2/5 (1 addressed, 1 new open: reserved-host cache key not canonical; commits b3075c8..8052c53). Fix round 3 dispatched (FIX_BASE 8052c53).
Task 11: implemented (0722f3a); note: backend lacks PUT /links/:id/routing-rules (needed by details page) → add in Task 16 dispatch as backend addition.
Task 9: fix round 3/5 (1 addressed, 0 open; commits 8052c53..eaef21c)
Task 9: complete (commits ed0297e..eaef21c, review clean after 3 fix rounds)
Task 9: minor (deferred): negative domain-cache race (shorter TTL); delete+re-add host within 1h reuses old link keys; InvalidateCache not normalised; stripPort IPv6; UpdateAccount non-transactional; apex TXT-only note for UI; domain Update whole-row overwrite.
Backend tasks 1–9 complete.
Task 11: complete (commits 8052c53..0722f3a, review approved)
Task 11: minor → carried into Task 12: UpdateLinkInput.title doc (null keeps), wrap network failures as ApiError(…,0), 401 → logout, queryClient.clear() on logout, comment on setGroupId coupling.
Task 12: dispatched (BASE eaef21c, implementer opus)
Task 7b (backend addendum): dispatched (BASE eaef21c, implementer opus) — GET/PUT /api/links/:id/routing-rules; runs in parallel with frontend Task 12 (disjoint files).
Task 7b: complete (commits eaef21c..af15c36, review approved)
Task 7b: minor (deferred): second GetByID for invalidation; PUT without key clears rules (use pointer/required); unbounded echoed value in 400; deadlock under concurrent PUT.
Note: subagent commits carry "Co-Authored-By: Claude Opus 5.5" (the implementing model) — Ruling: accepted as accurate attribution.
Task 12: review needs fixes (401→logout too broad; shortcuts in dialogs; hard-coded white). Fix round 1 dispatched (FIX_BASE 4c82bd8). Visual check in browser: shell renders per spec (sidebar/topbar/group switcher).
Task 12: fix round 1/5 (3 addressed, 0 open; commits 4c82bd8..544172b)
Task 12: complete (commits af15c36..544172b, review clean after 1 fix round)
Task 12: minor (deferred): search panel stays open on outside click; collapsed Create new tooltip; double redirect on sign out.
Task 13: dispatched (BASE 544172b, implementer opus)
Task 13: complete (commits 544172b..389d120, review approved)
Task 13: minor (deferred): expiration radio order (Any first); Tags label association; silent tag truncation; case-sensitive dedupe in list; future dates typed; Email target _blank; no loading indicator in ConfirmDialog.
Task 14: dispatched (BASE 389d120, implementer opus)
Task 14: complete (commits 389d120..2cca92f, review approved)
Task 14: minor (deferred): error state wipes loaded list; object URL revoked early; search input not synced to external q / Clear all; disabled-item tooltips hover-only; compact actions fade with menu open; fetchNextPage may cancel refetch; export clears selection.
Task 15: dispatched (BASE 2cca92f, implementer opus) — /api/analytics/today already exists (Task 8); use it for TodayStats.
Task 15: complete (commits 2cca92f..15e2b9b, review approved)
Task 15: minor (deferred): UTM preview re-serialises query; empty custom value dropped; routing long_url not normalised client-side; expiration switch on w/o date; tooltip mouse-only; Cancel anchor type attr; QuickCreate duplicates ShortLinkField; default domain label; Enter in hex input submits.
Task 16: dispatched (BASE 15e2b9b, implementer opus) — uses GET/PUT /links/:id/routing-rules (Task 7b) and PUT /links/:id with title/clear_expiration.
Task 16: review needs fixes (CSV injection in client export; error state; routing invalidation). Fix round 1 dispatched (FIX_BASE 94a0bcc). Visual check: details page matches Bitly layout.
Task 16: fix round 1/5 (3 addressed, 0 open; commits 94a0bcc..7008ea0)
Task 16: complete (commits 15e2b9b..7008ea0, review clean after 1 fix round)
Task 16: minor (deferred): keepPreviousData hides refetch errors; "Compare to a link" omitted; per-module CSV icon omitted; local date vs UTC day; pagination not aria-live.
Task 17: dispatched (BASE 7008ea0, implementer opus)
Task 17: complete (commits 7008ea0..6130837, review approved)
Task 17: minor (deferred): group detail error state; domain 400 message generic; close-animation blank; heading "Passkeys (2-Factor authentication)"; redundant checkAuth; docs link target.
Task 18: dispatched (BASE 6130837, implementer opus) — /api/meta exists; verification fields live.
Task 18: complete (commits 6130837..bcef1aa, review approved)
Task 18: minor (deferred): CNAME at apex fallback; dead Back button; sign-up link flash while meta loads; stale instructions snapshot.
Task 19: dispatched (BASE bcef1aa, implementer opus) — docs/compose/config only; controller runs the browser smoke.
Smoke (controller, browser vs live backend): register via API → login UI → Home quick create layout → Links list → details with 5 engagements + series chart → edit page → settings/api → custom-domains empty hero → Hide link → Show: Hidden shows it with badge → login page two-column. All OK. Note: register API response has zero created_at (minor, for final review).
Task 19: complete (commit 235b954; controller ran full checks: go vet/test green, vitest/lint/tsc/build green)
Final review (opus): Not mergeable — C1 proxy drops X-Group-ID (+CORS PATCH/X-Group-ID); I1 domain delete re-homes links (FK SET NULL) + app-only default-domain uniqueness; I2 group default domain unused + domain.group_id not enforced; I3 pending domain claims squat forever. Ledger must-fix: delete host link keys on domain delete; Verify writes only verification columns.
Final review: Ruling on I3 — allow takeover of a non-ok domain row owned by another user (delete stale row) — cost if wrong: a legitimate pending claim could be taken by someone who verifies first (they must control DNS anyway).
Final fix wave dispatched (FIX_BASE 235b954, implementer opus).
Final fix wave: commits b4b6acf (backend) + b8a0f3d (frontend); re-review dispatched. Controller verified: migration 004 applied+recorded on restart; group scoping works via API and via Next proxy; UI group switch shows only that group's links.
Final fix: Ruling: migration 004 replaces links.domain_id FK with ON DELETE RESTRICT (MariaDB refuses generated column with SET NULL) — cost if wrong: user-row delete fails while domains have links (no delete path exists).
Final fix wave re-review: all findings addressed. Parked — Ruling: `PUT /links/:id` cannot move a link with a custom back-half back to the default domain (domain_id 0 → 404), so a custom domain holding such links cannot be deleted — real but not load-bearing for Phase 1; carried into Phase 2 Task 1 together with a README ops note for migration 004 on DBs with pre-existing duplicate default-domain codes — cost if wrong: a user is stuck with a domain they cannot remove until Phase 2 lands.
Phase 1 complete: commits 554f630..b8a0f3d (32 commits). Checks green at b8a0f3d (controller ran go vet/test, vitest/lint/tsc/build before the fix wave; fix-wave agent re-ran all).

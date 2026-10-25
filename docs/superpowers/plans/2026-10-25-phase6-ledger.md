# SDD ledger — plan: docs/superpowers/plans/2026-10-25-phase6-polish.md
Spec: docs/superpowers/specs/2026-10-24-bitly-clone-design.md §8.6, §9.
Pre-flight scan:
| Pair / task | Produces vs consumes | Finding |
| T1↔T2/T3 | backend+deploy vs frontend | disjoint files; parallel |
| T2↔T3 | T2: hooks/useGlobalShortcuts, layout/AppShell, layout/CreateNewMenu; T3: common/StatusChip, links/FiltersDialog, domains/DomainRow, campaigns/ChannelCard, settings/*Row, Sidebar/SettingsNav tests | disjoint |
| T4 | README after T1–T3 | sequential |
| T1 self | shutdown helper testable without sockets | consistent |
Scan clean. Ruling: T1, T2, T3 dispatched in parallel.
BASE (T1,T2,T3): fa3951fbd28c52832ccb3159f16dc0c1bcd5657a
Phase 5 merged into local main (fa3951f); push to origin still fails (1Password SSH agent; origin at 1c38114). Phase 6 T1/T2/T3 dispatched in parallel.
Task 2: DONE, commit c87e4cc (52 tests pass; removed the pre-existing duplicate useCreateShortcuts in CreateNewMenu/Sidebar — ruling: accepted). Review dispatched.
Task 2: complete (review ✅/Approved).
Task 1: DONE, commit 3ea7ee4 (suite green; live SIGTERM clean). Notes: compose mariadb has no named data volume (pre-existing; prod uses its own volume per spec §11.1) — report to user; trusted_proxies default empty changes click IP behind a proxy until configured (intended, documented in Task 4). Review dispatched.
Task 3: DONE, commit 63717b8 (vitest 456/456, lint, tsc). Review dispatched. Task 4 (docs) dispatched (BASE 63717b8).
Task 3: complete (review ✅/Approved; minors: campaign fallback item may flash while loading; route test cwd-based).
Task 4: DONE, commit 2278cf7 (README overview/shortcuts/deployment; spec §8 status). Review dispatched.
Task 1 review: spec ✅, quality approved with Important: page-view IPs behind the Next proxy collapse to the frontend IP once proxies are untrusted. Ruling: `server.api_trusted_proxies` applied only to the API engine (default loopback; docker example 172.16.0.0/12 with the spoof caveat documented). Minors folded in: validate proxy lists before workers start; wait for click-recorder goroutines on shutdown; stop_grace_period 45s. Fix round 1 dispatched (resume implementer).
Task 5 smoke (controller): Q → /dashboard/qr-codes/create ✓; L → /dashboard/links/create ✓ (needs hydration; 2 s wait flaked, 3 s ok); mobile 375px: "Open navigation" opens the sheet with the full nav ✓; inactive link → 410 "This link is no longer available" page ✓; unknown code → 404 "Link not found" ✓; expiring link rejected under 5 min (validation) — expired page path covered by Phase 1 tests; all dashboard pages 200.
Task 4: complete (review: accurate; minors: shortcut-ignore list simplified in README (intended), bulk stop wording could add 'reclaim after crash' clause, overview omits Groups/Registration; phase6 ledger path will exist at close). Task 1 fix round owns the README proxy sentence update.
Task 1 fix round 1: commit 3236baa (api_trusted_proxies default loopback; docker 172.16/12; validation before workers; click-recorders stop; grace 45s). Controller e2e: two page-view beacons via the Next proxy with distinct XFF → page_views distinct ip_hash 1→3 ✓. Scoped re-review + final whole-branch review dispatched.
Task 1: complete (fix re-review: all addressed; minors: stale '30s' comment on shutdownTimeout; theoretical WaitGroup Add/Wait window only after a timed-out drain — parked).
Final review (fa3951f..3236baa): 2 Important (I1 no strict-parse guard for committed configs; I2 .env.example JWT_SECRET not read → placeholder secret in Docker) + 8 minors. Rulings in final-findings.md (strict YAML, JWT_SECRET/BASE_URL env overrides, placeholder warning). ONE fix wave dispatched (BASE 3236baa).
Final fix wave: 289e30e (I1) e40a80f (I2) 36e6f33 (M1-3,M8) 645114c (M5) 26073d8 (M4,M6) f6a52cd (M7); all gates green incl. docker builds. Note for user: compose now passes JWT_SECRET/BASE_URL from .env → next docker up re-keys sessions and ip-hash salt. Scoped re-review dispatched.
Fix-wave re-review (3236baa..f6a52cd): I1, I2, M1–M8 ADDRESSED; no new Critical/Important. Gates at f6a52cd: Go all ok (+race cmd/worker/handler); vitest 460/460; lint/tsc/build clean; compose config ok; both images build.
Phase 6: complete. Final review clean. All six phases done.

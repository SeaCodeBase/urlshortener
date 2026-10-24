# SDD ledger — plan: docs/superpowers/plans/2026-10-25-phase3-analytics.md
Spec: docs/superpowers/specs/2026-10-24-bitly-clone-design.md (§5.4, §5.8, §6.2, §6.4). Branch feat/bitly-clone, base fdaf50e. Controller: Fable 5.1 (main session, loop mode); implementers/reviewers: opus.

## Pre-flight conflict scan
| Pair / task | Produces vs consumes | Finding |
|---|---|---|
| T1 → T4 | DashboardRepository (EnsureDefault, CreatorNames) vs DashboardService.List | consistent |
| T2 → T3 | ClickScope + CountScope/SeriesScope/BreakdownScope/ItemsScope vs AnalyticsService | consistent; T3 also needs `ShortURL` (service.ShortURL exists at link_service.go:994) |
| T3 → T4 | DashboardFilter type defined in T3, validated in T4 | consistent |
| T3/T4 → T5 | services vs handlers; parseStatsQuery reuse | same package, fine |
| T5 ↔ T6 | both edit handler/stats.go? No: T5 edits analytics.go/dashboard.go/main.go; T6 edits stats.go | disjoint |
| T6 → T11 | compare_link JSON | consistent |
| T7 → T8/T9/T10/T11 | types/hooks/charts | consistent; T8 creates DashboardNameDialog reused by T9 |
| T1 self | migration FK to `groups` (name confirmed in 002) | ok |
| T4 self | ValidationError: none exists in service pkg → implementer adds `ValidationError{Msg}` | Ruling below |
| T10 self | custom range URL-only, presets persisted | matches contracts |
Ruling: no `ValidationError` type exists in `service`; T4 adds `type ValidationError struct{ Msg string }` with `Error()` and handlers map it to 400 — plan said "reuse if exists" — cost if wrong: one small type.
Ruling: dashboards FK references `groups` (backticked) — cost if wrong: none.
Task 1: dispatched (BASE fdaf50e, implementer opus, agent addb3cdb242f3096e)
Task 1: implemented 30cfcdc (CreatorNames display_name→email fallback; EnsureDefault tolerates 1213 deadlock)
Task 1: complete (commits fdaf50e..30cfcdc, review approved)
Task 1: minor (deferred): name-sort test non-discriminating (Alpha/Zeta order); EnsureDefault deadlock loser may return error (retry INSERT once); EnsureDefault relies on REPEATABLE READ (doc it); Update doesn't default empty DateRange; pre-existing gofmt noise in passkey_service_test.go.
Task 2: dispatched (BASE 30cfcdc, implementer opus)
Task 2: implemented bb8149f (legacy methods refactored onto scopeWhere)
Task 2: complete (commits 30cfcdc..bb8149f, review approved)
Task 2: minor (deferred): zero-value ClickScope silently means group 0 (consider rejecting); SeriesScope hour unit untested directly; BreakdownScope log lacks scope fields.
Task 3: dispatched (BASE bb8149f, implementer opus)
Task 7: dispatched (BASE bb8149f, implementer opus) — parallel with Task 3; frontend-only files
Task 7: implemented fca1190 (dnd-kit pinned; compare_link required in LinkStatsV2 type; EXTRA_COLOR #47B5A1)
Task 3: implemented cc3f2c0 (NewAnalyticsService(clickRepo, linkRepo, qrRepo, domainRepo, baseURL=cfg.URLs.BaseURL); export split into analytics_export.go; unknown filter type → plain error, Task 4 validates)
Task 7: complete (commits bb8149f..fca1190, review approved)
Task 7: minor (deferred): redundant dashboard refetch after update (invalidate only data subkey); DATE_RANGE_OPTIONS labels plain string; BarBreakdownChart/TimeSeriesChart bar+extraSeries/mutations untested; BarBreakdownChart YAxis width 120 clips long labels; DashboardDataQuery.unit sent but ignored; rangeFor local-day vs backend UTC (pre-existing convention).
Task 8: dispatched (BASE cc3f2c0, implementer opus) — frontend-only, parallel with Task 3 review
Task 3: review needs fixes (Important: breakdown/comparison logic duplicated from stats_service.go). Fix round 1 dispatched (FIX_BASE cc3f2c0) incl. minors: silent domain lookup error (Warn), test helper comment.
Task 3: Ruling: in the `all` view every click on a QR-owned link (source='qr' link) is the QR item, while the `qr` filter scope counts only source='qr' clicks — accepted as-is; QR-owned links receive non-qr clicks only when the URL is typed without ?r=qr — cost if wrong: tiny count mismatch between all-view and qr-filter for such clicks.
Task 3: note for Task 10: for ranges ≤2 days top_day.date is an hour bucket (YYYY-MM-DDTHH:00); the scorecard must format it.
Task 3: minor (deferred): dateFmt const used in one of two places; Overview fetches 1000 rows for 5 (comment/lower limit); test gaps (QR-owned link without QR row dropped; Count==0 row; top.csv with nulls).
Task 3: fix round 1/5 (3 addressed per implementer; commit 097bf9a) — re-review dispatched
Task 8: implemented f568ef0 (added TimeSeriesChart prevSeriesName; top performing uses generic icon since AnalyticsItem lacks original_url)
Task 3: fix round 1/5 (3 addressed, 0 open; commits cc3f2c0..097bf9a)
Task 3: complete (commits fca1190..097bf9a, review clean after 1 fix round)
Task 3: minor (deferred): shared breakdown helpers live in stats_service.go (could be their own file).
Task 4: dispatched (BASE f568ef0, implementer opus)
Task 8: complete (commits 097bf9a..f568ef0, review approved)
Task 8: minor (deferred): no error states on overview (failed request shows false empty state / skeleton forever); weak link query in PerformanceOverview.test; duplicated short URL when title empty; dialog rejection path untested; single loading tile.
Task 8: Ruling: AnalyticsItem gains `original_url` (backend, done in Task 5 alongside handler wiring; frontend Favicon use in Task 10 + final wave for TopPerforming) — spec §5.8 shows favicons on top performing rows — cost if wrong: one extra JSON field.
Task 9: dispatched (BASE f568ef0, implementer opus) — frontend-only, parallel with Task 4
Task 9: implemented ffbce66 (singular '1 link'/'1 QR Code' in filter summary — accepted)
Task 4: implemented 4956a84 (NewDashboardService(DashboardServiceDeps{Dashboards, Links, QRs, Activity}); filter all+ids → ValidationError; *ValidationError pointer receiver)
Task 6: dispatched (BASE 4956a84, implementer opus) — backend stats.go/stats_service.go only
Task 10: dispatched (BASE 4956a84, implementer opus) — frontend dashboard page; parallel with Task 6
Task 9: complete (commits f568ef0..ffbce66, review approved)
Task 9: minor (deferred): DisabledItem helper duplicated from LinkActionsMenu; disabled Delete tooltip unreachable by keyboard; no keepPreviousData on sort change; no page-level tests; narrow disabled-Delete test; "Created by " when name empty.
Task 4: complete (commits ffbce66..4956a84, review approved)
Task 4: minor (deferred): service duplicates repo constants (date range default, sort names); Update re-read after write may error post-success (+extra GetByID/CreatorNames); empty patch still writes+logs; filterFromMap accepts stored links/qr filter with no ids; Duplicate copies raw stored JSON; test gaps (List repo errors, valid qr filter, Update persisting layout); interface comments unqualified ErrDashboardNotFound.
Task 5: waiting on Task 6 (both touch cmd/server/main.go)
Task 6: implemented 3467de6 (NewStatsService(clickRepo, linkRepo, domainRepo, baseURL); compare_link always present)
Task 5: dispatched (BASE 3467de6, implementer opus) — includes AnalyticsItem.original_url ruling from Task 8
Task 6: complete (commits 4956a84..3467de6, review approved)
Task 6: minor (deferred): missing compare link logged at Error (Warn better); compare_link validated before owner check (400 before 404); compareLinkStats repeats CountRange+Series; service doesn't reject self-compare; no handler test for foreign compare link 404.
Task 11: dispatched (BASE 3467de6, implementer opus) — frontend LinkAnalytics/CompareLinkDialog; parallel with Task 5 (backend) and Task 10 (other frontend files)
Task 11: implemented 2bc260c (chip shows 'Compared link' until data loads)
Task 10: implemented eb173eb (ItemsTable split out; ChangeCell exported from BreakdownTable; original_url optional on AnalyticsItem)
Task 10: Ruling: dashboard defaults to "Previous period" comparison (?compare=none turns it off) — matches Bitly's dashboard default; spec lists "Previous period" first — cost if wrong: one default flip.
Task 5: implemented 22fffc8 (smoke via curl OK: create/data/export 7 files/overview/PATCH {} 400/DELETE default 409; /data without from/to uses 30-day default — frontend must send from/to; dev backend restarted detached)
Task 11: complete (commits 3467de6..2bc260c, review approved)
Task 11: minor (deferred): dialog load errors show as "No links found."; ?compare_link=<own id> → 400 error card (guard client-side); CSV "Compared link" section lacks the link name (and =Spring fixture asserts nothing); stray compare_link kept in URL on QR page; chart extraSeries wiring untested; trailing blank line csv.test.ts.
Task 5: complete (commits eb173eb..22fffc8, review approved)
Task 5: minor (deferred): Duplicate has no 404 test; {"name":null} → "nothing to update" undocumented; /data,/export without from/to ignore stored date_range (frontend always sends); unit validated though service picks it; Today test passes nil analytics service; smoke data left in dev DB + detached go run backend.
Task 12: dispatched (BASE 22fffc8, implementer opus) — README docs only
Task 12: complete (commit 472faeb) — Ruling: docs-only diff reviewed by the final whole-branch review instead of a task review (same as Phase 2) — cost if wrong: a README inaccuracy slips until final review. README claim "Default dashboard can be renamed" is correct (only Delete is guarded).
Task 10: complete (commits 2bc260c..eb173eb, review approved)
Task 10: minor (deferred): ModuleGrid untested (reorder/revert); overlapping layout saves may desync until refetch; every layout/name update refetches module data; preset/rename UI lags until server reply; ?from&to >366 days → dead-end error; failed background refetch hidden; cap/"Direct" helpers duplicated between ModuleCard and dashboardCsv.
All tasks 1–12 complete. Controller full checks + smoke next, then final review.
Full checks at 472faeb (controller): go vet clean, go test -p 1 ./... all ok; vitest 182/182, lint, tsc, build green (3 analytics routes in build output).
Smoke (controller, browser at 472faeb, dev backend restarted at 22fffc8+ on 8081): /dashboard/analytics → Dashboards tiles (+ Create, Default "Modified on Oct 25, 2026"), Performance overview (3 engagements, This week/Last week chart), Top performing (Smoke QR, 3 engagements) → /dashboard/analytics/dashboards list row (avatar, Modified/Created by, Last 7 days, filter summary, ⋯) → Default dashboard: 7 modules with real data (scorecards, line, doughnut, referrer bar, locations table, items table), Filter dialog → QR Codes → select 1 → Apply → "Showing data for 1 QR Code", PATCH 200, data refetched (3→2) → Download CSV: GET /export 200 → link details: Compare to a link dialog (excludes current link) → Compare → ?compare_link=10, stats request carries compare_link, chip "example.org ×" (Remove comparison), third scorecard, chart legend "example.org". Smoke PASSED.
Smoke finding (minor, for final fix wave): "Top location" scorecard shows boxed "X X" for the Local/unknown country — flagEmoji renders regional indicators for non-ISO codes (e.g. "XX"); should return '' for non-flag codes.
Final whole-branch review dispatched (opus, package fdaf50e..472faeb).
Final review (opus): With fixes — I1 local-day presets vs UTC (+ backend date_range fallback); I2 unbounded bar/doughnut rows; I3 ghost filter ids after deletion. Must-fix minors: TopPerforming favicon/original_url, overview error states, flagEmoji guard. Findings file: final-findings.md. Final fix wave dispatched (FIX_BASE 472faeb, implementer opus).
Final fix wave: commits e406bcb (backend) + 1f8eda4 (frontend); scoped re-review dispatched.
Final fix wave re-review: all 6 findings addressed, no new Critical/Important breakage. Residual minors (deferred): ?unit=hour without from/to validated against the 30-day default (400) before the stored window applies; DoughnutChart keys by label (folded "Other" could collide with a real "Other" row when >8 rows); LoadError stacks vertically in the overview tile; DashboardFilter.ids TS type required while API may omit it (readers defensive); Duplicate copies ghost ids (pruned on read); extra GetByIDs on QR-filtered /data.
Controller verified at 1f8eda4: go vet/test all ok (count=1); vitest 198/198, lint, tsc, build green.
Phase 3 complete: commits fdaf50e..1f8eda4 (15 commits incl. plan). Smoke passed at 472faeb; fix-wave behaviour covered by tests.

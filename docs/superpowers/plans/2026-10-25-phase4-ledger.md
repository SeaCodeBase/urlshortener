# SDD ledger — plan: docs/superpowers/plans/2026-10-25-phase4-pages.md
Spec: docs/superpowers/specs/2026-10-24-bitly-clone-design.md (§5.7, §6.1–6.3, §5.1–5.4 touchpoints). Branch feat/bitly-clone. Controller: Fable 5.1 (main session); implementers/reviewers opus.

## Pre-flight conflict scan
| Pair / task | Produces vs consumes | Finding |
|---|---|---|
| T1 → T3/T4/T5 | PageRepository (+ errors) vs services/presenter | consistent |
| T2 → T4 | PageViewRepository vs Stats | consistent; T2 must share click_repo helpers (halfOpen/seriesUnits/breakdown columns) — refactor allowed |
| T3 → T4 | both in `service`; T4 needs no T3 types except errors | disjoint files |
| T3 ↔ LinkService | AddBlock new_link uses LinkService.Create — signature checked by implementer | ok |
| T4 ↔ handler/redirect.go hashIP | T4 may move hashIP to util; T5 edits redirect.go — sequential (T4 before T5) | ok |
| T5 ↔ T6 | JSON contracts in shared.md | consistent |
| T6 → T7–T11 | hooks/presets/PagePreview | consistent; PagePreview must be server-safe (no client hooks) for T10 |
| T8 ↔ T9 ↔ T11 | different component files; T11 touches links/* and home/* only | disjoint |
| T1 self | reserved short code `m` — list location unknown; implementer finds it | ok |
| T3 self | `has_unpublished_changes` needs MaxBlockUpdatedAt (T1 provides) | ok |
Ruling: page-created links are ordinary `source='link'` links (visible in Links); `page` is only a click source via ?r=page — Bitly behaviour; spec §5.7 wording says "source page" — cost if wrong: links list shows page buttons' links.
Ruling: deleting a link cascades its page blocks (FK ON DELETE CASCADE); published snapshot keeps the dead button until republish — cost if wrong: a 404 button on a public page until the owner republishes.
Ruling: views are not de-duplicated per visitor — cost if wrong: inflated view counts on refresh.
Task 1: dispatched (BASE 42e3e13, implementer opus)
Task 6: dispatched (BASE 42e3e13, implementer opus) — frontend-only, parallel with Task 1
Task 6: implemented 8beac2d (useDebouncedValue extracted; Link.page_ids added; note: view beacon must forward visitor IP/UA/Referer through the proxy — carry to Task 5/10)
Task 1: implemented 7bf33ae (ErrInvalidBlockOrder added — Task 3 maps to 400; reservedShortCodes{m} case-sensitive; has_unpublished_changes relies on DB session UTC — DSN sets time_zone='+00:00')
Task 6: complete (commits 42e3e13..8beac2d, review approved)
Task 6: minor (deferred): minimal preset avatar initial invisible (outline border = text colour); lucide brand icons deprecated; hooks import designPresets (component module) for slug helpers; editor-mode href="#" scrolls (editor must preventDefault); no render-time URL scheme guard on public hrefs; partial preset tests; brittle svg count test.
Task 7: dispatched (BASE 7bf33ae, implementer opus) — frontend-only, parallel with Task 1 review
Task 1: complete (commits 8beac2d..7bf33ae, review approved)
Task 1: minor (deferred): CreateBlock position comment overclaims (no unique (page_id,position)); ListByGroup views subquery aggregates all page_views (use correlated count); reserved `m` error message misleading (handler/link.go, Task 5); pageFixture pass-through; Create re-reads partial row vs CreateBlock full reload; migration relies on DB default case-insensitive collation (service lowercases).
Task 2: dispatched (BASE 7bf33ae, implementer opus) — parallel with Task 7 (frontend)
Task 2: implemented e68815d (timeseries.go shared helpers; page-view facets exclude os)
Task 2: complete (commits 7bf33ae..e68815d, review approved)
Task 2: minor (deferred): InsertDefaultsViewedAt test may flake across UTC midnight; LastInsertId error unlogged; facet allowlist separate from breakdownColumns (comment).
Task 7: implementer died on a network error (ENOTFOUND); resumed.
Task 3: dispatched (BASE e68815d, implementer opus)
Task 7: implemented 772777b ('1 view' singular; uses page.url; extra copy 'Choose a template'/'Create a page' — accepted)
Task 7: complete (commits e68815d..772777b, review approved)
Task 7: minor (deferred): no keepPreviousData on search; stale debounce closure for setParams; "Create your first page" hero when only hidden pages exist; useBaseHost/viewsLabel duplicated; spaces dropped silently in slug input, min length not enforced client-side; bg-amber-500 hard-coded; template tile preview scaling/clipping; stale check-slug cache after 409.
Task 8: dispatched (BASE 772777b, implementer opus) — frontend-only, parallel with Task 3
Task 3: implemented 2529aec (also changed page_repo.go: Publish/SetArchived don't bump updated_at, DeleteBlock bumps page updated_at; Publish sets published_at = max(now, edit times); templates scratch|classic|bold|minimal only)
Task 3: Ruling: GetPublished tests belong to Task 4 (Review Focus 1 mislabelled Task 3) — cost if wrong: none.
Task 8: implemented 7b0d108 (PhonePreview.tsx, blockFormShared.tsx added; PageActionsMenu showViewPage; PageStatusChip exported; preview reflects toggles/reorder after refetch)
Task 3: complete (commits 772777b..2529aec, review approved)
Task 3: minor (deferred): domainName duplicated from analytics_service (extract shared helper); published_at from app clock vs updated_at DB clock; Create not atomic on seed failure; orphan link if CreateBlock fails after new_link; email platform error text; page_service.go 1043 lines (split validation); DeleteBlock/seed-failure test gaps; List N+1 MaxBlockUpdatedAt.
Task 4: dispatched (BASE 7b0d108, implementer opus)
Task 8: complete (commits 2529aec..7b0d108, review approved)
Task 8: minor (deferred): preview Dialog overflows phone viewports (fixed 375px frame, no max-w cap); reorder untested; social email stricter than backend (mailto only) + platform switch mangles address; body counter UTF-16 vs code points; title flicker after rename; h1 aria-label reads "Rename page …"; sheet empties during close animation; hidden picker selection after search; LinkPicker duplicates CompareLinkDialog; sortable wiring mirrors ModuleGrid; `as BlockInput` cast; AddBlockMenu imports constants from component modules; drag handle labels identical.
Task 9: dispatched (BASE 7b0d108, implementer opus) — frontend-only, parallel with Task 4
Task 4: implemented 3799616 (NewPagePublicService(pageRepo, viewRepo, clickRepo, linkRepo, domainRepo, baseURL, ipSalt); util.HashIP extracted; stats query validated before group check)
Task 4: complete (commits 7b0d108..3799616, review approved)
Task 4: minor (deferred): GetPublished copy is shallow; `linker` PageServiceImpl has nil LinkSvc/Activity (comment it); blockStats N+1 CountRange; query validated before group check (400 vs 404, matches link stats).
Task 5: dispatched (BASE 3799616, implementer opus) — parallel with Task 9 (frontend)
Task 9: implemented 5f3792d (designOverride lifted into editor page; clears when server matches)
Task 10: dispatched (BASE 5f3792d, implementer opus) — src/app/m/[slug], PublicPageView, ViewBeacon; parallel with Task 5 (backend) and Task 9 review
Task 9: review needs fixes (Important: unmount-flush save failure handled by dead per-mutate onError → silent failure + stuck designOverride). Fix round 1 dispatched (FIX_BASE 5f3792d).
Task 9: minor (deferred): utcToday/Module helpers inside TrackTab (share with LinkAnalytics; views disagree near midnight); preset tile stays pressed after customisation; override can stick on out-of-order writes; revert uses stale page.design; hex error on every keystroke; bio/avatar untrimmed in field; weak loading test; duplicated breakdown mapping.
Task 10: implemented a81a47c (proxy already forwards allowlisted headers; XFF passed as-is; beacon Referer = page itself)
Task 10: Ruling: `POST /public/pages/:slug/view` accepts optional body `{"referrer"}` (visitor's document.referrer) preferred over the Referer header; frontend beacon sends it — the beacon's own Referer is the page URL, so referrer stats would be useless otherwise — cost if wrong: a spoofable referrer field (same trust level as the header).
Task 9: fix round 1 commit f04d830 (mutateAsync().catch survives unmount); re-review dispatched
Task 9: fix round 1/5 (1 addressed, 0 open; commit f04d830)
Task 9: complete (commits 3799616..5f3792d + f04d830, review clean after 1 fix round)
Task 9: minor (deferred): stale flush failure from an old DesignTab instance can clear a newer instance's override (per-instance saveSeq).
Task 10: complete (commits 5f3792d..a81a47c, review approved); follow-up dispatched to the same implementer: beacon sends {"referrer": document.referrer} body (ruling) — reviewed by the final review.
Task 10: minor (deferred): client XFF forwarded verbatim + gin trusts all proxies (spoofable ip_hash/geo/logs); backend outage renders as 404; mixed-case slug duplicates without canonical; beacon guard ignores slug change; unused baseHref prop.
Task 11: dispatched (BASE f04d830, implementer opus) — links/home files; parallel with Task 5 (backend) and Task 10 follow-up
Task 5: implemented 284406b (body referrer; HEAD /m/:slug added; gin accepts /m/:slug beside /:code; smoke via curl passed incl. ?r=page click source)
Task 10 follow-up: commit d3d8306 (beacon sends {referrer}; api.recordPageView(slug, referrer?); baseHref removed) — covered by the final review
Task 5: complete (commits f04d830..284406b, review approved)
Task 5: minor (deferred): parse-id/bind helpers duplicated across handlers; public view endpoint unauthenticated with no rate limit + ClientIP trusts all proxies (known, same as clicks); Stats still validates `source` though ignored; List total redundant (unpaginated).
Task 12: dispatched (BASE d3d8306, implementer sonnet) — README only
Task 12: complete (commit bff7f53) — Ruling: docs-only diff reviewed by the final whole-branch review (same as earlier phases) — cost if wrong: README inaccuracy until final review.
Task 11: implemented 4a3bf7e (content sends full LinkBlockContent; dialog lives inside LinkActionsMenu; usePages({archived:'off'}) key differs from usePages())
Full checks at 4a3bf7e (controller): go vet clean, go test -count=1 -p 1 ./... all ok; vitest 301/301, lint, tsc, build green (ƒ /m/[slug]).
Task 11: complete (commits bff7f53..4a3bf7e, review approved)
Task 11: minor (deferred): close-while-saving guard uses a separate mutation instance (never fires); every link card mounts page mutations (dialog always mounted); usePages() vs usePages({archived:'off'}) duplicate requests; LinkForm shows empty pages state while loading; archived pages ignored in PagesCard/checklist (undocumented); no test opens the dialog from the ⋯ menu.
All tasks 1–12 complete. Browser smoke in progress at 4a3bf7e.
Smoke (controller, browser at 4a3bf7e, dev backend restarted at 284406b+): Pages list empty hero → /pages/new claim "smoke-page-4" (check-slug 200 "Available") → Classic template → editor (Welcome text block, phone preview, Draft chip) → + Add → Link → New destination + "My website" → POST blocks 201, preview shows button → Design tab → Sunset preset (PUT 200, gradient preview live) → Publish changes (200, chip Published) → /m/smoke-page-4 server-rendered with gradient + button; view beacon POST 204 → curl localhost:8080/m/smoke-page-4 → 302 to frontend; public JSON snapshot has ?r=page short_url → click short link ?r=page → 302 and clicks row source=page after flush → Track tab: Total views 1, referrer/device/location rows, Link clicks table → Pages list card (Published, views, Edit/Share/⋯) → Links list shows the page-created link (ruling) → link details Pages card lists "smoke-page-4" + "Add to a page" button. Smoke PASSED.
Final whole-branch review dispatched (opus, package 42e3e13..4a3bf7e).
Final review (opus): With fixes — I1 publish misses same-second/in-flight edits (second-precision timestamps + no wait for pending saves); I2 link delete cascade doesn't flag the page changed. Must-fix minors: Minimal avatar initial invisible; README inaccuracies. Findings: final-findings.md. Final fix wave dispatched (FIX_BASE 4a3bf7e, implementer opus).
Final review: Ruling: adopt a revision counter (migration 008: pages.revision / published_revision) for draft-vs-published state instead of timestamps — fixes I1+I2 with one mechanism — cost if wrong: one extra migration and two columns.
Final fix wave: commits b5151fb (backend: migration 008 revision/published_revision, LEAST(?, revision) at publish, link-delete bump, README) + aeeff46 (frontend: publish waits, key-order-insensitive design compare, minimal avatar); scoped re-review dispatched.
Final re-review (aeeff46): I1, I2, M1, M2 addressed. New Important: revision bump in UpdateBlock/DeleteBlock/LinkRepository.Delete uses `UPDATE pages … WHERE id = (SELECT page_id FROM page_blocks …)`, which S-locks the block before X-locking the page — opposite order to ReorderBlocks (page then blocks FOR UPDATE); reviewer reproduced ERROR 1213 on MariaDB.
Ruling: run one more fix round (round 2 of final) — the defect was introduced by the fix wave, is mechanical (select page_id without lock, bump by id, then write block), and shipping 500s on concurrent edits is not acceptable. Cost if wrong: one more short review cycle.
Round 2 fix: df25c84 (lock order page→block in UpdateBlock/DeleteBlock/link Delete); scoped re-review dispatched.
Round 2 re-review (df25c84): deadlock finding ADDRESSED, lock order page→block in all three paths, no new Critical/Important. Residual (parked): link Delete reads page ids without a lock, so a block added for that link on another page in the same instant misses one revision bump — Ruling: accept; fixing it reinstates the lock inversion, worst case is a missing "unpublished changes" badge until the next edit.
Suites at df25c84: Go all ok (reviewer run); vitest/lint/tsc/build exit 0.
Phase 4: complete. Final review clean.

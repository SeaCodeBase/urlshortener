# SDD ledger — plan: docs/superpowers/plans/2026-10-25-phase2-qr-codes.md
Spec: docs/superpowers/specs/2026-10-24-bitly-clone-design.md (read).
Branch: feat/bitly-clone, in-place checkout (Ruling: no worktree — same as Phase 1, user away, nothing else edits this checkout — cost if wrong: concurrent edits collide).
Models: implementers opus, reviewers opus, final review opus (owner preference).

## Pre-flight conflict scan
| Pair / task | Produces vs consumes | Finding |
|---|---|---|
| T1 ↔ T2 | T1 qr_codes/qr_tags tables, model.QRCode; T2 QRRepository scans them | consistent |
| T2 ↔ T3 | T2 QRRepository, QRCodeRow.ToLink(); T3 QRService consumes | consistent |
| T2 ↔ T3 (QRLookup) | T2 QRCodeIDsForLinks; T3 LinkServiceDeps.QRs interface with that method | consistent |
| T3 ↔ T4 | T3 QRService (Update returns prevLink, Delete returns linkDeleted); T4 handler invalidates | consistent |
| T4 ↔ T5 | T4 NewQRHandler(qrSvc, invalidator, domainRepo, cfg); T5 adds stats param | consistent (constructor grows in T5, stated) |
| T4 ↔ T6 | qrResponse JSON fields; frontend QrCode type mirrors (title, qr_url, image_url, link_source, link) | consistent |
| T5 ↔ T6 | /uploads/{name} static + POST /api/uploads; Next proxy for both | consistent |
| T6 ↔ T7/8/9/10 | QrPreview, renderQrBlob/downloadQr, hooks, normalizeQrDesign | consistent |
| T7 ↔ T8 | QrDownloadMenu created in T7, used by QrReadyDialog in T8 | consistent (order) |
| T7 ↔ T9 | QrActionsMenu/TagEditor qr variant reused by details | consistent |
| T9 ↔ T10 | LinkAnalytics subject prop; T10 touches links pages only | consistent |
| T1 self | migration + model + domain-0 update + README | agrees |
| T3 self | Delete ignores deletability for source=qr links (ruling in plan) | agrees |
| T6 self | FormData upload must skip JSON Content-Type in api.headers() | agrees |
Review rubric conflicts: none found.
Task 1: dispatched (BASE 3287ef0, implementer opus)
Task 1: complete (commits 3287ef0..1c12fec, review approved). Controller verified qr_codes/qr_tags exist in both DBs.
Task 1: minor (deferred): duplicate-QR migration test asserts only err != nil; no-op Update still writes/logs.
Task 2: dispatched (BASE 1c12fec, implementer opus)
Task 2: implemented 9cd073f; concern: QRCodeRow.ToLink() lacks is_active/parent_link_id/timestamps — Ruling: Task 3 adds those columns to the row select so ToLink() is complete — cost if wrong: QR JSON link object shows zero timestamps/is_active.
Task 2: complete (commits 1c12fec..9cd073f, review approved)
Task 2: minor (deferred): ToLink() UserID from QR not link; GetTagsFor vs ListTagsByGroup sort order differ (mirrors TagRepository); no search 500 test for QR count; Update doesn't refresh UpdatedAt; count+page not atomic.
Task 3: dispatched (BASE 9cd073f, implementer opus) — carried: add l.user_id, l.is_active, l.parent_link_id, l.created_at, l.updated_at to QRCodeRow so ToLink() is complete.
Task 3: implemented 9cd073f..287644e; concerns: QRServiceDeps gained Domains+BaseURL (Task 4 must pass); unused Tags dep; qr.update + link.update both logged (Ruling: accepted, both entries are factual — cost if wrong: noisy activity log); Update not transactional (same as codebase).
Task 3: review needs fixes (Important: Delete removes QR row before link — orphan on failure; use FK cascade). Fix round 1 dispatched (FIX_BASE 287644e). Minor 2 (double link.update logging) — Ruling stands: accepted.
Task 3: fix round 1/5 (3 addressed, 0 open; commits 287644e..c54fd61)
Task 3: complete (commits 9cd073f..c54fd61, review clean after 1 fix round)
Task 3: minor (deferred): ToLink().GroupID from QR row; qr.update logged with empty fields; Create with link_id ignores UTM/expiry/rules silently; coverage gaps (rollback, BulkTags foreign ids, 8KB boundary); totalPages duplication; qr_service.go ~600 lines.
Task 3: note: QRServiceDeps = {QRs, Links, LinkRepo, Domains, Activity, BaseURL} (Tags dropped); Update returns prev even on error; Delete returns linkDeleted.
Task 4: dispatched (BASE c54fd61, implementer opus)
Task 6: dispatched (BASE c54fd61, implementer opus) — Ruling: frontend-only task run in parallel with backend Task 4; file sets disjoint — cost if wrong: commit/index races (each agent adds only its own files).
Task 4: implemented 386fff7; concerns: nested link in QR JSON has tags:[]/total_clicks:0 (Ruling: accepted — UI reads qr.tags/qr.total_scans — cost if wrong: link card inside QR pages shows 0 clicks); link_id 404 message 'QR code not found'; PUT /:id/tags via Update.
Task 4: complete (commits c54fd61..386fff7, review approved)
Task 4: minor (deferred): nested link tags/total_clicks placeholders (ruled above); link_id 404 message "QR code not found" (plan-mandated, kept); SetTags drops prev; dead domain-change branch; untested caps/null tags.
Task 5: dispatched (BASE 386fff7, implementer opus)
Task 6: implemented ace65e1 (qr-code-styling 1.9.2; frame rendering unverified in a real browser → smoke).
Task 6: review needs fixes — Ruling: frame border scales with size (4%) instead of the plan's fixed 8 px, so preview and export agree — cost if wrong: thin border on small previews. Fix round 1 dispatched (FIX_BASE ace65e1) incl. minors: empty frame text band, import catch, bulk invalidates search.
Task 6: fix round 1/5 (4 addressed, 0 open; commits ace65e1..29621ad)
Task 6: complete (commits 386fff7..29621ad, review clean after 1 fix round)
Task 6: minor (deferred): double JPEG compression when framed; SVG long frame text overflow; uploads route 500 vs 502 when backend down; QrPreview memo on object identity; trimmed-gate vs untrimmed render of frame text.
Task 7: dispatched (BASE 29621ad, implementer opus) — frontend-only, parallel with backend Task 5 (disjoint files).
Task 5: implemented 6d3f461 (package built as 29621ad..6d3f461, backend commit only).
Task 5: complete (commits 29621ad..6d3f461, review approved). Controller resolved ⚠️: /uploads relative URL is served through the Next route added in Task 6; vendor dir is a local untracked artifact (see Dockerfile check below).
Task 5: minor (deferred): no test for >6MB MaxBytesReader branch; no nosniff on /uploads; no upload quota; crash leftovers of temp files; Save ignores ctx/userID.
Backend tasks 1–5 complete.
Task 7: implemented 461c003 (QrFilterBar sibling; LinksToolbar view props optional; BulkTagDialog noun; TagEditor tagType/onSave).
Task 7: complete (commits 6d3f461..461c003, review approved). Ruling: delete copy "This permanently deletes the QR Code and its short link." (only when link_source='qr') — plan text was ambiguous — cost if wrong: copy tweak.
Task 7: minor (deferred): QrList/QrFilterBar duplicate LinksList/LinksFilterBar logic (shared hooks before a third list); TagEditor props not a discriminated union; no QrList tests; aria "Select all codes".
Task 8: dispatched (BASE 461c003, implementer opus)
Task 11 (docs step only): dispatched (BASE 461c003, implementer opus) — parallel with Task 8; touches README/docker-compose/config.example only.
Task 11 docs step: complete (commit d044970) — Ruling: docs-only diff reviewed by the final whole-branch review instead of a task review — cost if wrong: a README inaccuracy slips until final review.
Task 8: implemented 528d0a3
Task 8: review needs fixes (Important: modal Download icon-only). Fix round 1 dispatched (FIX_BASE 528d0a3) incl. minors: design seed race, stale design on logo upload, double 409 toast, Enter on step 2. Rulings: Domain select replaced by ShortLinkField's while sharing — accepted; Advanced settings hidden for ?link= — accepted (backend ignores them) — cost if wrong: minor UI deviation.
Task 8: fix round 1/5 (5 addressed, 0 open; commits 528d0a3..ddbdbe0)
Task 8: complete (commits d044970..ddbdbe0, review clean after 1 fix round)
Task 8: minor (deferred): no client-side logo size/type pre-check; invalid hex no feedback; no page-level tests (buildInput, 409 routing, ?link/?design); nested useActiveGroupId() call; unused QR_BACKGROUNDS export.
Task 9: dispatched (BASE ddbdbe0, implementer opus)
Task 9: implemented 637e275 (edit page sends only changed fields — Ruling: accepted, avoids saving fallback title — cost if wrong: none visible).
Task 9: complete (commits ddbdbe0..637e275, review approved)
Task 9: minor (deferred): edit expiration compare at ms precision (minute-precision input) can block unrelated saves; mixed frozen/live baseline in edit; alias test name; thin edit tests; a11y (banner icon aria-hidden, Field dl/dt/dd, role=status); scansOnly naming; hostOf duplicated; routing URLs not validated inline on edit.
Task 10: dispatched (BASE 637e275, implementer opus)
Smoke (controller, API via curl against restarted backend at 6d3f461+): migration 005 recorded by the startup runner; POST /qr-codes creates a source=qr link hidden from /links (total 0) and counted by /search (qr_codes 1); GET {short_url}?r=qr → 302 twice + one plain click → clicks table source qr=2, link=1 after the 30 s flush; /qr-codes/:id/stats forces source=qr (total 2, series point); list total_scans 2; image svg (viewBox 41, headers ok) and png 256×256; /tags?type=qr → ["smoke"]. Browser smoke pending Task 10.
Task 10: implemented 3cc4ace
Task 10: complete (commits 637e275..3cc4ace, review approved)
Task 10: minor (deferred): QrCard on link page has no error state (skeleton forever on 404); create-link→QR follow-up duplicated in QuickCreate and LinkForm; untested LinkForm QR switch/logo/QrCard/GettingStarted; logo uploads eagerly; skeleton not aria-busy; stale ?url= handling on create page.
All tasks 1–11(docs) complete. Browser smoke in progress: Home QR mode → "Your code is ready!" dialog OK (QR id 2).
Smoke (controller, browser at 3cc4ace): login → Home QR mode (Domain row, "Also create a link for this QR Code", Enter) → "Your code is ready!" (preview canvas, Customize design / View code details / labelled Download) → details page (Code URL, Scan data modules) → Links ⋯ "Create QR Code" → step 1 attached-link panel → step 2 rounded + #2A5BD7 + text-bottom "Scan to visit" (240 px live canvas) → Create → persisted design; link.qr_code_id=3 → Download PNG ("From link.png", image/png blob) and SVG (image/svg+xml) → 2 curl scans → list shows "2 scans", frame visible on thumbnail, short link only on shared code; API: hide (archive) keeps scanning, delete QR owning link removes link (404) and redirect, delete link cascades QR, export header; uploads: PNG 201 + served via backend and Next proxy, SVG 400; Home link mode + "Also create a QR Code" → "View QR Code" in dialog; link details QR card 120 px canvas + Download/Customize; customize Save → "Design saved" (color persisted); edit Save → "QR Code updated" (title). All app requests 200; console 404s unrelated to QR routes. Smoke PASSED.
Full checks at 3cc4ace (controller): go vet clean, go test -p 1 ./... all packages ok; frontend vitest/lint/tsc/build green (see tasks output). Final whole-branch review dispatched (opus, package 1c38114..3cc4ace).
Final review (opus): With fixes — I1 link delete silently cascades QR (copy + stale QR caches on delete/update). Minors: checklist scans ignore QR-only scans; Code URL anchor records scans; create-step preview encodes wrong data; logo URL unvalidated server-side; nosniff; cascade not logging qr.delete; EditBackHalf on source=qr link; §6.2 POST /links qr{} deviation; hostOf duplication/memo.
Final review: Ruling: §6.2 `POST /links` with `qr{render_customizations}` is replaced by a second `POST /qr-codes {link_id}` call (non-atomic, toast on failure) — cost if wrong: a link without its QR on a transient failure.
Final review: Ruling: `GET /links/:id` and `/links/:id/stats` may still return QR-owned links by id (owner-only, read-only) — accepted; EditBackHalf is guarded instead.
Final fix wave dispatched (FIX_BASE 3cc4ace, implementer opus): I1 + minors 2, 4, 5, 6, 8 + QrCard error state (Task 10 deferred) + hostOf shared helper/memo.
Final fix wave: commits 0c12182 (backend) + 3551148 (frontend); scoped re-review dispatched.
Controller verified at 3551148: go vet/test all ok; vitest 138/138, lint, tsc, build green; live backend: back-half edit on QR-owned link → 404, external logo URL → 400, uploaded path → 200, nosniff on /uploads via backend and Next proxy.
Final fix wave re-review: all 8 findings addressed, no new Critical/Important breakage. Out-of-scope: pre-existing rows with a non-upload logo would 400 on re-save (none expected).
Phase 2 complete: commits 3287ef0..3551148 (18 commits incl. plan). Checks green at 3551148 (controller). Smoke passed at 3cc4ace + fix-wave behaviour verified live at 3551148.

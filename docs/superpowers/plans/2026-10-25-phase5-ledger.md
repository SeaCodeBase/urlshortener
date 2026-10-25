# SDD ledger — plan: docs/superpowers/plans/2026-10-25-phase5-campaigns-webhooks-bulk.md
Spec: docs/superpowers/specs/2026-10-24-bitly-clone-design.md (§5.9–5.11, §6.1–6.3, §11.5).
Pre-flight scan:
| Pair / task | Produces vs consumes | Finding |
| T1↔T2/T3/T6 | repo interfaces per Shared contracts | consistent (names pinned in plan) |
| T2↔T9 | CampaignInput, CreateLinkInChannelInput | same fields; frontend snake_case json |
| T4↔T5 | ApplyDeliveryResult, SignBody | shared helpers in service, used by worker |
| T5↔click_flusher | SetEnqueuer hook after BatchInsert | only flusher file touched by T5 |
| T6↔T1 | BulkUploadRepository.ClaimNext semantics | plan pins LAST_INSERT_ID(id) trick |
| T7↔T8–T12 | hooks names | pinned in T7 Interfaces |
| T9 TimeSeriesChart | may need multiSeries prop | ruled: T9 may add an optional prop with a test |
| T12 DomainRow | useUpdateGroup may not exist | ruled: add to useGroups.ts if missing |
Self-consistency: each task's tests name the code it specifies; no test asserts nothing. Scan clean otherwise.
Ruling: webhooks table gains failing_since/last_delivery_at; bulk_uploads gains source_csv/processed/error — needed for spec behaviour (24h deactivation, progress, worker input); cost if wrong: two extra nullable columns.
Ruling: parallel dispatch T1 (backend) and T7 (frontend) — disjoint files.
BASE (T1,T7): 52d8808ce85a3efc54ea05b97b3633335da1b381
Task 1 dispatched (a5bc23915a202698d, opus); Task 7 dispatched (a2f091e9e974a3fdd, opus) in parallel.
Task 7: DONE_WITH_CONCERNS, commit f2f45f7 (vitest 309/309, lint, tsc ok). Ruling: concerns 1–3 adopted as contract rulings (appended to shared.md): snake_case stats JSON with {date,clicks} series; link items via presenter; PUT channel_ids replaces set. Task review dispatched.
Task 7: complete (review ✅/Approved; minor import-order and quote-style nits parked for the final polish).
BASE (T8): f2f45f7a10391e1a9330ce934df707570bf29854
Task 8 dispatched (a479dd993c6fea1f8, opus) in parallel with Task 1.
Task 1: DONE_WITH_CONCERNS, commit a09dbdc (Go suite green, repo tests on real DB). Rulings: upsert instead of INSERT IGNORE accepted (FK violations surface as errors; Task 2 validates channel∈campaign and links before AddLinks); error vars in repo files (existing convention); Webhook.Create derives is_active from status. Task review dispatched. Task 4 dispatched in parallel (disjoint files from Task 2).
BASE (T4): a09dbdc
BASE (T2): a09dbdc
Task 1 review dispatched (a2b0a77b3adaa33e4, opus). Task 4 dispatched (a52dcc167b4282692, opus). Ruling: T2/T6 wait for T4 (shared interfaces.go/activity_service.go/mocks); T3 and T5 both touch main.go → sequential.
Task 8: DONE, commit dee2e36 (vitest 322/322, lint/tsc/build ok). Review dispatched. Task 11 dispatched in parallel (frontend-only, disjoint).
BASE (T11): dee2e36
Task 1: complete (review ✅/Approved). Minors parked: stale 'INSERT IGNORE' comments (interfaces.go:410, campaign_repo_test.go:84); requireRowAffected could be reused by page_repo; AddLinks unchunked → Task 2 caps link_ids at 500 per request; Task 6 worker must reclaim jobs stuck in 'processing' (e.g. older than 10 min) — appended to shared.md.
Task 8 review dispatched (a32d4f3e2cf1c0774, sonnet); Task 11 dispatched (a143277f9bf3b93ed, opus).
Task 8: complete (review ✅/Approved; minors parked: unused utmHelp id, redundant saving state). Task 9 dispatched in parallel with Task 11 (disjoint files).
BASE (T9): dee2e3634c66b57c66d95b09c6a1004e6fec1661
Task 9 dispatched (a20d863489207e149, opus).
Task 4: DONE, commit a63fe6f (Go suite green). Rulings: userID on mutating methods; NewWebhookHTTPClient for service+dispatcher; DNSResolver.LookupIPAddr; verify keeps deactivated. Review dispatched. Task 2 dispatched (BASE a63fe6f).
Task 4 review dispatched (a09cd8f440b590a42, opus); Task 2 dispatched (abd582b9c52682c12, opus).
Task 11: DONE_WITH_CONCERNS, commit 07ac22a (14 new tests pass; the one red test is Task 9's uncommitted TimeSeriesChart work). Rulings: amber Alert chip accepted; absolute times accepted; extra copy accepted. Review dispatched. Task 12 dispatched (frontend-only).
BASE (T12): 07ac22a
Task 11 review dispatched (sonnet); Task 12 dispatched (opus).
Task 11: complete (review ✅/Approved; minors parked: shared chip-class helper, unused export WEBHOOK_COLUMNS, duplicated saving state, secret-not-in-toast assertion) — candidates for the final polish.
Task 4: complete (review ✅/Approved). Important hardening (isPublicIP misses 100.64/10, 0/8, 255.255.255.255, 64:ff9b::/96, 2002::/16, ::7f00:1-style v4-compatible, 240/4, 198.18/15) — Ruling: folded into Task 5 as a required step (same files, no second review cycle); minors (WithoutCancel on Verify, log scheme+host only, redundant LastDeliveryAt reset, extra test rows) also carried into Task 5. Task 5 dispatched in parallel with Task 2 (disjoint files).
BASE (T5): 07ac22a42dc70647e2b3854038159299ad7010f0
Task 5 dispatched (ab99a8bbfcde77168, opus) with the Task 4 hardening folded in.
Task 12: DONE, commit 0664e9d (15 new tests; red files belong to Task 9 in progress). Ruling: hide 'Set as default for group' when domain.group_id is set and differs from the active group — carried to the final polish list. Review dispatched. Task 10 dispatched.
BASE (T10): 0664e9d
Task 12 review dispatched (sonnet); Task 10 dispatched (a6100cea72f629a18, opus).
Task 12: complete (review ✅/Approved; minor: hide default-for-group item for domains scoped to another group → final polish).
Task 9: DONE, commit 2c8c0a3 (vitest 369/369, lint ok; tsc red only on Task 10's in-progress file). Review dispatched.
Task 10: DONE, commit eaf5c44 (links tests 68/68, lint ok). Rulings: campaign_id lives in linksUrlState.toApiParams; details card uses the Pages-card text button style. Review dispatched.
Task 2: DONE_WITH_CONCERNS, commit dd5c70c (suite green in a scratch worktree; main checkout red only from Task 5's in-progress edits). Rulings recorded in shared.md; orphan-link-on-membership-failure accepted (rare, visible in Links). Reviews for Tasks 2 and 10 dispatched. Task 3 waits for Task 5 (main.go).
Task 9: complete (review ✅/Approved; minors parked for final polish: ChannelCard mixes preset vs all-time counts and shows — for a known link count; dead 'custom' filter in CampaignPerformance; duplicated margin/axes in TimeSeriesChart multiSeries branch). Task 2 review dispatched (opus); Task 10 review dispatched (sonnet).
Task 10: complete (review ✅/Approved; minor: Filters campaign SelectValue blank when the URL campaign is not in the list → final polish).
Task 2: complete (review ✅/Approved; minors parked: List does Links+CountScope per campaign; LinkMemberships per-campaign loads; Update creates channels before update; orphan link on membership failure; Stats wraps StatsQueryError in ValidationError).
Task 5: DONE, commit fd8785c (Go suite green incl. -race on worker; dev backend restarted). Rulings: NewWebhookEnqueuer takes tagRepo; embedded-IPv4 ranges allowed only when the inner IPv4 is public (accepted); dispatcher/Verify read-modify-write race accepted (noted for ledger); no SIGTERM handling predates Phase 5 → Phase 6 polish item; at-least-once delivery accepted (documented in Task 13). Review dispatched. Task 3 dispatched (BASE fd8785c); Task 6 after Task 3.
BASE (T3): fd8785c
Task 5 review dispatched (af4435305cf7d7493, opus); Task 3 dispatched (ac99c1c7ba5680214, opus).
Task 5 review: spec ✅, quality NOT approved — Important: dispatcher full-row Update can revert concurrent PUT url/name and lose failure increments. Fix round 1 (resume implementer): column-scoped UpdateDeliveryState with WHERE is_active=1; also the three extra deny-list ranges, recover in deliver goroutine, enqueue-before-LTrim assertion.
Task 3: DONE, commit 2fcfdb5 (suite green; dev backend restarted on 8081). Review dispatched. Task 6 dispatched (BASE 2fcfdb5; disjoint from Task 5 fix files except repo mocks regen — deterministic).
Task 3 review dispatched (sonnet); Task 6 dispatched (a2a8d2ed7b98c977e, opus).
Task 3: complete (review ✅/Approved; 0 JSON mismatches vs frontend types; minors: user-scoped domain map in campaign link rendering, bindPageJSON naming).
Task 5 fix round 1: commit 83adc0a (UpdateDeliveryState with row lock + callback; Verify uses it; deny-list gaps; recover; flusher-order assertion). Ruling: callback signature accepted (import cycle). Scoped re-review dispatched.
Task 5: complete (re-review: all findings addressed, no new breakage; residual minor: user-driven Update/Deactivate/Activate still full-row writes — parked).
Task 6: DONE, commit 794ad47 (suite green; dev backend restarted). Concerns: shutdown marks in-flight job failed with partial CSV (accepted); crash-reclaim reruns from row 1 (documented); slow title fetches vs 10-min stale window — reviewer to judge. Review dispatched. Task 13 dispatched in parallel.
Task 6 review dispatched (opus); Task 13 dispatched (sonnet). Stale-window concern closed: title fetch ≤3 s/row, batch ≤150 s, updated_at bumped per batch.
Gates at 794ad47: Go all ok; vitest 385/385; lint/tsc/build clean.
Task 13: DONE, commit 1027cb7 (README Phase 5 section; code-vs-brief differences documented). Browser smoke in progress at 1027cb7.
Task 6 review: spec ❌ (Important: LazyQuotes swallows rows after an unterminated quote), quality approved. Fix round 1 dispatched (resume implementer).
Browser/API smoke at 1027cb7+73db29d: campaign create (new channel, auto-UTM) → detail → create link in channel → redirect carries utm_source=Email&utm_medium=bitly&utm_campaign=Spring+Sale → engagement 1 in list/stats → links?campaign_id=1 filter OK → webhook private URL 400, public create (64-char secret), verify ping recorded 405 failure (failures=1) → bulk upload 3 rows → worker done 3/3 (2 ok, 1 failed with readable error) → results CSV attachment → unterminated-quote CSV rejected 400 → Bulk upload and Webhooks pages render. PASS.
Task 6 fix round 1: commit 73db29d (strict quoting; Stop idempotent; shutdown-path test). Scoped re-review dispatched. Final whole-branch review dispatched in parallel (52d8808..HEAD).
Task 6: complete (re-review: all findings addressed, no new breakage). All 13 tasks complete. Final whole-branch review in progress.
Final review (52d8808..73db29d): 3 Important (I1 click-dup window around enqueue; I2 slow-endpoint head-of-line; I3 non-UTF-8 CSV 500) + 5 minors. Ruling: trim-before-enqueue (click integrity > webhook completeness); per-webhook in-flight cap; utf8 check; resume-from-processed on reclaim. ONE fix wave dispatched from final-findings.md (BASE 73db29d).
Final fix wave dispatched (a8ddf5b9a2a0fa83b, opus).
Final fix wave: bb32936 (I3) 510b218 (I1) fa63edb (I2) c8d3d21 (M1) c79cbee (M2-4) 4b22c57 (I2 paging); gates green per report. Rulings: ZADD XX in-place reschedule accepted; links lookup per batch accepted; 5 s webhook cache lag documented. Scoped re-review dispatched.
Re-smoke after fix wave (4b22c57): non-UTF-8 CSV → 400 with Excel hint; UTF-8 accent row processed (job 2 done 1/1); campaign stats totals {links 1, engagements 1}; deliveries intact; bulk link redirects. PASS.
Fix-wave re-review: I1, I3, M1–M4 ADDRESSED; I2 addressed as ruled but the count cap (2 jobs/webhook/poll, 5 s defer) is a ~1 event/s ceiling even for fast endpoints — backlog grows unbounded for busy groups. Ruling CHANGED: replace the per-poll count cap with a per-webhook in-flight concurrency cap (2, 1 in alert) tracked across polls; deliveries outlive the poll; only webhooks at their cap are deferred; jobs for deactivated/deleted webhooks are dropped in the claim loop. Fix round 2 dispatched (resume implementer). Cost if wrong: one more scoped re-review.
Fix round 2: f98270d (in-flight cap, deliveries outlive poll, drop deactivated in claim loop; 200 ms slot wait deviation accepted) + e031edc (per-batch insert ctx, gofmt). Scoped re-review dispatched.
Frontend gates at e031edc: vitest 392/392, lint/tsc/build clean.
Round 2 re-review (f98270d, e031edc): I2 v2, gofmt, README, per-batch insert ctx ADDRESSED; no new Critical/Important. Residual minor (parked): successful slot waits are not charged to the 1 s budget, so a ~400 ms-latency webhook with a big backlog can stretch one poll to ~20 s (others delayed, not starved).
Gates at e031edc: Go all ok (+ race ×3 on worker); vitest 392/392; lint/tsc/build clean.
Phase 5: complete. Final review clean.

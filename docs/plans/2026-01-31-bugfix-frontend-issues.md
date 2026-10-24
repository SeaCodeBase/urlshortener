# Frontend Bug Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix five bugs discovered during testing: Invalid Date display, Stats page crash, timestamp zero values, date format parsing error, and short code minimum length.

**Architecture:** Fix backend JSON serialization for `sql.NullTime`, ensure empty slices instead of nil for stats, return proper timestamps from Create operations, fix date parsing for datetime-local input, and adjust short code minimum length.

**Tech Stack:** Go/Gin backend, Next.js frontend, MariaDB

---

## Bug Summary

| Bug | Location | Symptom | Root Cause |
|-----|----------|---------|------------|
| #1 | Dashboard links table | "Invalid Date" in Expires column | `sql.NullTime` serializes as `{"Time":"...","Valid":false}` instead of `null` |
| #2 | Stats page | Page crashes with TypeError | Backend returns `null` instead of `[]` for empty stats arrays |
| #3 | API responses | Timestamps show "0001-01-01" | Create() doesn't return DB-generated timestamps |
| #4 | Create link with expiry | Error: `cannot parse "" as "-"` | Frontend sends `2026-02-28T23:59`, backend expects RFC3339 format |
| #5 | Custom Short Code | Cannot use 3-char codes | Validation requires minimum 4 characters, should allow 3 |

---

## Task 1: Fix sql.NullTime JSON Serialization

**Files:**
- Create: `backend/internal/model/nullable.go`
- Modify: `backend/internal/model/link.go`

**Step 1: Create custom NullTime type with proper JSON marshaling**

Create `backend/internal/model/nullable.go`:

```go
package model

import (
	"database/sql"
	"encoding/json"
	"time"
)

// NullTime is a wrapper around sql.NullTime that serializes to JSON properly.
// It serializes to null when invalid, or to an ISO8601 string when valid.
type NullTime struct {
	sql.NullTime
}

func (nt NullTime) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(nt.Time.Format(time.RFC3339))
}

func (nt *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		nt.Valid = false
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	nt.Time = t
	nt.Valid = true
	return nil
}
```

**Step 2: Update Link model to use custom NullTime**

Modify `backend/internal/model/link.go` line 14:

```go
// Change from:
ExpiresAt   sql.NullTime `db:"expires_at" json:"expires_at,omitempty"`

// To:
ExpiresAt   NullTime `db:"expires_at" json:"expires_at"`
```

**Step 3: Run tests to verify no regressions**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/model/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add backend/internal/model/nullable.go backend/internal/model/link.go
git commit -m "fix: proper JSON serialization for nullable time fields"
```

---

## Task 2: Fix Stats Service to Return Empty Slices

**Files:**
- Modify: `backend/internal/service/stats_service.go`

**Step 1: Update GetLinkStats to ensure non-nil slices**

Modify `backend/internal/service/stats_service.go` around lines 49-76:

```go
func (s *StatsServiceImpl) GetLinkStats(ctx context.Context, userID, linkID uint64) (*LinkStatsResponse, error) {
	// Verify ownership
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if err != nil {
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	// Get all stats
	stats, err := s.clickRepo.GetStatsByLinkID(ctx, linkID)
	if err != nil {
		return nil, err
	}

	daily, err := s.clickRepo.GetDailyStats(ctx, linkID, 30)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if daily == nil {
		daily = []repository.DailyClickStats{}
	}

	referrers, err := s.clickRepo.GetTopReferrers(ctx, linkID, 10)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if referrers == nil {
		referrers = []repository.ReferrerStats{}
	}

	devices, err := s.clickRepo.GetDeviceStats(ctx, linkID)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if devices == nil {
		devices = []repository.DeviceStats{}
	}

	browsers, err := s.clickRepo.GetBrowserStats(ctx, linkID)
	if err != nil {
		return nil, err
	}
	// Ensure non-nil slice for JSON serialization
	if browsers == nil {
		browsers = []repository.BrowserStats{}
	}

	return &LinkStatsResponse{
		TotalClicks:    stats.TotalClicks,
		UniqueVisitors: stats.UniqueVisitors,
		DailyStats:     daily,
		TopReferrers:   referrers,
		DeviceStats:    devices,
		BrowserStats:   browsers,
	}, nil
}
```

**Step 2: Run stats service tests**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -v -run Stats`
Expected: PASS

**Step 3: Commit**

```bash
git add backend/internal/service/stats_service.go
git commit -m "fix: return empty slices instead of nil in stats response"
```

---

## Task 3: Fix Link Create to Return Timestamps

**Files:**
- Modify: `backend/internal/repository/link_repo.go`

**Step 1: Update Create to fetch timestamps after insert**

Modify `backend/internal/repository/link_repo.go` Create function:

```go
func (r *LinkRepositoryImpl) Create(ctx context.Context, link *model.Link) error {
	query := `INSERT INTO links (user_id, short_code, original_url, title, expires_at, is_active)
			  VALUES (?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		link.UserID, link.ShortCode, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrShortCodeExists
		}
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	link.ID = uint64(id)

	// Fetch the created record to get DB-generated timestamps
	created, err := r.GetByID(ctx, link.ID)
	if err != nil {
		return err
	}
	link.CreatedAt = created.CreatedAt
	link.UpdatedAt = created.UpdatedAt

	return nil
}
```

**Step 2: Run link repository tests**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/repository/... -v -run Link`
Expected: PASS

**Step 3: Commit**

```bash
git add backend/internal/repository/link_repo.go
git commit -m "fix: return DB-generated timestamps from link creation"
```

---

## Task 4: Fix Date Format Parsing for Expiration

**Files:**
- Modify: `frontend/src/app/dashboard/page.tsx`

**Step 1: Update frontend to send RFC3339 format**

The `datetime-local` input produces `2026-02-28T23:59` but backend needs RFC3339 `2026-02-28T23:59:00Z`.

Modify `frontend/src/app/dashboard/page.tsx` in handleQuickCreate function around line 27-32:

```tsx
const handleQuickCreate = async (e: React.FormEvent) => {
  e.preventDefault();
  setError('');
  setLoading(true);

  try {
    // Convert datetime-local to RFC3339 format
    let expiresAtRFC3339: string | undefined;
    if (expiresAt) {
      // datetime-local format: "2026-02-28T23:59"
      // RFC3339 format needed: "2026-02-28T23:59:00Z"
      expiresAtRFC3339 = expiresAt + ':00Z';
    }

    await api.createLink({
      original_url: url,
      custom_code: customCode || undefined,
      title: title || undefined,
      expires_at: expiresAtRFC3339,
    });
    // ... rest of function
```

**Step 2: Verify the fix compiles**

Run: `cd /Users/jose/GolandProjects/urlshortener/frontend && npm run build`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add frontend/src/app/dashboard/page.tsx
git commit -m "fix: convert datetime-local to RFC3339 format for expiration"
```

---

## Task 5: Allow 3-Character Custom Short Codes

**Files:**
- Modify: `backend/internal/service/shortcode_service.go`
- Modify: `frontend/src/app/dashboard/page.tsx`

**Step 1: Update backend validation**

Modify `backend/internal/service/shortcode_service.go` line 57:

```go
// Change from:
if len(code) < 4 || len(code) > 16 {

// To:
if len(code) < 3 || len(code) > 16 {
```

**Step 2: Update frontend validation**

Modify `frontend/src/app/dashboard/page.tsx` around line 86-89:

```tsx
// Change from:
pattern="[a-zA-Z0-9]{4,16}"
title="4-16 alphanumeric characters"
// ...
<p className="text-xs text-gray-500">4-16 alphanumeric characters</p>

// To:
pattern="[a-zA-Z0-9]{3,16}"
title="3-16 alphanumeric characters"
// ...
<p className="text-xs text-gray-500">3-16 alphanumeric characters</p>
```

**Step 3: Update shortcode service test**

Modify `backend/internal/service/shortcode_service_test.go` to update test cases:

```go
// Change test case from:
{"too short 3 chars", "abc", false},
{"valid min length 4 chars", "ab12", true},

// To:
{"too short 2 chars", "ab", false},
{"valid min length 3 chars", "ab1", true},
{"valid 4 chars", "ab12", true},
```

**Step 4: Run tests**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -v -run ShortCode`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/service/shortcode_service.go backend/internal/service/shortcode_service_test.go frontend/src/app/dashboard/page.tsx
git commit -m "feat: allow 3-character custom short codes"
```

---

## Task 6: Rebuild and Test Docker

**Step 1: Rebuild Docker images**

Run: `cd /Users/jose/GolandProjects/urlshortener && docker compose build --no-cache`
Expected: Build succeeds

**Step 2: Restart containers**

Run: `docker compose down && docker compose up -d`
Expected: All containers healthy

**Step 3: Test API responses**

Run:
```bash
# Register new user and test
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"bugtest@example.com","password":"TestPass123"}' | jq -r .token)

# Create link and verify expires_at is null (not object)
curl -s -X POST 'http://127.0.0.1:8080/api/links' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"original_url":"https://example.com"}' | jq .expires_at

# Verify timestamps are not zero
curl -s -X POST 'http://127.0.0.1:8080/api/links' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"original_url":"https://example.com"}' | jq '.created_at, .updated_at'
```

Expected:
- `expires_at` should be `null` (not an object)
- `created_at` and `updated_at` should be valid timestamps (not "0001-01-01")

**Step 4: Commit if all tests pass**

```bash
git add -A
git commit -m "test: verify bug fixes with manual API testing"
```

---

## Task 7: Browser Testing

**Step 1: Navigate to dashboard**

Open http://127.0.0.1:3000/dashboard in browser

**Step 2: Verify "Invalid Date" bug is fixed**

- Create a new link without expiration
- Check the "Expires" column shows "Never" instead of "Invalid Date"

**Step 3: Verify Stats page works**

- Click "Stats" button on a link
- Page should load without crashing
- Should show "No data" messages for empty stats

**Step 4: Verify timestamps display correctly**

- Create a new link
- Check "Created" column shows today's date, not "1/1/1"

---

## Verification Checklist

- [ ] API returns `null` for empty `expires_at` (not `{"Time":"...", "Valid":false}`)
- [ ] API returns `[]` for empty stats arrays (not `null`)
- [ ] API returns valid timestamps for `created_at` and `updated_at`
- [ ] Dashboard shows "Never" for links without expiration
- [ ] Stats page loads without crashing
- [ ] Stats page shows "No data" messages for empty stats
- [ ] Created dates display correctly
- [ ] Can create link with expiration date (no parsing error)
- [ ] Can use 3-character custom short codes
- [ ] Frontend validation shows "3-16 alphanumeric characters"

# Admin Dashboard Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add missing admin dashboard functionality: password change, custom URL creation UI, URL expiration UI, and statistics page.

**Architecture:** Extend existing Go/Gin backend with new endpoints. Add React components to Next.js frontend. Follow existing patterns for handlers, services, and components.

**Tech Stack:** Go (Gin), Next.js 15, TypeScript, MariaDB, Redis, TailwindCSS, shadcn/ui

**Prerequisites:** Run the refactoring plan first (`2026-01-31-refactor-interfaces-di.md`) to set up interfaces and mockgen.

---

## Current State Analysis

| Feature | Backend Status | Frontend Status |
|---------|---------------|-----------------|
| Password Change | Missing | Missing |
| Custom URL Creation | Done (API supports `custom_code`) | Missing UI |
| URL Expiration | Done (API supports `expires_at`) | Missing UI |
| Statistics Page | Partial (clicks table exists, no endpoint) | Missing |

---

## Task 1: Add ChangePassword Handler

**Files:**
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/cmd/server/main.go`

### Step 1: Add ChangePassword handler method

Add to `backend/internal/handler/auth.go`:

```go
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.authService.ChangePassword(c.Request.Context(), userID, input)
	if errors.Is(err, service.ErrWrongPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	if errors.Is(err, repository.ErrUserNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}
```

### Step 2: Register the route

Add to `backend/cmd/server/main.go` in auth routes (after `/me`):

```go
auth.PUT("/password", authMiddleware, authHandler.ChangePassword)
```

### Step 3: Commit

```bash
git add backend/internal/handler/auth.go backend/cmd/server/main.go
git commit -m "feat(api): add PUT /api/auth/password endpoint"
```

---

## Task 2: Create StatsService

**Files:**
- Create: `backend/internal/service/stats_service.go`
- Create: `backend/internal/service/stats_service_test.go`

### Step 1: Create StatsService implementation

Create `backend/internal/service/stats_service.go`:

```go
package service

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/repository"
)

// Compile-time check: StatsServiceImpl implements StatsService
var _ StatsService = (*StatsServiceImpl)(nil)

type StatsServiceImpl struct {
	clickRepo repository.ClickRepository
	linkRepo  repository.LinkRepository
}

func NewStatsService(clickRepo repository.ClickRepository, linkRepo repository.LinkRepository) *StatsServiceImpl {
	return &StatsServiceImpl{
		clickRepo: clickRepo,
		linkRepo:  linkRepo,
	}
}

type LinkStatsResponse struct {
	TotalClicks    int64                        `json:"total_clicks"`
	UniqueVisitors int64                        `json:"unique_visitors"`
	DailyStats     []repository.DailyClickStats `json:"daily_stats"`
	TopReferrers   []repository.ReferrerStats   `json:"top_referrers"`
	DeviceStats    []repository.DeviceStats     `json:"device_stats"`
	BrowserStats   []repository.BrowserStats    `json:"browser_stats"`
}

func (s *StatsServiceImpl) GetLinkStats(ctx context.Context, userID, linkID uint64) (*LinkStatsResponse, error) {
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if err != nil {
		return nil, err
	}
	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	stats, err := s.clickRepo.GetStatsByLinkID(ctx, linkID)
	if err != nil {
		return nil, err
	}

	daily, err := s.clickRepo.GetDailyStats(ctx, linkID, 30)
	if err != nil {
		return nil, err
	}

	referrers, err := s.clickRepo.GetTopReferrers(ctx, linkID, 10)
	if err != nil {
		return nil, err
	}

	devices, err := s.clickRepo.GetDeviceStats(ctx, linkID)
	if err != nil {
		return nil, err
	}

	browsers, err := s.clickRepo.GetBrowserStats(ctx, linkID)
	if err != nil {
		return nil, err
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

### Step 2: Write tests

Create `backend/internal/service/stats_service_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/repository/mocks"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestStatsServiceImpl_GetLinkStats_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	link := &model.Link{ID: 1, UserID: 1, ShortCode: "abc123"}

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(link, nil)

	mockClickRepo.EXPECT().
		GetStatsByLinkID(gomock.Any(), uint64(1)).
		Return(&repository.ClickStats{TotalClicks: 100, UniqueVisitors: 50}, nil)

	mockClickRepo.EXPECT().
		GetDailyStats(gomock.Any(), uint64(1), 30).
		Return([]repository.DailyClickStats{{Date: "2026-01-31", Clicks: 10}}, nil)

	mockClickRepo.EXPECT().
		GetTopReferrers(gomock.Any(), uint64(1), 10).
		Return([]repository.ReferrerStats{{Referrer: "google.com", Count: 20}}, nil)

	mockClickRepo.EXPECT().
		GetDeviceStats(gomock.Any(), uint64(1)).
		Return([]repository.DeviceStats{{DeviceType: "desktop", Count: 60}}, nil)

	mockClickRepo.EXPECT().
		GetBrowserStats(gomock.Any(), uint64(1)).
		Return([]repository.BrowserStats{{Browser: "Chrome", Count: 40}}, nil)

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(1))
	assert.NoError(t, err)
	assert.Equal(t, int64(100), resp.TotalClicks)
	assert.Equal(t, int64(50), resp.UniqueVisitors)
}

func TestStatsServiceImpl_GetLinkStats_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClickRepo := mocks.NewMockClickRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	svc := service.NewStatsService(mockClickRepo, mockLinkRepo)

	link := &model.Link{ID: 1, UserID: 2, ShortCode: "abc123"} // Different user

	mockLinkRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(link, nil)

	resp, err := svc.GetLinkStats(context.Background(), uint64(1), uint64(1))
	assert.ErrorIs(t, err, service.ErrNotLinkOwner)
	assert.Nil(t, resp)
}
```

### Step 3: Run tests

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -run TestStatsService -v`
Expected: All tests PASS

### Step 4: Commit

```bash
git add backend/internal/service/stats_service.go backend/internal/service/stats_service_test.go
git commit -m "feat(service): add StatsService with tests"
```

---

## Task 3: Create StatsHandler

**Files:**
- Create: `backend/internal/handler/stats.go`
- Modify: `backend/cmd/server/main.go`

### Step 1: Create StatsHandler

Create `backend/internal/handler/stats.go`:

```go
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
)

type StatsHandler struct {
	statsService service.StatsService
}

func NewStatsHandler(statsService service.StatsService) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
	}
}

func (h *StatsHandler) GetLinkStats(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	stats, err := h.statsService.GetLinkStats(c.Request.Context(), userID, linkID)
	if errors.Is(err, repository.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
```

### Step 2: Register route

Add to `backend/cmd/server/main.go`:

```go
// Create stats service and handler
statsService := service.NewStatsService(clickRepo, linkRepo)
statsHandler := handler.NewStatsHandler(statsService)

// Add route under links group
links.GET("/:id/stats", statsHandler.GetLinkStats)
```

### Step 3: Commit

```bash
git add backend/internal/handler/stats.go backend/cmd/server/main.go
git commit -m "feat(api): add GET /api/links/:id/stats endpoint"
```

---

## Task 4: Frontend - Password Change Page

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Create: `frontend/src/app/dashboard/settings/page.tsx`
- Modify: `frontend/src/app/dashboard/layout.tsx`

### Step 1: Add changePassword to API client

Add to `frontend/src/lib/api.ts`:

```typescript
async changePassword(oldPassword: string, newPassword: string) {
  return this.request<{ message: string }>('/api/auth/password', {
    method: 'PUT',
    body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
  });
}
```

### Step 2: Create settings page

Create `frontend/src/app/dashboard/settings/page.tsx`:

```tsx
'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { api } from '@/lib/api';

export default function SettingsPage() {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');

    if (newPassword !== confirmPassword) {
      setError('New passwords do not match');
      return;
    }

    if (newPassword.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    setLoading(true);
    try {
      await api.changePassword(oldPassword, newPassword);
      setSuccess('Password changed successfully');
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to change password');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-md">
      <Card>
        <CardHeader>
          <CardTitle>Change Password</CardTitle>
          <CardDescription>Update your account password</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="oldPassword">Current Password</Label>
              <Input id="oldPassword" type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="newPassword">New Password</Label>
              <Input id="newPassword" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required minLength={8} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="confirmPassword">Confirm New Password</Label>
              <Input id="confirmPassword" type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} required minLength={8} />
            </div>
            {error && <p className="text-sm text-red-500">{error}</p>}
            {success && <p className="text-sm text-green-500">{success}</p>}
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? 'Changing...' : 'Change Password'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
```

### Step 3: Add Settings nav link

Update `frontend/src/app/dashboard/layout.tsx` nav:

```tsx
<div className="flex items-center">
  <Link href="/dashboard" className="text-xl font-bold">URL Shortener</Link>
  <nav className="ml-8 flex gap-4">
    <Link href="/dashboard" className="text-sm text-gray-600 hover:text-gray-900">Links</Link>
    <Link href="/dashboard/settings" className="text-sm text-gray-600 hover:text-gray-900">Settings</Link>
  </nav>
</div>
```

### Step 4: Commit

```bash
git add frontend/src/lib/api.ts frontend/src/app/dashboard/settings/page.tsx frontend/src/app/dashboard/layout.tsx
git commit -m "feat(frontend): add password change settings page"
```

---

## Task 5: Frontend - Enhanced Link Creation Form

**Files:**
- Modify: `frontend/src/app/dashboard/page.tsx`

### Step 1: Update dashboard with advanced options

Replace `frontend/src/app/dashboard/page.tsx`:

```tsx
'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LinkTable } from '@/components/links/LinkTable';
import { api } from '@/lib/api';

export default function DashboardPage() {
  const [url, setUrl] = useState('');
  const [customCode, setCustomCode] = useState('');
  const [title, setTitle] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);

  const handleQuickCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await api.createLink({
        original_url: url,
        custom_code: customCode || undefined,
        title: title || undefined,
        expires_at: expiresAt || undefined,
      });
      setUrl('');
      setCustomCode('');
      setTitle('');
      setExpiresAt('');
      setRefreshKey(k => k + 1);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create link');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Create Short Link</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleQuickCreate} className="space-y-4">
            <div className="flex gap-4">
              <Input
                type="url"
                placeholder="Enter URL to shorten"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                className="flex-1"
                required
              />
              <Button type="submit" disabled={loading}>
                {loading ? 'Creating...' : 'Shorten'}
              </Button>
            </div>

            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setShowAdvanced(!showAdvanced)}
            >
              {showAdvanced ? 'Hide' : 'Show'} Advanced Options
            </Button>

            {showAdvanced && (
              <div className="grid gap-4 pt-2 border-t">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="customCode">Custom Short Code (optional)</Label>
                    <Input
                      id="customCode"
                      placeholder="my-custom-url"
                      value={customCode}
                      onChange={(e) => setCustomCode(e.target.value)}
                      pattern="[a-zA-Z0-9]{4,16}"
                      title="4-16 alphanumeric characters"
                    />
                    <p className="text-xs text-gray-500">4-16 alphanumeric characters</p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="title">Title (optional)</Label>
                    <Input
                      id="title"
                      placeholder="My Link Title"
                      value={title}
                      onChange={(e) => setTitle(e.target.value)}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="expiresAt">Expires At (optional)</Label>
                  <Input
                    id="expiresAt"
                    type="datetime-local"
                    value={expiresAt}
                    onChange={(e) => setExpiresAt(e.target.value)}
                    min={new Date().toISOString().slice(0, 16)}
                  />
                </div>
              </div>
            )}

            {error && <p className="text-sm text-red-500">{error}</p>}
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Your Links</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkTable key={refreshKey} />
        </CardContent>
      </Card>
    </div>
  );
}
```

### Step 2: Commit

```bash
git add frontend/src/app/dashboard/page.tsx
git commit -m "feat(frontend): add custom code, title, expiration to link form"
```

---

## Task 6: Frontend - Statistics Page

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/lib/api.ts`
- Create: `frontend/src/app/dashboard/links/[id]/stats/page.tsx`
- Modify: `frontend/src/components/links/LinkTable.tsx`

### Step 1: Add stats types

Add to `frontend/src/types/index.ts`:

```typescript
export interface LinkStats {
  total_clicks: number;
  unique_visitors: number;
  daily_stats: { date: string; clicks: number }[];
  top_referrers: { referrer: string; count: number }[];
  device_stats: { device_type: string; count: number }[];
  browser_stats: { browser: string; count: number }[];
}
```

### Step 2: Add API method

Add to `frontend/src/lib/api.ts`:

```typescript
async getLinkStats(id: number) {
  return this.request<LinkStats>(`/api/links/${id}/stats`);
}
```

### Step 3: Create stats page

Create `frontend/src/app/dashboard/links/[id]/stats/page.tsx`:

```tsx
'use client';

import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '@/lib/api';
import type { LinkStats, Link as LinkType } from '@/types';

export default function LinkStatsPage() {
  const params = useParams();
  const linkId = Number(params.id);
  const [stats, setStats] = useState<LinkStats | null>(null);
  const [link, setLink] = useState<LinkType | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, [linkId]);

  const loadData = async () => {
    try {
      const [statsData, linkData] = await Promise.all([
        api.getLinkStats(linkId),
        api.getLink(linkId),
      ]);
      setStats(statsData);
      setLink(linkData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load stats');
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div>Loading statistics...</div>;
  if (error) return <div className="text-red-500">{error}</div>;
  if (!stats || !link) return <div>No data available</div>;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Link Statistics</h1>
          <p className="text-gray-600">{link.short_url}</p>
          <p className="text-sm text-gray-500 truncate max-w-md">{link.original_url}</p>
        </div>
        <Link href="/dashboard">
          <Button variant="outline">Back to Dashboard</Button>
        </Link>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-500">Total Clicks</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{stats.total_clicks}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-gray-500">Unique Visitors</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-bold">{stats.unique_visitors}</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Clicks Over Time (Last 30 Days)</CardTitle>
        </CardHeader>
        <CardContent>
          {stats.daily_stats.length === 0 ? (
            <p className="text-gray-500">No click data yet</p>
          ) : (
            <div className="space-y-2">
              {stats.daily_stats.map((day) => (
                <div key={day.date} className="flex items-center gap-4">
                  <span className="w-24 text-sm text-gray-600">{day.date}</span>
                  <div className="flex-1 bg-gray-100 rounded-full h-4">
                    <div
                      className="bg-blue-500 h-4 rounded-full"
                      style={{
                        width: `${Math.min(100, (day.clicks / Math.max(...stats.daily_stats.map(d => d.clicks))) * 100)}%`,
                      }}
                    />
                  </div>
                  <span className="w-12 text-sm font-medium">{day.clicks}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid grid-cols-3 gap-4">
        <Card>
          <CardHeader><CardTitle>Top Referrers</CardTitle></CardHeader>
          <CardContent>
            {stats.top_referrers.length === 0 ? (
              <p className="text-gray-500 text-sm">No data</p>
            ) : (
              <ul className="space-y-2">
                {stats.top_referrers.map((ref) => (
                  <li key={ref.referrer} className="flex justify-between text-sm">
                    <span className="truncate max-w-[150px]">{ref.referrer}</span>
                    <span className="font-medium">{ref.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Devices</CardTitle></CardHeader>
          <CardContent>
            {stats.device_stats.length === 0 ? (
              <p className="text-gray-500 text-sm">No data</p>
            ) : (
              <ul className="space-y-2">
                {stats.device_stats.map((device) => (
                  <li key={device.device_type} className="flex justify-between text-sm">
                    <span className="capitalize">{device.device_type}</span>
                    <span className="font-medium">{device.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Browsers</CardTitle></CardHeader>
          <CardContent>
            {stats.browser_stats.length === 0 ? (
              <p className="text-gray-500 text-sm">No data</p>
            ) : (
              <ul className="space-y-2">
                {stats.browser_stats.map((browser) => (
                  <li key={browser.browser} className="flex justify-between text-sm">
                    <span>{browser.browser}</span>
                    <span className="font-medium">{browser.count}</span>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
```

### Step 4: Add Stats button to LinkTable

Update `frontend/src/components/links/LinkTable.tsx`:

Add import:
```tsx
import Link from 'next/link';
```

Update Actions column:
```tsx
<TableCell>
  <div className="flex gap-2">
    <Link href={`/dashboard/links/${link.id}/stats`}>
      <Button variant="outline" size="sm">Stats</Button>
    </Link>
    <Button variant="destructive" size="sm" onClick={() => handleDelete(link.id)}>
      Delete
    </Button>
  </div>
</TableCell>
```

### Step 5: Commit

```bash
git add frontend/src/types/index.ts frontend/src/lib/api.ts frontend/src/app/dashboard/links/[id]/stats/page.tsx frontend/src/components/links/LinkTable.tsx
git commit -m "feat(frontend): add statistics page with analytics"
```

---

## Task 7: Add Expiration Column to LinkTable

**Files:**
- Modify: `frontend/src/components/links/LinkTable.tsx`

### Step 1: Add expiration column

Add to TableHeader:
```tsx
<TableHead>Expires</TableHead>
```

Add to TableBody after Status:
```tsx
<TableCell>
  {link.expires_at ? (
    <span className={new Date(link.expires_at) < new Date() ? 'text-red-500' : ''}>
      {new Date(link.expires_at).toLocaleDateString()}
    </span>
  ) : (
    <span className="text-gray-400">Never</span>
  )}
</TableCell>
```

### Step 2: Commit

```bash
git add frontend/src/components/links/LinkTable.tsx
git commit -m "feat(frontend): add expiration column to links table"
```

---

## Final Verification

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend && go test ./... -v
cd /Users/jose/GolandProjects/urlshortener/frontend && npm run build
```

All tests should pass and frontend should build successfully.

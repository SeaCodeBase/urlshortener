# Stats & Links Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix device/browser detection bug and redesign Stats page + Links list to match Bitly style.

**Architecture:** Backend adds User-Agent parsing and GeoIP lookup in click_flusher. Frontend uses Recharts for donut/bar charts, replaces table with card-based layout.

**Tech Stack:** Go (mssola/useragent, oschwald/geoip2-golang), React/Next.js, Recharts, Tailwind/shadcn

---

## Task 1: Add User-Agent Parsing Library

**Files:**
- Modify: `backend/go.mod`
- Create: `backend/internal/util/useragent.go`
- Create: `backend/internal/util/useragent_test.go`

**Step 1: Write the failing test**

Create `backend/internal/util/useragent_test.go`:

```go
package util

import "testing"

func TestParseUserAgent_Chrome(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	result := ParseUserAgent(ua)

	if result.Browser != "Chrome" {
		t.Errorf("expected browser Chrome, got %s", result.Browser)
	}
	if result.DeviceType != "desktop" {
		t.Errorf("expected device desktop, got %s", result.DeviceType)
	}
}

func TestParseUserAgent_Safari_Mobile(t *testing.T) {
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	result := ParseUserAgent(ua)

	if result.Browser != "Safari" {
		t.Errorf("expected browser Safari, got %s", result.Browser)
	}
	if result.DeviceType != "mobile" {
		t.Errorf("expected device mobile, got %s", result.DeviceType)
	}
}

func TestParseUserAgent_Empty(t *testing.T) {
	result := ParseUserAgent("")

	if result.Browser != "Unknown" {
		t.Errorf("expected browser Unknown, got %s", result.Browser)
	}
	if result.DeviceType != "unknown" {
		t.Errorf("expected device unknown, got %s", result.DeviceType)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go test ./internal/util/... -v`
Expected: FAIL (package/function not found)

**Step 3: Add dependency and implement**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go get github.com/mssola/useragent`

Create `backend/internal/util/useragent.go`:

```go
package util

import "github.com/mssola/useragent"

type UAResult struct {
	Browser    string
	DeviceType string
}

func ParseUserAgent(uaString string) UAResult {
	if uaString == "" {
		return UAResult{Browser: "Unknown", DeviceType: "unknown"}
	}

	ua := useragent.New(uaString)

	browserName, _ := ua.Browser()
	if browserName == "" {
		browserName = "Unknown"
	}

	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
	} else if ua.Tablet() {
		deviceType = "tablet"
	}

	return UAResult{
		Browser:    browserName,
		DeviceType: deviceType,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go test ./internal/util/... -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/go.mod backend/go.sum backend/internal/util/
git commit -m "feat(backend): add user-agent parsing utility"
```

---

## Task 2: Add GeoIP Library and Config

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/internal/config/config.go`
- Create: `backend/internal/util/geoip.go`
- Create: `backend/internal/util/geoip_test.go`

**Step 1: Add GeoIP dependency**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go get github.com/oschwald/geoip2-golang`

**Step 2: Update config to include GeoIP path**

Read current config first, then add GeoIPPath field to Config struct in `backend/internal/config/config.go`:

```go
// Add to Config struct:
GeoIPPath string `envconfig:"GEOIP_PATH" default:""`
```

**Step 3: Create GeoIP utility**

Create `backend/internal/util/geoip.go`:

```go
package util

import (
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

type GeoIPResult struct {
	Country string
	City    string
}

type GeoIPLookup struct {
	db   *geoip2.Reader
	mu   sync.RWMutex
}

var geoIPInstance *GeoIPLookup
var geoIPOnce sync.Once

func InitGeoIP(dbPath string) error {
	if dbPath == "" {
		return nil // GeoIP disabled
	}

	var initErr error
	geoIPOnce.Do(func() {
		db, err := geoip2.Open(dbPath)
		if err != nil {
			initErr = err
			return
		}
		geoIPInstance = &GeoIPLookup{db: db}
	})
	return initErr
}

func CloseGeoIP() {
	if geoIPInstance != nil && geoIPInstance.db != nil {
		geoIPInstance.db.Close()
	}
}

func LookupIP(ipStr string) GeoIPResult {
	if geoIPInstance == nil || geoIPInstance.db == nil {
		return GeoIPResult{}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return GeoIPResult{}
	}

	geoIPInstance.mu.RLock()
	defer geoIPInstance.mu.RUnlock()

	record, err := geoIPInstance.db.City(ip)
	if err != nil {
		return GeoIPResult{}
	}

	return GeoIPResult{
		Country: record.Country.IsoCode,
		City:    record.City.Names["en"],
	}
}
```

**Step 4: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/go.mod backend/go.sum backend/internal/config/config.go backend/internal/util/geoip.go
git commit -m "feat(backend): add GeoIP lookup utility"
```

---

## Task 3: Add ip_address Column to Database

**Files:**
- Create: `backend/migrations/004_add_ip_address.sql`
- Modify: `backend/internal/model/click.go`

**Step 1: Create migration file**

Create `backend/migrations/004_add_ip_address.sql`:

```sql
-- Add ip_address column to clicks table
ALTER TABLE clicks ADD COLUMN ip_address VARCHAR(45) AFTER ip_hash;

-- Create index for potential future queries by IP
CREATE INDEX idx_clicks_ip_address ON clicks(ip_address);
```

**Step 2: Update Click model**

Modify `backend/internal/model/click.go` to add IPAddress field after IPHash:

```go
type Click struct {
	ID          uint64    `db:"id" json:"id"`
	LinkID      uint64    `db:"link_id" json:"link_id"`
	ClickedAt   time.Time `db:"clicked_at" json:"clicked_at"`
	IPHash      string    `db:"ip_hash" json:"-"`
	IPAddress   string    `db:"ip_address" json:"-"`
	UserAgent   string    `db:"user_agent" json:"user_agent"`
	Referrer    string    `db:"referrer" json:"referrer"`
	Country     string    `db:"country" json:"country"`
	City        string    `db:"city" json:"city"`
	DeviceType  string    `db:"device_type" json:"device_type"`
	Browser     string    `db:"browser" json:"browser"`
	UTMSource   string    `db:"utm_source" json:"utm_source"`
	UTMMedium   string    `db:"utm_medium" json:"utm_medium"`
	UTMCampaign string    `db:"utm_campaign" json:"utm_campaign"`
}
```

**Step 3: Update BatchInsert query**

Modify `backend/internal/repository/click_repo.go` BatchInsert to include ip_address:

```go
query := `INSERT INTO clicks (link_id, clicked_at, ip_hash, ip_address, user_agent, referrer, country, city, device_type, browser, utm_source, utm_medium, utm_campaign)
		  VALUES (:link_id, :clicked_at, :ip_hash, :ip_address, :user_agent, :referrer, :country, :city, :device_type, :browser, :utm_source, :utm_medium, :utm_campaign)`
```

**Step 4: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/migrations/004_add_ip_address.sql backend/internal/model/click.go backend/internal/repository/click_repo.go
git commit -m "feat(backend): add ip_address column to clicks table"
```

---

## Task 4: Update Click Event to Include Raw IP

**Files:**
- Modify: `backend/internal/service/click_service.go`
- Modify: `backend/internal/handler/redirect.go`

**Step 1: Add IPAddress to ClickEvent struct**

Modify `backend/internal/service/click_service.go`:

```go
type ClickEvent struct {
	LinkID      uint64    `json:"link_id"`
	ClickedAt   time.Time `json:"clicked_at"`
	IPHash      string    `json:"ip_hash"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Referrer    string    `json:"referrer"`
	UTMSource   string    `json:"utm_source,omitempty"`
	UTMMedium   string    `json:"utm_medium,omitempty"`
	UTMCampaign string    `json:"utm_campaign,omitempty"`
}
```

**Step 2: Update redirect handler to include raw IP**

Modify `backend/internal/handler/redirect.go` in the Redirect function:

```go
event := service.ClickEvent{
	LinkID:      linkID,
	ClickedAt:   time.Now().UTC(),
	IPHash:      h.hashIP(c.ClientIP()),
	IPAddress:   c.ClientIP(),
	UserAgent:   c.GetHeader("User-Agent"),
	Referrer:    c.GetHeader("Referer"),
	UTMSource:   c.Query("utm_source"),
	UTMMedium:   c.Query("utm_medium"),
	UTMCampaign: c.Query("utm_campaign"),
}
```

**Step 3: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/internal/service/click_service.go backend/internal/handler/redirect.go
git commit -m "feat(backend): capture raw IP address in click events"
```

---

## Task 5: Update Click Flusher with UA Parsing and GeoIP

**Files:**
- Modify: `backend/internal/worker/click_flusher.go`

**Step 1: Update click_flusher.go to use UA parsing and GeoIP**

```go
package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/util"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// ... keep existing struct and constructor ...

func (f *ClickFlusher) flush() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	bufferKey := "clicks:buffer"

	for {
		events, err := f.rdb.LRange(ctx, bufferKey, 0, int64(f.batchSize-1)).Result()
		if err != nil {
			logger.Log.Errorf("Failed to get click events from buffer: %v", err)
			return
		}

		if len(events) == 0 {
			return
		}

		clicks := make([]model.Click, 0, len(events))
		for _, eventData := range events {
			var event service.ClickEvent
			if err := json.Unmarshal([]byte(eventData), &event); err != nil {
				logger.Log.Warnf("Failed to unmarshal click event: %v", err)
				continue
			}

			// Parse User-Agent
			uaResult := util.ParseUserAgent(event.UserAgent)

			// Lookup GeoIP
			geoResult := util.LookupIP(event.IPAddress)

			click := model.Click{
				LinkID:      event.LinkID,
				ClickedAt:   event.ClickedAt,
				IPHash:      event.IPHash,
				IPAddress:   event.IPAddress,
				UserAgent:   event.UserAgent,
				Referrer:    event.Referrer,
				Country:     geoResult.Country,
				City:        geoResult.City,
				DeviceType:  uaResult.DeviceType,
				Browser:     uaResult.Browser,
				UTMSource:   event.UTMSource,
				UTMMedium:   event.UTMMedium,
				UTMCampaign: event.UTMCampaign,
			}
			clicks = append(clicks, click)
		}

		if err := f.clickRepo.BatchInsert(ctx, clicks); err != nil {
			logger.Log.Errorf("Failed to batch insert clicks: %v", err)
			return
		}

		if err := f.rdb.LTrim(ctx, bufferKey, int64(len(events)), -1).Err(); err != nil {
			logger.Log.Errorf("Failed to trim click buffer: %v", err)
		}

		logger.Log.Infof("Flushed %d click events to database", len(clicks))

		if len(events) < f.batchSize {
			return
		}
	}
}
```

**Step 2: Verify build passes**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/internal/worker/click_flusher.go
git commit -m "feat(backend): integrate UA parsing and GeoIP in click flusher"
```

---

## Task 6: Add Location Stats to Repository

**Files:**
- Modify: `backend/internal/repository/interfaces.go`
- Modify: `backend/internal/repository/click_repo.go`

**Step 1: Add location stats types and interface methods**

Add to `backend/internal/repository/interfaces.go`:

```go
type CountryStats struct {
	Country    string  `db:"country" json:"code"`
	CountryName string `json:"name"`
	Count      int64   `db:"count" json:"clicks"`
	Percentage float64 `json:"percentage"`
}

type CityStats struct {
	City       string  `db:"city" json:"name"`
	Country    string  `db:"country" json:"country"`
	Count      int64   `db:"count" json:"clicks"`
	Percentage float64 `json:"percentage"`
}
```

Add to ClickRepository interface:
```go
GetCountryStats(ctx context.Context, linkID uint64, limit int) ([]CountryStats, error)
GetCityStats(ctx context.Context, linkID uint64, limit int) ([]CityStats, error)
```

**Step 2: Implement in click_repo.go**

Add to `backend/internal/repository/click_repo.go`:

```go
func (r *ClickRepositoryImpl) GetCountryStats(ctx context.Context, linkID uint64, limit int) ([]CountryStats, error) {
	var stats []CountryStats
	query := `SELECT COALESCE(NULLIF(country, ''), 'Unknown') as country, COUNT(*) as count
			  FROM clicks WHERE link_id = ? GROUP BY country ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	return stats, err
}

func (r *ClickRepositoryImpl) GetCityStats(ctx context.Context, linkID uint64, limit int) ([]CityStats, error) {
	var stats []CityStats
	query := `SELECT COALESCE(NULLIF(city, ''), 'Unknown') as city,
			  COALESCE(NULLIF(country, ''), 'Unknown') as country, COUNT(*) as count
			  FROM clicks WHERE link_id = ? GROUP BY city, country ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	return stats, err
}
```

**Step 3: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/internal/repository/interfaces.go backend/internal/repository/click_repo.go
git commit -m "feat(backend): add location stats repository methods"
```

---

## Task 7: Update Stats Service with Locations

**Files:**
- Modify: `backend/internal/service/stats_service.go`

**Step 1: Update LinkStatsResponse and GetLinkStats**

Add Locations to response in `backend/internal/service/stats_service.go`:

```go
type LocationStats struct {
	Countries []repository.CountryStats `json:"countries"`
	Cities    []repository.CityStats    `json:"cities"`
}

type LinkStatsResponse struct {
	TotalClicks    int64                        `json:"total_clicks"`
	UniqueVisitors int64                        `json:"unique_visitors"`
	DailyStats     []repository.DailyClickStats `json:"daily_stats"`
	TopReferrers   []repository.ReferrerStats   `json:"top_referrers"`
	DeviceStats    []repository.DeviceStats     `json:"device_stats"`
	BrowserStats   []repository.BrowserStats    `json:"browser_stats"`
	Locations      LocationStats                `json:"locations"`
}
```

Update GetLinkStats method to fetch and calculate location stats with percentages:

```go
// Add after browser stats fetch:
countries, err := s.clickRepo.GetCountryStats(ctx, linkID, 10)
if err != nil {
	return nil, err
}
if countries == nil {
	countries = []repository.CountryStats{}
}
// Calculate percentages
for i := range countries {
	if stats.TotalClicks > 0 {
		countries[i].Percentage = float64(countries[i].Count) / float64(stats.TotalClicks) * 100
	}
	countries[i].CountryName = getCountryName(countries[i].Country)
}

cities, err := s.clickRepo.GetCityStats(ctx, linkID, 10)
if err != nil {
	return nil, err
}
if cities == nil {
	cities = []repository.CityStats{}
}
// Calculate percentages
for i := range cities {
	if stats.TotalClicks > 0 {
		cities[i].Percentage = float64(cities[i].Count) / float64(stats.TotalClicks) * 100
	}
}

// Add to return:
Locations: LocationStats{
	Countries: countries,
	Cities:    cities,
},
```

Add helper function:
```go
func getCountryName(code string) string {
	names := map[string]string{
		"CN": "China", "US": "United States", "JP": "Japan", "GB": "United Kingdom",
		"DE": "Germany", "FR": "France", "KR": "South Korea", "IN": "India",
		"BR": "Brazil", "RU": "Russia", "Unknown": "Unknown",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add backend/internal/service/stats_service.go
git commit -m "feat(backend): add location stats to stats API response"
```

---

## Task 8: Install Recharts in Frontend

**Files:**
- Modify: `frontend/package.json`

**Step 1: Install recharts**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/frontend && npm install recharts`

**Step 2: Verify installation**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/frontend && npm run build`
Expected: Build succeeds

**Step 3: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/package.json frontend/package-lock.json
git commit -m "feat(frontend): add recharts dependency"
```

---

## Task 9: Create DonutChart Component

**Files:**
- Create: `frontend/src/components/charts/DonutChart.tsx`

**Step 1: Create the component**

Create `frontend/src/components/charts/DonutChart.tsx`:

```tsx
'use client';

import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts';

interface DonutChartProps {
  data: Array<{
    name: string;
    value: number;
    percentage?: number;
  }>;
  colors?: string[];
}

const DEFAULT_COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884d8', '#82ca9d'];

export function DonutChart({ data, colors = DEFAULT_COLORS }: DonutChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-gray-400">
        No data available
      </div>
    );
  }

  return (
    <div className="flex items-center gap-4">
      <div className="w-[150px] h-[150px]">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={data}
              cx="50%"
              cy="50%"
              innerRadius={40}
              outerRadius={70}
              paddingAngle={2}
              dataKey="value"
            >
              {data.map((_, index) => (
                <Cell key={`cell-${index}`} fill={colors[index % colors.length]} />
              ))}
            </Pie>
            <Tooltip
              formatter={(value: number) => [value, 'Clicks']}
            />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="flex-1 space-y-2">
        {data.map((item, index) => (
          <div key={item.name} className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2">
              <div
                className="w-3 h-3 rounded-full"
                style={{ backgroundColor: colors[index % colors.length] }}
              />
              <span className="truncate max-w-[100px]">{item.name}</span>
            </div>
            <span className="text-gray-500">
              {item.percentage !== undefined ? `${item.percentage.toFixed(0)}%` : item.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
```

**Step 2: Verify build**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/frontend && npm run build`

**Step 3: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/components/charts/DonutChart.tsx
git commit -m "feat(frontend): add DonutChart component"
```

---

## Task 10: Create EngagementsChart Component

**Files:**
- Create: `frontend/src/components/charts/EngagementsChart.tsx`

**Step 1: Create the component**

Create `frontend/src/components/charts/EngagementsChart.tsx`:

```tsx
'use client';

import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

interface EngagementsChartProps {
  data: Array<{
    date: string;
    clicks: number;
  }>;
}

export function EngagementsChart({ data }: EngagementsChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-gray-400">
        No engagement data yet
      </div>
    );
  }

  // Format date for display
  const formattedData = data.map(item => ({
    ...item,
    displayDate: new Date(item.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
  })).reverse(); // Show oldest to newest

  return (
    <ResponsiveContainer width="100%" height={200}>
      <BarChart data={formattedData}>
        <XAxis
          dataKey="displayDate"
          tick={{ fontSize: 12 }}
          interval="preserveStartEnd"
        />
        <YAxis
          tick={{ fontSize: 12 }}
          allowDecimals={false}
        />
        <Tooltip
          labelFormatter={(label) => `Date: ${label}`}
          formatter={(value: number) => [value, 'Clicks']}
        />
        <Bar dataKey="clicks" fill="#14b8a6" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/components/charts/EngagementsChart.tsx
git commit -m "feat(frontend): add EngagementsChart component"
```

---

## Task 11: Create LocationsChart Component

**Files:**
- Create: `frontend/src/components/charts/LocationsChart.tsx`

**Step 1: Create the component**

Create `frontend/src/components/charts/LocationsChart.tsx`:

```tsx
'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';

interface LocationsChartProps {
  countries: Array<{
    code: string;
    name: string;
    clicks: number;
    percentage: number;
  }>;
  cities: Array<{
    name: string;
    country: string;
    clicks: number;
    percentage: number;
  }>;
}

export function LocationsChart({ countries, cities }: LocationsChartProps) {
  const [view, setView] = useState<'countries' | 'cities'>('countries');

  const data = view === 'countries' ? countries : cities;
  const maxCount = Math.max(...data.map(d => d.clicks), 1);

  if (data.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-gray-400">
        No location data yet
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <Button
          variant={view === 'countries' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setView('countries')}
        >
          Countries
        </Button>
        <Button
          variant={view === 'cities' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setView('cities')}
        >
          Cities
        </Button>
      </div>
      <div className="space-y-3">
        {data.map((item, index) => (
          <div key={view === 'countries' ? (item as any).code : `${item.name}-${(item as any).country}`} className="space-y-1">
            <div className="flex justify-between text-sm">
              <span className="flex items-center gap-2">
                <span className="text-gray-500 w-4">{index + 1}.</span>
                {view === 'countries' ? (item as any).name : `${item.name}, ${(item as any).country}`}
              </span>
              <span className="text-gray-500">
                {item.clicks} ({item.percentage.toFixed(0)}%)
              </span>
            </div>
            <div className="w-full bg-gray-100 rounded-full h-2">
              <div
                className="bg-teal-500 h-2 rounded-full transition-all"
                style={{ width: `${(item.clicks / maxCount) * 100}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/components/charts/LocationsChart.tsx
git commit -m "feat(frontend): add LocationsChart component"
```

---

## Task 12: Update Frontend Types for Locations

**Files:**
- Modify: `frontend/src/types/index.ts`

**Step 1: Add location types**

Add to `frontend/src/types/index.ts`:

```typescript
export interface CountryStats {
  code: string;
  name: string;
  clicks: number;
  percentage: number;
}

export interface CityStats {
  name: string;
  country: string;
  clicks: number;
  percentage: number;
}

export interface LocationStats {
  countries: CountryStats[];
  cities: CityStats[];
}
```

Update LinkStats interface:
```typescript
export interface LinkStats {
  total_clicks: number;
  unique_visitors: number;
  daily_stats: DailyStats[];
  top_referrers: ReferrerStats[];
  device_stats: DeviceStats[];
  browser_stats: BrowserStats[];
  locations: LocationStats;
}
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/types/index.ts
git commit -m "feat(frontend): add location stats types"
```

---

## Task 13: Redesign Stats Page

**Files:**
- Modify: `frontend/src/app/dashboard/links/[id]/stats/page.tsx`

**Step 1: Rewrite the stats page**

Replace `frontend/src/app/dashboard/links/[id]/stats/page.tsx`:

```tsx
'use client';

import { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { ArrowLeft, Copy, Check } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '@/lib/api';
import { DonutChart } from '@/components/charts/DonutChart';
import { EngagementsChart } from '@/components/charts/EngagementsChart';
import { LocationsChart } from '@/components/charts/LocationsChart';
import type { LinkStats, Link as LinkType } from '@/types';

export default function LinkStatsPage() {
  const params = useParams();
  const linkId = Number(params.id);
  const [stats, setStats] = useState<LinkStats | null>(null);
  const [link, setLink] = useState<LinkType | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

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

  const copyToClipboard = async () => {
    if (!link) return;
    await navigator.clipboard.writeText(link.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (loading) return <div className="p-8">Loading statistics...</div>;
  if (error) return <div className="p-8 text-red-500">{error}</div>;
  if (!stats || !link) return <div className="p-8">No data available</div>;

  const deviceData = stats.device_stats.map(d => ({
    name: d.device_type.charAt(0).toUpperCase() + d.device_type.slice(1),
    value: d.count,
    percentage: stats.total_clicks > 0 ? (d.count / stats.total_clicks) * 100 : 0,
  }));

  const browserData = stats.browser_stats.map(b => ({
    name: b.browser,
    value: b.count,
    percentage: stats.total_clicks > 0 ? (b.count / stats.total_clicks) * 100 : 0,
  }));

  const referrerData = stats.top_referrers.map(r => ({
    name: r.referrer,
    value: r.count,
    percentage: stats.total_clicks > 0 ? (r.count / stats.total_clicks) * 100 : 0,
  }));

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link href="/dashboard/links" className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700">
        <ArrowLeft className="w-4 h-4 mr-1" />
        Back to list
      </Link>

      {/* Link Info Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex justify-between items-start">
            <div className="space-y-2">
              <h1 className="text-2xl font-bold">{link.title || 'Untitled Link'}</h1>
              <div className="flex items-center gap-2">
                <a href={link.short_url} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">
                  {link.short_url.replace('http://', '').replace('https://', '')}
                </a>
                <Button variant="ghost" size="sm" onClick={copyToClipboard}>
                  {copied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </Button>
              </div>
              <p className="text-sm text-gray-500 truncate max-w-lg">{link.original_url}</p>
              <p className="text-sm text-gray-400">
                {new Date(link.created_at).toLocaleDateString('en-US', {
                  year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit'
                })}
              </p>
            </div>
            <div className="text-right">
              <div className="text-3xl font-bold">{stats.total_clicks}</div>
              <div className="text-sm text-gray-500">Total Clicks</div>
              <div className="text-lg font-medium text-gray-600 mt-2">{stats.unique_visitors}</div>
              <div className="text-sm text-gray-500">Unique Visitors</div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Engagements Over Time */}
      <Card>
        <CardHeader>
          <CardTitle>Engagements over time</CardTitle>
        </CardHeader>
        <CardContent>
          <EngagementsChart data={stats.daily_stats} />
        </CardContent>
      </Card>

      {/* Locations */}
      <Card>
        <CardHeader>
          <CardTitle>Locations</CardTitle>
        </CardHeader>
        <CardContent>
          <LocationsChart
            countries={stats.locations?.countries || []}
            cities={stats.locations?.cities || []}
          />
        </CardContent>
      </Card>

      {/* Referrers and Devices */}
      <div className="grid grid-cols-2 gap-4">
        <Card>
          <CardHeader>
            <CardTitle>Referrers</CardTitle>
          </CardHeader>
          <CardContent>
            <DonutChart data={referrerData} colors={['#f97316', '#14b8a6', '#3b82f6', '#8b5cf6', '#ec4899']} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Devices</CardTitle>
          </CardHeader>
          <CardContent>
            <DonutChart data={deviceData} colors={['#14b8a6', '#f97316', '#3b82f6']} />
          </CardContent>
        </Card>
      </div>

      {/* Browsers */}
      <Card>
        <CardHeader>
          <CardTitle>Browsers</CardTitle>
        </CardHeader>
        <CardContent>
          <DonutChart data={browserData} colors={['#3b82f6', '#14b8a6', '#f97316', '#8b5cf6', '#ec4899', '#10b981']} />
        </CardContent>
      </Card>
    </div>
  );
}
```

**Step 2: Verify build**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/frontend && npm run build`

**Step 3: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/app/dashboard/links/[id]/stats/page.tsx
git commit -m "feat(frontend): redesign stats page with Bitly-style charts"
```

---

## Task 14: Create LinkCard Component

**Files:**
- Create: `frontend/src/components/links/LinkCard.tsx`

**Step 1: Create the component**

Create `frontend/src/components/links/LinkCard.tsx`:

```tsx
'use client';

import { useState } from 'react';
import Link from 'next/link';
import { Copy, Check, Pencil, Share2, BarChart3, MoreHorizontal, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { Link as LinkType } from '@/types';

interface LinkCardProps {
  link: LinkType;
  selected: boolean;
  onSelect: (id: number, selected: boolean) => void;
  onDelete: (id: number) => void;
}

export function LinkCard({ link, selected, onSelect, onDelete }: LinkCardProps) {
  const [copied, setCopied] = useState(false);

  const copyToClipboard = async () => {
    await navigator.clipboard.writeText(link.short_url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className={`border rounded-lg p-4 bg-white hover:shadow-sm transition-shadow ${selected ? 'ring-2 ring-orange-500' : ''}`}>
      <div className="flex items-start gap-3">
        <Checkbox
          checked={selected}
          onCheckedChange={(checked) => onSelect(link.id, checked as boolean)}
          className="mt-1"
        />

        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between">
            <h3 className="font-semibold text-lg truncate">
              {link.title || 'Untitled'}
            </h3>
            <div className="flex items-center gap-1">
              <Button variant="ghost" size="sm" asChild>
                <Link href={`/dashboard/links/${link.id}/edit`}>
                  <Pencil className="w-4 h-4" />
                </Link>
              </Button>
              <Button variant="ghost" size="sm">
                <Share2 className="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="sm" asChild>
                <Link href={`/dashboard/links/${link.id}/stats`}>
                  <BarChart3 className="w-4 h-4" />
                </Link>
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="sm">
                    <MoreHorizontal className="w-4 h-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem
                    className="text-red-600"
                    onClick={() => onDelete(link.id)}
                  >
                    <Trash2 className="w-4 h-4 mr-2" />
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          <div className="flex items-center gap-2 mt-1">
            <a
              href={link.short_url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:underline text-sm"
            >
              {link.short_url.replace('http://', '').replace('https://', '')}
            </a>
            <Button variant="ghost" size="sm" className="h-6 w-6 p-0" onClick={copyToClipboard}>
              {copied ? <Check className="w-3 h-3 text-green-500" /> : <Copy className="w-3 h-3" />}
            </Button>
          </div>

          <p className="text-sm text-gray-500 truncate mt-1">
            {link.original_url}
          </p>

          <div className="flex items-center gap-4 mt-3 text-xs text-gray-400">
            <span>{new Date(link.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}</span>
            <span className={`px-2 py-0.5 rounded-full ${link.is_active ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
              {link.is_active ? 'Active' : 'Inactive'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/components/links/LinkCard.tsx
git commit -m "feat(frontend): add LinkCard component for Bitly-style list"
```

---

## Task 15: Create LinksToolbar Component

**Files:**
- Create: `frontend/src/components/links/LinksToolbar.tsx`

**Step 1: Create the component**

Create `frontend/src/components/links/LinksToolbar.tsx`:

```tsx
'use client';

import { Search, Calendar, Filter, List, LayoutGrid, Rows3 } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

export type ViewMode = 'list' | 'compact' | 'grid';
export type StatusFilter = 'all' | 'active' | 'inactive';

interface LinksToolbarProps {
  search: string;
  onSearchChange: (value: string) => void;
  status: StatusFilter;
  onStatusChange: (value: StatusFilter) => void;
  viewMode: ViewMode;
  onViewModeChange: (mode: ViewMode) => void;
  selectedCount: number;
  onBulkDelete: () => void;
}

export function LinksToolbar({
  search,
  onSearchChange,
  status,
  onStatusChange,
  viewMode,
  onViewModeChange,
  selectedCount,
  onBulkDelete,
}: LinksToolbarProps) {
  return (
    <div className="space-y-3">
      {/* Search and Filters */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
          <Input
            placeholder="Search links"
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="pl-10"
          />
        </div>
        <Button variant="outline" size="sm">
          <Calendar className="w-4 h-4 mr-2" />
          Filter by date
        </Button>
        <Button variant="outline" size="sm">
          <Filter className="w-4 h-4 mr-2" />
          Add filters
        </Button>
      </div>

      {/* Bulk Actions and View Toggle */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-sm text-gray-500">
            {selectedCount} selected
          </span>
          {selectedCount > 0 && (
            <>
              <Button variant="ghost" size="sm">Export</Button>
              <Button variant="ghost" size="sm">Hide</Button>
              <Button variant="ghost" size="sm" onClick={onBulkDelete} className="text-red-600">
                Delete
              </Button>
            </>
          )}
        </div>
        <div className="flex items-center gap-2">
          <div className="flex border rounded-md">
            <Button
              variant={viewMode === 'list' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-r-none"
              onClick={() => onViewModeChange('list')}
            >
              <List className="w-4 h-4" />
            </Button>
            <Button
              variant={viewMode === 'compact' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-none border-x"
              onClick={() => onViewModeChange('compact')}
            >
              <Rows3 className="w-4 h-4" />
            </Button>
            <Button
              variant={viewMode === 'grid' ? 'secondary' : 'ghost'}
              size="sm"
              className="rounded-l-none"
              onClick={() => onViewModeChange('grid')}
            >
              <LayoutGrid className="w-4 h-4" />
            </Button>
          </div>
          <Select value={status} onValueChange={(v) => onStatusChange(v as StatusFilter)}>
            <SelectTrigger className="w-[130px]">
              <SelectValue placeholder="Show: All" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Show: All</SelectItem>
              <SelectItem value="active">Show: Active</SelectItem>
              <SelectItem value="inactive">Show: Inactive</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/components/links/LinksToolbar.tsx
git commit -m "feat(frontend): add LinksToolbar component"
```

---

## Task 16: Redesign Links List Page

**Files:**
- Modify: `frontend/src/app/dashboard/links/page.tsx`

**Step 1: First read current page structure**

Check if file exists at `frontend/src/app/dashboard/links/page.tsx`

**Step 2: Create/update the links page**

Create `frontend/src/app/dashboard/links/page.tsx`:

```tsx
'use client';

import { useState, useEffect, useMemo } from 'react';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { api } from '@/lib/api';
import { LinkCard } from '@/components/links/LinkCard';
import { LinksToolbar, ViewMode, StatusFilter } from '@/components/links/LinksToolbar';
import type { Link as LinkType } from '@/types';

export default function LinksPage() {
  const [links, setLinks] = useState<LinkType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [viewMode, setViewMode] = useState<ViewMode>('list');
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());

  useEffect(() => {
    loadLinks();
  }, []);

  const loadLinks = async () => {
    try {
      setError(null);
      const data = await api.getLinks();
      setLinks(data.links || []);
    } catch (err) {
      setError('Failed to load links. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const filteredLinks = useMemo(() => {
    return links.filter(link => {
      // Status filter
      if (status === 'active' && !link.is_active) return false;
      if (status === 'inactive' && link.is_active) return false;

      // Search filter
      if (search) {
        const searchLower = search.toLowerCase();
        return (
          link.short_code.toLowerCase().includes(searchLower) ||
          link.original_url.toLowerCase().includes(searchLower) ||
          (link.title?.toLowerCase().includes(searchLower) ?? false)
        );
      }

      return true;
    });
  }, [links, search, status]);

  const handleSelect = (id: number, selected: boolean) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (selected) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this link?')) return;
    try {
      await api.deleteLink(id);
      setLinks(links.filter(l => l.id !== id));
      setSelectedIds(prev => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    } catch {
      alert('Failed to delete link');
    }
  };

  const handleBulkDelete = async () => {
    if (selectedIds.size === 0) return;
    if (!confirm(`Are you sure you want to delete ${selectedIds.size} links?`)) return;

    try {
      await Promise.all(Array.from(selectedIds).map(id => api.deleteLink(id)));
      setLinks(links.filter(l => !selectedIds.has(l.id)));
      setSelectedIds(new Set());
    } catch {
      alert('Failed to delete some links');
      loadLinks();
    }
  };

  if (loading) {
    return <div className="p-8">Loading links...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Your Links</h1>
        <Button asChild>
          <Link href="/dashboard/links/create">
            <Plus className="w-4 h-4 mr-2" />
            Create link
          </Link>
        </Button>
      </div>

      {error ? (
        <div className="text-center py-8 text-red-500">
          {error}
          <Button variant="ghost" onClick={loadLinks}>Retry</Button>
        </div>
      ) : (
        <>
          <LinksToolbar
            search={search}
            onSearchChange={setSearch}
            status={status}
            onStatusChange={setStatus}
            viewMode={viewMode}
            onViewModeChange={setViewMode}
            selectedCount={selectedIds.size}
            onBulkDelete={handleBulkDelete}
          />

          {filteredLinks.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              {links.length === 0
                ? "No links yet. Create your first short link!"
                : "No links match your filters."}
            </div>
          ) : (
            <div className={
              viewMode === 'grid'
                ? 'grid grid-cols-2 gap-4'
                : 'space-y-3'
            }>
              {filteredLinks.map(link => (
                <LinkCard
                  key={link.id}
                  link={link}
                  selected={selectedIds.has(link.id)}
                  onSelect={handleSelect}
                  onDelete={handleDelete}
                />
              ))}
            </div>
          )}

          {filteredLinks.length > 0 && (
            <div className="text-center text-sm text-gray-400 py-4">
              You've reached the end of your links
            </div>
          )}
        </>
      )}
    </div>
  );
}
```

**Step 3: Verify build**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/frontend && npm run build`

**Step 4: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/app/dashboard/links/page.tsx
git commit -m "feat(frontend): redesign links list page with Bitly-style cards"
```

---

## Task 17: Add Charts Index Export

**Files:**
- Create: `frontend/src/components/charts/index.ts`

**Step 1: Create index file**

Create `frontend/src/components/charts/index.ts`:

```typescript
export { DonutChart } from './DonutChart';
export { EngagementsChart } from './EngagementsChart';
export { LocationsChart } from './LocationsChart';
```

**Step 2: Commit**

```bash
cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign
git add frontend/src/components/charts/index.ts
git commit -m "feat(frontend): add charts index export"
```

---

## Task 18: Final Build Verification

**Step 1: Build backend**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go build ./...`
Expected: No errors

**Step 2: Build frontend**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/frontend && npm run build`
Expected: Build succeeds

**Step 3: Run backend tests**

Run: `cd /Users/jose/GolandProjects/urlshortener/.worktrees/stats-links-redesign/backend && go test ./internal/util/... -v`
Expected: All tests pass

**Step 4: Final commit if any fixes needed**

If any fixes were required, commit them with appropriate message.

---

## Summary

This plan implements:

1. **Backend Bug Fix**: User-Agent parsing for device/browser detection
2. **Backend Enhancement**: GeoIP lookup for location stats
3. **Backend Enhancement**: Store real IP address
4. **Backend API**: Add location stats to stats endpoint
5. **Frontend**: Recharts integration with DonutChart, EngagementsChart, LocationsChart
6. **Frontend**: Redesigned Stats page matching Bitly style
7. **Frontend**: Redesigned Links list with cards, search, filters, bulk actions

Total: 18 tasks

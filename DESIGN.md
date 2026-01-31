# URL Shortener Design Document

## Project Overview

A full-featured, open-source, self-hosted URL shortener similar to Bitly.

### Tech Stack

**Frontend:**
- React + TypeScript
- Next.js
- Tailwind CSS
- shadcn/ui
- Zustand (state management)

**Backend:**
- Go + Gin (HTTP framework)
- Zap (logging)
- MariaDB (primary database)
- Redis (caching & real-time data)

**Deployment:**
- Docker Compose (all services in one file)

### Core Features

- Short URL creation (auto-generated + custom aliases)
- User accounts (email/password authentication)
- API access via API keys
- QR code generation
- Link editing and bulk operations
- Advanced analytics (unique visitors, real-time stats, geo heatmaps, UTM tracking)
- Custom domain support
- Optional link expiration (permanent by default)

---

## Section 1: System Architecture Overview

### Two-Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Docker Compose                          │
├─────────────┬─────────────┬─────────────┬─────────────────┤
│  Frontend   │   Backend   │   MariaDB   │      Redis      │
│  (Next.js)  │    (Gin)    │             │                 │
│  Port 3000  │  Port 8080  │  Port 3306  │   Port 6379     │
└─────────────┴─────────────┴─────────────┴─────────────────┘
```

### Frontend (Next.js)

- Dashboard UI for link management
- Analytics visualization
- User authentication pages
- QR code generation (client-side)
- Zustand for state management

### Backend (Gin)

- REST API for all operations
- Redirect service (the core `/:code` endpoint)
- Click tracking and analytics processing
- Background jobs for geo-IP lookups
- Custom domain validation

### Data Stores

- **MariaDB**: Users, links, domains, analytics aggregates
- **Redis**: Session cache, rate limiting, real-time click counters, short code lookups for fast redirects

### Key Design Decision

The redirect endpoint (`GET /:code`) hits Redis first for speed. If cache miss, query MariaDB, then cache. Click events are buffered in Redis and flushed to MariaDB periodically for analytics.

---

## Section 2: Database Schema

### MariaDB Tables

```sql
-- 用户表
CREATE TABLE users (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 短链接表
CREATE TABLE links (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    short_code      VARCHAR(16) NOT NULL UNIQUE,
    original_url    TEXT NOT NULL,
    title           VARCHAR(255),
    expires_at      TIMESTAMP NULL,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_short_code (short_code),
    INDEX idx_user_id (user_id)
);

-- 自定义域名表
CREATE TABLE domains (
    id                  BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id             BIGINT UNSIGNED NOT NULL,
    domain              VARCHAR(255) NOT NULL UNIQUE,
    verified            BOOLEAN DEFAULT FALSE,
    verification_token  VARCHAR(64) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- API密钥表
CREATE TABLE api_keys (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    key_hash        VARCHAR(64) NOT NULL UNIQUE,
    name            VARCHAR(100) NOT NULL,
    last_used_at    TIMESTAMP NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 点击事件表 (详细记录)
CREATE TABLE clicks (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    link_id         BIGINT UNSIGNED NOT NULL,
    clicked_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_hash         VARCHAR(64),
    user_agent      VARCHAR(512),
    referrer        VARCHAR(2048),
    country         VARCHAR(2),
    city            VARCHAR(100),
    device_type     ENUM('desktop', 'mobile', 'tablet', 'unknown') DEFAULT 'unknown',
    browser         VARCHAR(50),
    utm_source      VARCHAR(255),
    utm_medium      VARCHAR(255),
    utm_campaign    VARCHAR(255),
    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE,
    INDEX idx_link_clicked (link_id, clicked_at)
);

-- 每日统计聚合表 (快速查询)
CREATE TABLE link_stats_daily (
    link_id         BIGINT UNSIGNED NOT NULL,
    date            DATE NOT NULL,
    total_clicks    INT UNSIGNED DEFAULT 0,
    unique_visitors INT UNSIGNED DEFAULT 0,
    PRIMARY KEY (link_id, date),
    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);
```

### Redis Data Structures

| Key Pattern | Type | Purpose |
|-------------|------|---------|
| `link:{code}` | String | Original URL for fast redirect |
| `link:{code}:meta` | Hash | Link metadata (user_id, expires_at, is_active) |
| `clicks:{link_id}:count` | Counter | Real-time click count |
| `clicks:{link_id}:buffer` | List | Click events buffer (flush to MariaDB periodically) |
| `rate:{ip}` | Counter with TTL | Rate limiting per IP |
| `session:{token}` | Hash | User session data |

---

## Section 3: API Design

### Authentication

- **Web UI**: JWT Token stored in httpOnly cookie
- **API Access**: `X-API-Key` header

### API Endpoints

#### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/logout` | Logout |
| GET | `/api/auth/me` | Get current user info |

#### Link Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/links` | List user's links (paginated) |
| POST | `/api/links` | Create short link |
| GET | `/api/links/:id` | Get link details |
| PUT | `/api/links/:id` | Update link |
| DELETE | `/api/links/:id` | Delete link |
| POST | `/api/links/bulk` | Bulk create links |

#### Analytics

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/links/:id/stats` | Overall statistics |
| GET | `/api/links/:id/stats/realtime` | Real-time click count |
| GET | `/api/links/:id/stats/clicks` | Click details (paginated) |
| GET | `/api/links/:id/stats/geo` | Geographic distribution |
| GET | `/api/links/:id/stats/devices` | Device distribution |
| GET | `/api/links/:id/stats/referrers` | Referrer distribution |

#### Domain Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/domains` | List user's domains |
| POST | `/api/domains` | Add custom domain |
| DELETE | `/api/domains/:id` | Remove domain |
| POST | `/api/domains/:id/verify` | Verify domain ownership |

#### API Keys

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/keys` | List API keys |
| POST | `/api/keys` | Create API key |
| DELETE | `/api/keys/:id` | Delete API key |

#### Redirect (Separate Route)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/:code` | Redirect to original URL |

---

## Section 4: Frontend Structure

### Page Routes

| Route | Description |
|-------|-------------|
| `/` | Landing page (guest) / Redirect to dashboard (logged in) |
| `/login` | Login page |
| `/register` | Registration page |
| `/dashboard` | Dashboard home (link list + quick create) |
| `/dashboard/links` | Link management |
| `/dashboard/links/new` | Create new link |
| `/dashboard/links/:id` | Link details + analytics |
| `/dashboard/bulk` | Bulk link creation |
| `/dashboard/domains` | Custom domain management |
| `/dashboard/settings` | Account settings |
| `/dashboard/api-keys` | API key management |

### Component Structure

```
components/
├── ui/                  # shadcn/ui components
├── layout/
│   ├── Header          # Top navigation
│   ├── Sidebar         # Dashboard sidebar
│   └── Footer
├── links/
│   ├── LinkCard        # Link card component
│   ├── LinkForm        # Create/edit form
│   ├── LinkTable       # Link list table
│   └── QRCodeModal     # QR code modal
├── analytics/
│   ├── StatsOverview   # Statistics overview
│   ├── ClickChart      # Click trend chart
│   ├── GeoMap          # Geographic distribution map
│   ├── DevicePieChart  # Device distribution pie chart
│   └── ReferrerList    # Referrer list
└── domains/
    ├── DomainList      # Domain list
    └── DomainVerify    # Domain verification guide
```

### Zustand Stores

```
stores/
├── authStore           # User authentication state
├── linksStore          # Link list + pagination
└── statsStore          # Current viewing statistics
```

---

## Section 5: Backend Project Structure

### Go Project Layout

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Config loading (env/yaml)
│   ├── handler/
│   │   ├── auth.go              # Authentication handlers
│   │   ├── link.go              # Link management
│   │   ├── stats.go             # Analytics
│   │   ├── domain.go            # Domain management
│   │   ├── apikey.go            # API keys
│   │   └── redirect.go          # Redirect handler
│   ├── middleware/
│   │   ├── auth.go              # JWT validation
│   │   ├── apikey.go            # API Key validation
│   │   ├── ratelimit.go         # Rate limiting
│   │   └── cors.go              # CORS handling
│   ├── model/
│   │   ├── user.go
│   │   ├── link.go
│   │   ├── click.go
│   │   ├── domain.go
│   │   └── apikey.go
│   ├── repository/
│   │   ├── user_repo.go
│   │   ├── link_repo.go
│   │   ├── click_repo.go
│   │   └── domain_repo.go
│   ├── service/
│   │   ├── auth_service.go
│   │   ├── link_service.go
│   │   ├── stats_service.go
│   │   ├── shortcode_service.go # Short code generation
│   │   └── geoip_service.go     # IP geolocation
│   └── worker/
│       └── click_flusher.go     # Background: flush clicks to DB
├── pkg/
│   └── logger/
│       └── zap.go               # Zap logger wrapper
├── migrations/                   # Database migrations
├── Dockerfile
└── go.mod
```

### Key Design Principles

- **Layered Architecture**: Handler → Service → Repository
- **Dependency Injection**: Via constructors
- **Repository Pattern**: Isolate database operations

---

## Section 6: Core Flow - Redirect & Click Tracking

### Redirect Flow (Performance Critical Path)

```
GET /:code
    │
    ▼
┌─────────────────────┐
│ 1. Query Redis      │
│    link:{code}      │
└─────────────────────┘
    │
    ├── HIT ───────────────────────────────────┐
    │                                          │
    ▼ MISS                                     │
┌─────────────────────┐                        │
│ 2. Query MariaDB    │                        │
│    SELECT * FROM    │                        │
│    links WHERE      │                        │
│    short_code=?     │                        │
└─────────────────────┘                        │
    │                                          │
    ├── NOT FOUND → Return 404                 │
    │                                          │
    ▼ FOUND                                    │
┌─────────────────────┐                        │
│ 3. Cache in Redis   │                        │
│    (TTL: 1 hour)    │                        │
└─────────────────────┘                        │
    │                                          │
    ▼◄─────────────────────────────────────────┘
┌─────────────────────┐
│ 4. Check link state │
│    - is_active?     │
│    - expires_at?    │
└─────────────────────┘
    │
    ├── INVALID/EXPIRED → Return 410 Gone
    │
    ▼ VALID
┌─────────────────────┐
│ 5. Async log click  │
│    LPUSH clicks:    │
│    {link_id}:buffer │
└─────────────────────┘
    │
    ▼
┌─────────────────────┐
│ 6. 302 Redirect     │
│    → original_url   │
└─────────────────────┘
```

### Click Data Background Flush (Every 30 seconds)

```
Worker: click_flusher
    │
    ▼
┌─────────────────────┐
│ 1. LRANGE + DEL     │
│    Get buffer data  │
└─────────────────────┘
    │
    ▼
┌─────────────────────┐
│ 2. Parse User-Agent │
│    Extract device/  │
│    browser info     │
└─────────────────────┘
    │
    ▼
┌─────────────────────┐
│ 3. GeoIP Lookup     │
│    IP → Country/City│
└─────────────────────┘
    │
    ▼
┌─────────────────────┐
│ 4. Batch insert to  │
│    clicks table +   │
│    update aggregates│
└─────────────────────┘
```

---

## Section 7: Custom Domain Verification

### Domain Verification Flow

```
1. User adds domain (e.g., links.example.com)
   │
   ▼
2. System generates verification token
   Verification methods (choose one):

   Method A - DNS TXT Record:
   ┌────────────────────────────────────────┐
   │ _urlshortener.links.example.com  TXT  │
   │ "verify=abc123xyz"                     │
   └────────────────────────────────────────┘

   Method B - DNS CNAME Record:
   ┌────────────────────────────────────────┐
   │ links.example.com  CNAME               │
   │ → custom.yourdomain.com                │
   └────────────────────────────────────────┘
   │
   ▼
3. User configures DNS and clicks "Verify"
   │
   ▼
4. Backend queries DNS records for verification
   │
   ├── FAILED → Show error message
   │
   ▼ SUCCESS
5. Mark domain as verified (verified=true)
```

### Custom Domain Redirect Handling

```
Request: GET https://links.example.com/abc123
    │
    ▼
┌─────────────────────┐
│ 1. Check Host header│
│    Is it a custom   │
│    domain?          │
└─────────────────────┘
    │
    ▼
┌─────────────────────┐
│ 2. Query domains    │
│    table, verify    │
│    domain validity  │
└─────────────────────┘
    │
    ├── INVALID → Return 404
    │
    ▼ VALID
┌─────────────────────┐
│ 3. Normal redirect  │
│    flow             │
└─────────────────────┘
```

---

## Section 8: Docker Compose Deployment

### docker-compose.yml

```yaml
version: '3.8'

services:
  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://backend:8080
    depends_on:
      - backend

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=mariadb
      - DB_PORT=3306
      - DB_USER=urlshortener
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=urlshortener
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - JWT_SECRET=${JWT_SECRET}
      - BASE_URL=${BASE_URL}
    depends_on:
      mariadb:
        condition: service_healthy
      redis:
        condition: service_started

  mariadb:
    image: mariadb:10.11
    volumes:
      - mariadb_data:/var/lib/mysql
      - ./backend/migrations:/docker-entrypoint-initdb.d
    environment:
      - MYSQL_ROOT_PASSWORD=${DB_ROOT_PASSWORD}
      - MYSQL_DATABASE=urlshortener
      - MYSQL_USER=urlshortener
      - MYSQL_PASSWORD=${DB_PASSWORD}
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes

volumes:
  mariadb_data:
  redis_data:
```

### Environment Variables (.env.example)

```
DB_PASSWORD=your_secure_password
DB_ROOT_PASSWORD=your_root_password
JWT_SECRET=your_jwt_secret_key
BASE_URL=https://sho.rt
```

---

## Section 9: Complete Project Structure

```
urlshortener/
├── frontend/
│   ├── src/
│   │   ├── app/                 # Next.js App Router
│   │   │   ├── (auth)/
│   │   │   │   ├── login/
│   │   │   │   └── register/
│   │   │   ├── dashboard/
│   │   │   │   ├── links/
│   │   │   │   ├── domains/
│   │   │   │   ├── api-keys/
│   │   │   │   └── settings/
│   │   │   ├── layout.tsx
│   │   │   └── page.tsx
│   │   ├── components/
│   │   ├── stores/
│   │   ├── lib/
│   │   │   ├── api.ts           # API client
│   │   │   └── utils.ts
│   │   └── types/
│   ├── public/
│   ├── tailwind.config.js
│   ├── package.json
│   └── Dockerfile
│
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   ├── pkg/
│   ├── migrations/
│   ├── go.mod
│   └── Dockerfile
│
├── docker-compose.yml
├── .env.example
├── README.md
└── DESIGN.md
```

### Recommended Third-Party Libraries

| Purpose | Frontend | Backend |
|---------|----------|---------|
| HTTP Client | axios / fetch | gin |
| Logging | - | zap |
| Database | - | sqlx / gorm |
| Cache | - | go-redis |
| Password Hashing | - | bcrypt |
| JWT | - | golang-jwt |
| QR Code | qrcode.react | - |
| Charts | recharts | - |
| Maps | react-simple-maps | - |
| GeoIP | - | maxmind/geoip2-golang |
| User-Agent Parsing | - | mssola/useragent |

---

## Summary

This design document outlines a full-featured, open-source, self-hosted URL shortener with:

- **Frontend**: Next.js + React + TypeScript + Tailwind + shadcn/ui + Zustand
- **Backend**: Go + Gin + Zap + MariaDB + Redis
- **Features**: User accounts, custom aliases, API keys, QR codes, advanced analytics, custom domains
- **Deployment**: Docker Compose for easy self-hosting

The architecture prioritizes:
1. **Performance**: Redis caching for fast redirects
2. **Scalability**: Async click processing with background workers
3. **Simplicity**: Single Docker Compose file for deployment
4. **Extensibility**: Clean layered architecture in backend

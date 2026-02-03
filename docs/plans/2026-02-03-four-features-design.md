# Four Features Design

Date: 2026-02-03

## Overview

This document covers four new features for the URL shortener:

1. Registration toggle (enable/disable new user signups)
2. Domain management (users bind custom domains for short URLs)
3. Split ports (separate redirect server from API server)
4. Configurable CORS origins

---

## 1. Registration Toggle

### Config

```yaml
server:
  allow_registration: true  # Set false to disable new user registration
```

### Behavior

- When `allow_registration: false`, `POST /api/auth/register` returns:
  - Status: `403 Forbidden`
  - Body: `{"error": "Registration is disabled"}`
- Existing users can still log in
- Config change requires server restart

### Backend Changes

- Add `AllowRegistration bool` to `ServerConfig` struct
- Check config in `AuthHandler.Register()` before processing

---

## 2. Domain Management

### Database Schema

New `domains` table:

```sql
CREATE TABLE domains (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    domain      VARCHAR(255) NOT NULL UNIQUE,  -- globally unique
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_user_id (user_id)
);
```

Modify `links` table:

```sql
ALTER TABLE links ADD COLUMN domain_id BIGINT UNSIGNED NULL;
ALTER TABLE links ADD FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE SET NULL;
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/domains` | List user's domains |
| POST | `/api/domains` | Add domain `{ "domain": "short.example.com" }` |
| DELETE | `/api/domains/:id` | Remove domain (only owner can delete) |

Error on duplicate domain: `409 Conflict` with `{"error": "Domain already in use"}`

### Link Changes

- `POST /api/links` accepts optional `domain_id`
- `PUT /api/links/:id` accepts optional `domain_id`
- Response includes `domain` object when set:
  ```json
  {
    "id": 1,
    "short_code": "abc123",
    "domain": { "id": 5, "domain": "short.example.com" },
    "short_url": "https://short.example.com/abc123"
  }
  ```
- When `domain_id` is null, `short_url` uses `config.urls.base_url`

### Frontend

New sidebar menu: **Domains** (Globe icon) between Links and Analytics

Pages:
- `/dashboard/domains` - List domains with delete buttons
- `/dashboard/domains/create` - Add domain form

Create/Edit link pages:
- Add "Domain" dropdown
- Options: user's domains + "Default" (uses base_url)
- Default selection: "Default" (null domain_id)

---

## 3. Split Ports

### Config

```yaml
server:
  redirect_port: 8080  # Short URL redirects
  api_port: 8081       # Admin API
```

### Architecture

Two separate Gin routers:

**Redirect Server** (port 8080):
```
GET /:short_code  → Redirect to original URL
GET /health       → Health check
```

**API Server** (port 8081):
```
GET /health       → Health check
/api/*            → All API routes
```

### Implementation

```go
func main() {
    // ... setup code ...

    redirectRouter := gin.New()
    redirectRouter.GET("/:short_code", redirectHandler.Redirect)
    redirectRouter.GET("/health", healthHandler)

    apiRouter := gin.New()
    apiRouter.Use(corsMiddleware)  // CORS only on API
    setupAPIRoutes(apiRouter)

    // Run both concurrently
    go func() {
        log.Fatal(redirectRouter.Run(fmt.Sprintf(":%d", cfg.Server.RedirectPort)))
    }()
    log.Fatal(apiRouter.Run(fmt.Sprintf(":%d", cfg.Server.APIPort)))
}
```

### Docker/Deployment

- Expose both ports in docker-compose
- Frontend `NEXT_PUBLIC_API_URL` points to API port
- Short URLs use redirect port

---

## 4. Configurable CORS Origins

### Config

```yaml
server:
  allow_origins:
    - "http://localhost:3000"
    - "https://app.example.com"
    - "https://*.example.com"  # Wildcard support
```

### Wildcard Matching

- `*.example.com` matches `app.example.com`, `api.example.com`
- Does NOT match `example.com` (no subdomain)
- Does NOT match `sub.app.example.com` (nested subdomain)

### Implementation

```go
func matchOrigin(origin string, patterns []string) bool {
    for _, pattern := range patterns {
        if strings.HasPrefix(pattern, "*.") {
            // Wildcard: *.example.com
            suffix := pattern[1:]  // .example.com
            if strings.HasSuffix(origin, suffix) {
                // Ensure only one level: count dots
                prefix := strings.TrimSuffix(origin, suffix)
                prefix = strings.TrimPrefix(prefix, "https://")
                prefix = strings.TrimPrefix(prefix, "http://")
                if !strings.Contains(prefix, ".") && len(prefix) > 0 {
                    return true
                }
            }
        } else if origin == pattern {
            return true
        }
    }
    return false
}
```

---

## Config File Summary

Complete new `server` section:

```yaml
server:
  redirect_port: 8080
  api_port: 8081
  allow_registration: true
  allow_origins:
    - "http://localhost:3000"
    - "https://*.myapp.com"
```

Go struct:

```go
type ServerConfig struct {
    RedirectPort      int      `yaml:"redirect_port"`
    APIPort           int      `yaml:"api_port"`
    AllowRegistration bool     `yaml:"allow_registration"`
    AllowOrigins      []string `yaml:"allow_origins"`
}
```

---

## Migration Path

1. Add new config fields with sensible defaults
2. Run database migration for `domains` table and `links.domain_id`
3. Deploy backend with new dual-port setup
4. Update frontend to use new API port
5. Deploy frontend with domain management UI

---

## Files to Modify

### Backend
- `internal/config/config.go` - Add new config fields
- `cmd/server/main.go` - Split into two routers
- `internal/handler/auth.go` - Check registration toggle
- `internal/handler/domain.go` - NEW: domain CRUD handlers
- `internal/handler/link.go` - Add domain_id handling
- `internal/repository/domain.go` - NEW: domain repository
- `internal/repository/link.go` - Include domain in queries
- `internal/model/domain.go` - NEW: domain model
- `internal/model/link.go` - Add DomainID field
- `migrations/005_add_domains.sql` - NEW: migration

### Frontend
- `src/components/layout/Sidebar.tsx` - Add Domains menu
- `src/app/dashboard/domains/page.tsx` - NEW: list page
- `src/app/dashboard/domains/create/page.tsx` - NEW: create page
- `src/app/dashboard/links/create/page.tsx` - Add domain dropdown
- `src/app/dashboard/links/[id]/edit/page.tsx` - Add domain dropdown
- `src/lib/api.ts` - Add domain API methods
- `src/types/index.ts` - Add Domain type

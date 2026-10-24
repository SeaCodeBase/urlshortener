# Four Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add registration toggle, domain management, split ports, and configurable CORS to the URL shortener.

**Architecture:** Config-driven feature flags for registration and CORS. New domains table with foreign key to links. Two separate Gin routers for redirect vs API traffic. Frontend domain management pages with dropdown integration in link forms.

**Tech Stack:** Go/Gin (backend), MySQL (database), Next.js/React (frontend), sqlx (database access), zap (logging)

---

## Phase 1: Registration Toggle

### Task 1.1: Add AllowRegistration to Config

**Files:**
- Modify: `/backend/internal/config/config.go`

**Step 1: Add field to ServerConfig struct**

Add `AllowRegistration` field to the existing `ServerConfig` struct:

```go
type ServerConfig struct {
	Port              int  `yaml:"port"`
	AllowRegistration bool `yaml:"allow_registration"`
}
```

**Step 2: Set default value in LoadConfig**

In the `LoadConfig` function, after creating the default config, set the default:

```go
cfg := &Config{
	Server: ServerConfig{
		Port:              8080,
		AllowRegistration: true, // Default to allowing registration
	},
	// ... rest of defaults
}
```

**Step 3: Run existing tests**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat(config): add allow_registration field to server config"
```

---

### Task 1.2: Add Registration Check to Auth Handler

**Files:**
- Modify: `/backend/internal/handler/auth.go`

**Step 1: Write failing test for disabled registration**

Create test file if it doesn't exist, or add to existing:

```go
// In backend/internal/handler/auth_test.go

func TestRegisterDisabled(t *testing.T) {
	// Setup
	cfg := &config.Config{
		Server: config.ServerConfig{
			AllowRegistration: false,
		},
	}

	mockAuthService := &MockAuthService{}
	handler := NewAuthHandler(mockAuthService, cfg)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", handler.Register)

	body := `{"email": "test@example.com", "password": "password123"}`
	req := httptest.NewRequest("POST", "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["error"] != "Registration is disabled" {
		t.Errorf("expected 'Registration is disabled' error, got %s", response["error"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/handler/... -run TestRegisterDisabled -v`
Expected: FAIL (handler doesn't check config yet, or test file doesn't exist)

**Step 3: Update AuthHandler struct to include config**

Modify the `AuthHandler` struct:

```go
type AuthHandler struct {
	authService service.AuthService
	cfg         *config.Config
}

func NewAuthHandler(authService service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		cfg:         cfg,
	}
}
```

**Step 4: Add registration check to Register method**

At the start of the `Register` method, add:

```go
func (h *AuthHandler) Register(c *gin.Context) {
	// Check if registration is allowed
	if !h.cfg.Server.AllowRegistration {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled"})
		return
	}

	// ... rest of existing code
}
```

**Step 5: Update main.go to pass config to AuthHandler**

In `/backend/cmd/server/main.go`, update the handler instantiation:

```go
authHandler := handler.NewAuthHandler(authService, cfg)
```

**Step 6: Run test to verify it passes**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/handler/... -run TestRegisterDisabled -v`
Expected: PASS

**Step 7: Commit**

```bash
git add backend/internal/handler/auth.go backend/internal/handler/auth_test.go backend/cmd/server/main.go
git commit -m "feat(auth): check registration toggle before allowing new signups"
```

---

## Phase 2: Configurable CORS Origins

### Task 2.1: Add AllowOrigins to Config

**Files:**
- Modify: `/backend/internal/config/config.go`

**Step 1: Add AllowOrigins field**

```go
type ServerConfig struct {
	Port              int      `yaml:"port"`
	AllowRegistration bool     `yaml:"allow_registration"`
	AllowOrigins      []string `yaml:"allow_origins"`
}
```

**Step 2: Set default values**

```go
cfg := &Config{
	Server: ServerConfig{
		Port:              8080,
		AllowRegistration: true,
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		},
	},
	// ... rest
}
```

**Step 3: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat(config): add allow_origins field for CORS configuration"
```

---

### Task 2.2: Create Origin Matching Function

**Files:**
- Create: `/backend/internal/middleware/cors.go`
- Create: `/backend/internal/middleware/cors_test.go`

**Step 1: Write failing tests for origin matching**

```go
// backend/internal/middleware/cors_test.go
package middleware

import "testing"

func TestMatchOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		patterns []string
		expected bool
	}{
		{
			name:     "exact match",
			origin:   "http://localhost:3000",
			patterns: []string{"http://localhost:3000"},
			expected: true,
		},
		{
			name:     "no match",
			origin:   "http://evil.com",
			patterns: []string{"http://localhost:3000"},
			expected: false,
		},
		{
			name:     "wildcard subdomain match",
			origin:   "https://app.example.com",
			patterns: []string{"https://*.example.com"},
			expected: true,
		},
		{
			name:     "wildcard does not match root domain",
			origin:   "https://example.com",
			patterns: []string{"https://*.example.com"},
			expected: false,
		},
		{
			name:     "wildcard does not match nested subdomain",
			origin:   "https://sub.app.example.com",
			patterns: []string{"https://*.example.com"},
			expected: false,
		},
		{
			name:     "multiple patterns - second matches",
			origin:   "https://api.myapp.com",
			patterns: []string{"http://localhost:3000", "https://*.myapp.com"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchOrigin(tt.origin, tt.patterns)
			if result != tt.expected {
				t.Errorf("MatchOrigin(%q, %v) = %v, want %v", tt.origin, tt.patterns, result, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/middleware/... -run TestMatchOrigin -v`
Expected: FAIL (file doesn't exist)

**Step 3: Implement MatchOrigin function**

```go
// backend/internal/middleware/cors.go
package middleware

import "strings"

// MatchOrigin checks if an origin matches any of the allowed patterns.
// Supports exact matches and single-level wildcard subdomains (*.example.com).
func MatchOrigin(origin string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "https://*.") || strings.HasPrefix(pattern, "http://*.") {
			// Wildcard pattern: extract protocol and suffix
			protocolEnd := strings.Index(pattern, "://") + 3
			suffix := pattern[protocolEnd+1:] // e.g., ".example.com" from "https://*.example.com"
			protocol := pattern[:protocolEnd]  // e.g., "https://"

			if !strings.HasPrefix(origin, protocol) {
				continue
			}

			if !strings.HasSuffix(origin, suffix) {
				continue
			}

			// Extract subdomain part
			subdomain := origin[len(protocol) : len(origin)-len(suffix)]

			// Subdomain must exist and not contain dots (single level only)
			if len(subdomain) > 0 && !strings.Contains(subdomain, ".") {
				return true
			}
		} else if origin == pattern {
			return true
		}
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/middleware/... -run TestMatchOrigin -v`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/middleware/cors.go backend/internal/middleware/cors_test.go
git commit -m "feat(cors): add origin matching with wildcard subdomain support"
```

---

### Task 2.3: Create CORS Middleware

**Files:**
- Modify: `/backend/internal/middleware/cors.go`
- Modify: `/backend/internal/middleware/cors_test.go`

**Step 1: Add CORS middleware function**

Add to `cors.go`:

```go
import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware creates a CORS middleware with configurable origins.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if origin != "" && MatchOrigin(origin, allowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
```

**Step 2: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add backend/internal/middleware/cors.go
git commit -m "feat(cors): add configurable CORS middleware"
```

---

### Task 2.4: Use Configurable CORS in Main

**Files:**
- Modify: `/backend/cmd/server/main.go`

**Step 1: Replace hardcoded CORS with config-driven middleware**

Find the existing CORS setup and replace with:

```go
// Use configurable CORS middleware
router.Use(middleware.CORSMiddleware(cfg.Server.AllowOrigins))
```

**Step 2: Add import if needed**

```go
import (
	// ... existing imports
	"urlshortener/internal/middleware"
)
```

**Step 3: Build and run to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./cmd/server`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(server): use configurable CORS middleware from config"
```

---

## Phase 3: Split Ports

### Task 3.1: Add Port Config Fields

**Files:**
- Modify: `/backend/internal/config/config.go`

**Step 1: Update ServerConfig with new port fields**

```go
type ServerConfig struct {
	Port              int      `yaml:"port"`              // Legacy, will be deprecated
	RedirectPort      int      `yaml:"redirect_port"`     // Port for redirect server
	APIPort           int      `yaml:"api_port"`          // Port for API server
	AllowRegistration bool     `yaml:"allow_registration"`
	AllowOrigins      []string `yaml:"allow_origins"`
}
```

**Step 2: Update defaults**

```go
cfg := &Config{
	Server: ServerConfig{
		Port:              8080, // Keep for backwards compat
		RedirectPort:      8080, // Default redirect port
		APIPort:           8081, // Default API port
		AllowRegistration: true,
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		},
	},
	// ... rest
}
```

**Step 3: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat(config): add redirect_port and api_port fields"
```

---

### Task 3.2: Split Server into Two Routers

**Files:**
- Modify: `/backend/cmd/server/main.go`

**Step 1: Create redirect router**

Refactor main.go to create two separate routers:

```go
func main() {
	// ... existing setup code for config, db, services ...

	// Redirect Router (public, minimal)
	redirectRouter := gin.New()
	redirectRouter.Use(gin.Recovery())
	redirectRouter.GET("/:code", redirectHandler.Redirect)
	redirectRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "server": "redirect"})
	})

	// API Router (authenticated, full functionality)
	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())
	apiRouter.Use(middleware.CORSMiddleware(cfg.Server.AllowOrigins))

	apiRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "server": "api"})
	})

	// Public auth routes
	auth := apiRouter.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		// ... passkey verify routes
	}

	// Protected routes
	protected := apiRouter.Group("/api")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/auth/me", authHandler.Me)
		protected.PUT("/auth/me", authHandler.UpdateMe)
		protected.POST("/auth/change-password", authHandler.ChangePassword)

		// Links routes
		protected.POST("/links", linkHandler.Create)
		protected.GET("/links", linkHandler.List)
		protected.GET("/links/:id", linkHandler.Get)
		protected.PUT("/links/:id", linkHandler.Update)
		protected.DELETE("/links/:id", linkHandler.Delete)
		protected.GET("/links/:id/stats", statsHandler.GetLinkStats)

		// Passkey routes
		// ... existing passkey routes
	}

	// Start both servers
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", cfg.Server.RedirectPort)
		log.Info("Starting redirect server", zap.String("addr", addr))
		if err := redirectRouter.Run(addr); err != nil {
			log.Fatal("Redirect server failed", zap.Error(err))
		}
	}()

	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", cfg.Server.APIPort)
		log.Info("Starting API server", zap.String("addr", addr))
		if err := apiRouter.Run(addr); err != nil {
			log.Fatal("API server failed", zap.Error(err))
		}
	}()

	wg.Wait()
}
```

**Step 2: Add sync import**

```go
import (
	"sync"
	// ... other imports
)
```

**Step 3: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./cmd/server`
Expected: Build succeeds

**Step 4: Manual test**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && ./server &`
Test: `curl http://localhost:8080/health` (should return redirect server)
Test: `curl http://localhost:8081/health` (should return api server)
Stop: `kill %1`

**Step 5: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(server): split into redirect and API servers on separate ports"
```

---

### Task 3.3: Update Docker Compose

**Files:**
- Modify: `/docker-compose.yml` (or wherever docker config is)

**Step 1: Expose both ports**

```yaml
services:
  backend:
    # ... existing config
    ports:
      - "8080:8080"  # Redirect server
      - "8081:8081"  # API server
```

**Step 2: Commit**

```bash
git add docker-compose.yml
git commit -m "feat(docker): expose both redirect and API ports"
```

---

## Phase 4: Domain Management

### Task 4.1: Create Domain Model

**Files:**
- Create: `/backend/internal/model/domain.go`

**Step 1: Create domain model file**

```go
// backend/internal/model/domain.go
package model

import "time"

// Domain represents a custom domain bound to a user
type Domain struct {
	ID        uint64    `json:"id" db:"id"`
	UserID    uint64    `json:"user_id" db:"user_id"`
	Domain    string    `json:"domain" db:"domain"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

**Step 2: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add backend/internal/model/domain.go
git commit -m "feat(model): add Domain model"
```

---

### Task 4.2: Update Link Model

**Files:**
- Modify: `/backend/internal/model/link.go`

**Step 1: Add DomainID field to Link**

```go
type Link struct {
	ID          uint64    `json:"id" db:"id"`
	UserID      uint64    `json:"user_id" db:"user_id"`
	ShortCode   string    `json:"short_code" db:"short_code"`
	OriginalURL string    `json:"original_url" db:"original_url"`
	Title       *string   `json:"title,omitempty" db:"title"`
	ExpiresAt   NullTime  `json:"expires_at,omitempty" db:"expires_at"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	DomainID    *uint64   `json:"domain_id,omitempty" db:"domain_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
```

**Step 2: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add backend/internal/model/link.go
git commit -m "feat(model): add DomainID field to Link"
```

---

### Task 4.3: Create Database Migration

**Files:**
- Create: `/backend/migrations/005_add_domains.sql`

**Step 1: Create migration file**

```sql
-- 005_add_domains.sql

-- Create domains table
CREATE TABLE IF NOT EXISTS domains (
    id          BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    domain      VARCHAR(255) NOT NULL UNIQUE,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_domains_user_id (user_id)
);

-- Add domain_id to links table
ALTER TABLE links ADD COLUMN domain_id BIGINT UNSIGNED NULL;
ALTER TABLE links ADD FOREIGN KEY fk_links_domain (domain_id) REFERENCES domains(id) ON DELETE SET NULL;
ALTER TABLE links ADD INDEX idx_links_domain_id (domain_id);
```

**Step 2: Commit**

```bash
git add backend/migrations/005_add_domains.sql
git commit -m "feat(migration): add domains table and links.domain_id column"
```

---

### Task 4.4: Create Domain Repository Interface

**Files:**
- Create: `/backend/internal/repository/domain_repo.go`

**Step 1: Define repository interface and errors**

```go
// backend/internal/repository/domain_repo.go
package repository

import (
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"urlshortener/internal/model"
)

var (
	ErrDomainNotFound = errors.New("domain not found")
	ErrDomainExists   = errors.New("domain already exists")
)

type DomainRepository interface {
	Create(domain *model.Domain) error
	GetByID(id uint64) (*model.Domain, error)
	GetByDomain(domain string) (*model.Domain, error)
	ListByUserID(userID uint64) ([]*model.Domain, error)
	Delete(id uint64) error
}

type DomainRepositoryImpl struct {
	db  *sqlx.DB
	log *zap.Logger
}

func NewDomainRepository(db *sqlx.DB, log *zap.Logger) DomainRepository {
	return &DomainRepositoryImpl{db: db, log: log}
}

func (r *DomainRepositoryImpl) Create(domain *model.Domain) error {
	query := `INSERT INTO domains (user_id, domain) VALUES (?, ?)`
	result, err := r.db.Exec(query, domain.UserID, domain.Domain)
	if err != nil {
		if isDuplicateError(err) {
			return ErrDomainExists
		}
		r.log.Error("failed to create domain", zap.Error(err))
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	domain.ID = uint64(id)
	return nil
}

func (r *DomainRepositoryImpl) GetByID(id uint64) (*model.Domain, error) {
	var domain model.Domain
	query := `SELECT id, user_id, domain, created_at FROM domains WHERE id = ?`
	err := r.db.Get(&domain, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDomainNotFound
		}
		r.log.Error("failed to get domain by ID", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	return &domain, nil
}

func (r *DomainRepositoryImpl) GetByDomain(domainName string) (*model.Domain, error) {
	var domain model.Domain
	query := `SELECT id, user_id, domain, created_at FROM domains WHERE domain = ?`
	err := r.db.Get(&domain, query, domainName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDomainNotFound
		}
		r.log.Error("failed to get domain by name", zap.String("domain", domainName), zap.Error(err))
		return nil, err
	}
	return &domain, nil
}

func (r *DomainRepositoryImpl) ListByUserID(userID uint64) ([]*model.Domain, error) {
	var domains []*model.Domain
	query := `SELECT id, user_id, domain, created_at FROM domains WHERE user_id = ? ORDER BY created_at DESC`
	err := r.db.Select(&domains, query, userID)
	if err != nil {
		r.log.Error("failed to list domains by user ID", zap.Uint64("userID", userID), zap.Error(err))
		return nil, err
	}
	return domains, nil
}

func (r *DomainRepositoryImpl) Delete(id uint64) error {
	query := `DELETE FROM domains WHERE id = ?`
	result, err := r.db.Exec(query, id)
	if err != nil {
		r.log.Error("failed to delete domain", zap.Uint64("id", id), zap.Error(err))
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDomainNotFound
	}
	return nil
}

// Helper function to check for MySQL duplicate entry error
func isDuplicateError(err error) bool {
	return err != nil && (contains(err.Error(), "Duplicate entry") || contains(err.Error(), "duplicate key"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add backend/internal/repository/domain_repo.go
git commit -m "feat(repository): add DomainRepository for domain CRUD operations"
```

---

### Task 4.5: Create Domain Handler

**Files:**
- Create: `/backend/internal/handler/domain.go`

**Step 1: Create domain handler**

```go
// backend/internal/handler/domain.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"urlshortener/internal/model"
	"urlshortener/internal/repository"
)

type DomainHandler struct {
	domainRepo repository.DomainRepository
}

func NewDomainHandler(domainRepo repository.DomainRepository) *DomainHandler {
	return &DomainHandler{domainRepo: domainRepo}
}

type CreateDomainRequest struct {
	Domain string `json:"domain" binding:"required"`
}

func (h *DomainHandler) Create(c *gin.Context) {
	userID := c.GetUint64("userID")

	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	domain := &model.Domain{
		UserID: userID,
		Domain: req.Domain,
	}

	if err := h.domainRepo.Create(domain); err != nil {
		if err == repository.ErrDomainExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Domain already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
		return
	}

	c.JSON(http.StatusCreated, domain)
}

func (h *DomainHandler) List(c *gin.Context) {
	userID := c.GetUint64("userID")

	domains, err := h.domainRepo.ListByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list domains"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (h *DomainHandler) Delete(c *gin.Context) {
	userID := c.GetUint64("userID")

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	// Check ownership
	domain, err := h.domainRepo.GetByID(id)
	if err != nil {
		if err == repository.ErrDomainNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get domain"})
		return
	}

	if domain.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't own this domain"})
		return
	}

	if err := h.domainRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete domain"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted"})
}
```

**Step 2: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add backend/internal/handler/domain.go
git commit -m "feat(handler): add DomainHandler for domain CRUD endpoints"
```

---

### Task 4.6: Register Domain Routes

**Files:**
- Modify: `/backend/cmd/server/main.go`

**Step 1: Create domain repository and handler**

Add after other repository/handler creation:

```go
domainRepo := repository.NewDomainRepository(db, log)
domainHandler := handler.NewDomainHandler(domainRepo)
```

**Step 2: Add domain routes to protected group**

```go
protected := apiRouter.Group("/api")
protected.Use(middleware.AuthMiddleware(authService))
{
	// ... existing routes

	// Domain routes
	protected.GET("/domains", domainHandler.List)
	protected.POST("/domains", domainHandler.Create)
	protected.DELETE("/domains/:id", domainHandler.Delete)
}
```

**Step 3: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./cmd/server`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(routes): register domain CRUD endpoints"
```

---

### Task 4.7: Update Link Repository for Domain Support

**Files:**
- Modify: `/backend/internal/repository/link_repo.go`

**Step 1: Update Create method to include domain_id**

Update the INSERT query:

```go
func (r *LinkRepositoryImpl) Create(link *model.Link) error {
	query := `INSERT INTO links (user_id, short_code, original_url, title, expires_at, is_active, domain_id)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.Exec(query, link.UserID, link.ShortCode, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive, link.DomainID)
	// ... rest of method
}
```

**Step 2: Update SELECT queries to include domain_id**

```go
func (r *LinkRepositoryImpl) GetByID(id uint64) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, domain_id, created_at, updated_at
	          FROM links WHERE id = ?`
	// ... rest
}
```

**Step 3: Update Update method**

```go
func (r *LinkRepositoryImpl) Update(link *model.Link) error {
	query := `UPDATE links SET original_url = ?, title = ?, expires_at = ?, is_active = ?, domain_id = ?, updated_at = NOW()
	          WHERE id = ?`
	_, err := r.db.Exec(query, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive, link.DomainID, link.ID)
	// ... rest
}
```

**Step 4: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add backend/internal/repository/link_repo.go
git commit -m "feat(repository): update LinkRepository to handle domain_id"
```

---

### Task 4.8: Update Link Handler for Domain Support

**Files:**
- Modify: `/backend/internal/handler/link.go`

**Step 1: Update CreateLinkRequest to include domain_id**

```go
type CreateLinkRequest struct {
	OriginalURL string  `json:"original_url" binding:"required,url"`
	Title       *string `json:"title,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	DomainID    *uint64 `json:"domain_id,omitempty"`
}
```

**Step 2: Update UpdateLinkRequest**

```go
type UpdateLinkRequest struct {
	OriginalURL string  `json:"original_url,omitempty"`
	Title       *string `json:"title,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	DomainID    *uint64 `json:"domain_id,omitempty"`
}
```

**Step 3: Update Create method to set domain_id**

```go
func (h *LinkHandler) Create(c *gin.Context) {
	// ... existing code

	link := &model.Link{
		UserID:      userID,
		ShortCode:   shortCode,
		OriginalURL: req.OriginalURL,
		Title:       req.Title,
		IsActive:    true,
		DomainID:    req.DomainID,
	}

	// ... rest
}
```

**Step 4: Update Update method to handle domain_id**

```go
func (h *LinkHandler) Update(c *gin.Context) {
	// ... existing code

	if req.DomainID != nil {
		link.DomainID = req.DomainID
	}

	// ... rest
}
```

**Step 5: Build to verify**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add backend/internal/handler/link.go
git commit -m "feat(handler): update LinkHandler to accept domain_id"
```

---

### Task 4.9: Add Frontend Domain Type

**Files:**
- Modify: `/frontend/src/types/index.ts`

**Step 1: Add Domain type**

```typescript
export interface Domain {
  id: number;
  user_id: number;
  domain: string;
  created_at: string;
}

export interface DomainsListResponse {
  domains: Domain[];
}
```

**Step 2: Update Link type to include domain_id**

```typescript
export interface Link {
  id: number;
  user_id: number;
  short_code: string;
  original_url: string;
  title?: string;
  expires_at?: string;
  is_active: boolean;
  domain_id?: number;
  created_at: string;
  updated_at: string;
  short_url?: string;
}
```

**Step 3: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "feat(types): add Domain type and domain_id to Link"
```

---

### Task 4.10: Add Domain API Methods

**Files:**
- Modify: `/frontend/src/lib/api.ts`

**Step 1: Add domain API methods**

```typescript
// Domains
async getDomains(): Promise<DomainsListResponse> {
  return this.request<DomainsListResponse>('/api/domains');
}

async createDomain(domain: string): Promise<Domain> {
  return this.request<Domain>('/api/domains', {
    method: 'POST',
    body: JSON.stringify({ domain }),
  });
}

async deleteDomain(id: number): Promise<void> {
  await this.request<void>(`/api/domains/${id}`, {
    method: 'DELETE',
  });
}
```

**Step 2: Add imports**

```typescript
import { Domain, DomainsListResponse } from '@/types';
```

**Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(api): add domain API methods"
```

---

### Task 4.11: Add Domains to Sidebar

**Files:**
- Modify: `/frontend/src/components/layout/Sidebar.tsx`

**Step 1: Add Globe icon import**

```typescript
import { Home, Link2, BarChart3, Settings, Plus, Globe } from 'lucide-react';
```

**Step 2: Add Domains nav item**

Add after Links in the navigation array:

```typescript
const navItems = [
  { href: '/dashboard', icon: Home, label: 'Home' },
  { href: '/dashboard/links', icon: Link2, label: 'Links' },
  { href: '/dashboard/domains', icon: Globe, label: 'Domains' },
  { href: '/dashboard/analytics', icon: BarChart3, label: 'Analytics' },
  { href: '/dashboard/settings', icon: Settings, label: 'Settings' },
];
```

**Step 3: Commit**

```bash
git add frontend/src/components/layout/Sidebar.tsx
git commit -m "feat(sidebar): add Domains navigation item"
```

---

### Task 4.12: Create Domains List Page

**Files:**
- Create: `/frontend/src/app/dashboard/domains/page.tsx`

**Step 1: Create domains list page**

```typescript
'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Plus, Trash2, Globe } from 'lucide-react';
import { api } from '@/lib/api';
import { Domain } from '@/types';
import { Button } from '@/components/ui/button';

export default function DomainsPage() {
  const router = useRouter();
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchDomains();
  }, []);

  const fetchDomains = async () => {
    try {
      const response = await api.getDomains();
      setDomains(response.domains || []);
    } catch (err) {
      setError('Failed to load domains');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this domain?')) return;

    try {
      await api.deleteDomain(id);
      setDomains(domains.filter(d => d.id !== id));
    } catch (err) {
      setError('Failed to delete domain');
    }
  };

  if (loading) {
    return <div className="p-6">Loading...</div>;
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Domains</h1>
        <Button onClick={() => router.push('/dashboard/domains/create')}>
          <Plus className="w-4 h-4 mr-2" />
          Add Domain
        </Button>
      </div>

      {error && (
        <div className="bg-red-50 text-red-600 p-4 rounded mb-4">{error}</div>
      )}

      {domains.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          <Globe className="w-12 h-12 mx-auto mb-4 opacity-50" />
          <p>No custom domains yet</p>
          <p className="text-sm mt-2">Add a custom domain to use for your short URLs</p>
        </div>
      ) : (
        <div className="bg-white rounded-lg shadow">
          {domains.map((domain) => (
            <div
              key={domain.id}
              className="flex items-center justify-between p-4 border-b last:border-b-0"
            >
              <div>
                <p className="font-medium">{domain.domain}</p>
                <p className="text-sm text-gray-500">
                  Added {new Date(domain.created_at).toLocaleDateString()}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleDelete(domain.id)}
              >
                <Trash2 className="w-4 h-4 text-red-500" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add frontend/src/app/dashboard/domains/page.tsx
git commit -m "feat(frontend): add domains list page"
```

---

### Task 4.13: Create Domain Create Page

**Files:**
- Create: `/frontend/src/app/dashboard/domains/create/page.tsx`

**Step 1: Create domain creation page**

```typescript
'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export default function CreateDomainPage() {
  const router = useRouter();
  const [domain, setDomain] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      await api.createDomain(domain);
      router.push('/dashboard/domains');
    } catch (err: any) {
      if (err.message?.includes('409') || err.message?.includes('already')) {
        setError('This domain is already in use');
      } else {
        setError('Failed to add domain');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 max-w-lg">
      <button
        onClick={() => router.back()}
        className="flex items-center text-gray-600 hover:text-gray-900 mb-6"
      >
        <ArrowLeft className="w-4 h-4 mr-2" />
        Back
      </button>

      <h1 className="text-2xl font-bold mb-6">Add Custom Domain</h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="bg-red-50 text-red-600 p-4 rounded">{error}</div>
        )}

        <div>
          <Label htmlFor="domain">Domain</Label>
          <Input
            id="domain"
            type="text"
            value={domain}
            onChange={(e) => setDomain(e.target.value)}
            placeholder="short.example.com"
            required
          />
          <p className="text-sm text-gray-500 mt-1">
            Enter the domain you want to use for short URLs
          </p>
        </div>

        <Button type="submit" disabled={loading}>
          {loading ? 'Adding...' : 'Add Domain'}
        </Button>
      </form>

      <div className="mt-8 p-4 bg-gray-50 rounded">
        <h3 className="font-medium mb-2">DNS Configuration</h3>
        <p className="text-sm text-gray-600">
          After adding your domain, create a CNAME record pointing to your
          URL shortener server to enable redirects.
        </p>
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add frontend/src/app/dashboard/domains/create/page.tsx
git commit -m "feat(frontend): add domain creation page"
```

---

### Task 4.14: Add Domain Dropdown to Link Create Page

**Files:**
- Modify: `/frontend/src/app/dashboard/links/create/page.tsx`

**Step 1: Add domain state and fetch**

```typescript
import { Domain } from '@/types';

// Add to component state
const [domains, setDomains] = useState<Domain[]>([]);
const [selectedDomainId, setSelectedDomainId] = useState<number | null>(null);

// Add useEffect to fetch domains
useEffect(() => {
  const fetchDomains = async () => {
    try {
      const response = await api.getDomains();
      setDomains(response.domains || []);
    } catch (err) {
      // Ignore error, domains are optional
    }
  };
  fetchDomains();
}, []);
```

**Step 2: Add domain dropdown to form**

```typescript
<div>
  <Label htmlFor="domain">Domain (optional)</Label>
  <select
    id="domain"
    value={selectedDomainId || ''}
    onChange={(e) => setSelectedDomainId(e.target.value ? Number(e.target.value) : null)}
    className="w-full border rounded-md px-3 py-2"
  >
    <option value="">Default</option>
    {domains.map((d) => (
      <option key={d.id} value={d.id}>{d.domain}</option>
    ))}
  </select>
</div>
```

**Step 3: Update submit handler**

```typescript
const handleSubmit = async (e: React.FormEvent) => {
  // ... existing code
  await api.createLink({
    original_url: url,
    title: title || undefined,
    domain_id: selectedDomainId || undefined,
  });
  // ... rest
};
```

**Step 4: Commit**

```bash
git add frontend/src/app/dashboard/links/create/page.tsx
git commit -m "feat(frontend): add domain dropdown to link creation page"
```

---

### Task 4.15: Add Domain Dropdown to Link Edit Page

**Files:**
- Modify: `/frontend/src/app/dashboard/links/[id]/edit/page.tsx`

**Step 1: Add domain state and fetch**

Similar to create page:

```typescript
import { Domain } from '@/types';

const [domains, setDomains] = useState<Domain[]>([]);
const [selectedDomainId, setSelectedDomainId] = useState<number | null>(null);

useEffect(() => {
  const fetchDomains = async () => {
    try {
      const response = await api.getDomains();
      setDomains(response.domains || []);
    } catch (err) {
      // Ignore
    }
  };
  fetchDomains();
}, []);

// Set initial domain when link loads
useEffect(() => {
  if (link?.domain_id) {
    setSelectedDomainId(link.domain_id);
  }
}, [link]);
```

**Step 2: Add domain dropdown to form**

```typescript
<div>
  <Label htmlFor="domain">Domain</Label>
  <select
    id="domain"
    value={selectedDomainId || ''}
    onChange={(e) => setSelectedDomainId(e.target.value ? Number(e.target.value) : null)}
    className="w-full border rounded-md px-3 py-2"
  >
    <option value="">Default</option>
    {domains.map((d) => (
      <option key={d.id} value={d.id}>{d.domain}</option>
    ))}
  </select>
</div>
```

**Step 3: Update save handler**

```typescript
const handleSave = async () => {
  // ... existing code
  await api.updateLink(link.id, {
    original_url: link.original_url,
    title: link.title,
    domain_id: selectedDomainId || undefined,
    // ... other fields
  });
  // ... rest
};
```

**Step 4: Commit**

```bash
git add frontend/src/app/dashboard/links/[id]/edit/page.tsx
git commit -m "feat(frontend): add domain dropdown to link edit page"
```

---

## Phase 5: Final Integration

### Task 5.1: Update Config Example

**Files:**
- Modify: `/backend/config.example.yaml` (or create if doesn't exist)

**Step 1: Create/update example config**

```yaml
server:
  redirect_port: 8080
  api_port: 8081
  allow_registration: true
  allow_origins:
    - "http://localhost:3000"
    - "https://*.myapp.com"

database:
  host: localhost
  port: 3306
  user: root
  password: password
  name: urlshortener

redis:
  host: localhost
  port: 6379

jwt:
  secret: your-secret-key-here

urls:
  base_url: http://localhost:8080
```

**Step 2: Commit**

```bash
git add backend/config.example.yaml
git commit -m "docs: update config example with new server options"
```

---

### Task 5.2: Run Full Test Suite

**Step 1: Run backend tests**

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./... -v`
Expected: All tests pass

**Step 2: Run frontend build**

Run: `cd /Users/jose/GolandProjects/urlshortener/frontend && npm run build`
Expected: Build succeeds

**Step 3: Fix any issues found**

If tests fail, debug and fix before proceeding.

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix: address test failures from integration"
```

---

### Task 5.3: Final Commit

**Step 1: Review all changes**

Run: `git log --oneline -20`
Review the commit history.

**Step 2: Create summary commit if needed**

If everything looks good, the feature is complete!

---

## Summary

This plan implements four features:

1. **Registration Toggle** (Tasks 1.1-1.2): Config flag to disable new user signups
2. **Configurable CORS** (Tasks 2.1-2.4): Origin patterns with wildcard support
3. **Split Ports** (Tasks 3.1-3.3): Separate redirect and API servers
4. **Domain Management** (Tasks 4.1-4.15): Full CRUD for custom domains with frontend UI

Each task is small, testable, and commits frequently. TDD approach used where appropriate.

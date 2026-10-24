# Refactor: Interfaces + Dependency Injection + Mockgen

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor backend to use interfaces with dependency injection pattern. Replace sqlmock with mockgen for unit tests. Rename module from `github.com/jose/urlshortener` to `github.com/SeaCodeBase/urlshortener`.

**Architecture:** All layers (repository, service, handler) depend on interfaces, not concrete types. Generate mocks with mockgen for unit tests.

**Tech Stack:** Go (Gin), mockgen (`go.uber.org/mock`)

---

## Prerequisites

### Step 1: Install mockgen

Run: `go install go.uber.org/mock/mockgen@latest`

### Step 2: Add mockgen dependency

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
go get go.uber.org/mock/gomock
```

---

## Task 1: Rename Module

**Files:**
- Modify: All `.go` files
- Modify: `backend/go.mod`

### Step 1: Update module name across codebase

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
find . -type f -name "*.go" -exec sed -i '' 's|github.com/jose/urlshortener|github.com/SeaCodeBase/urlshortener|g' {} +
sed -i '' 's|github.com/jose/urlshortener|github.com/SeaCodeBase/urlshortener|g' go.mod
go mod tidy
```

### Step 2: Verify build

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go build ./...`
Expected: Build succeeds

### Step 3: Commit

```bash
git add -A
git commit -m "refactor: rename module to github.com/SeaCodeBase/urlshortener"
```

---

## Task 2: Define Repository Interfaces

**Files:**
- Create: `backend/internal/repository/interfaces.go`

### Step 1: Create repository interfaces file

Create `backend/internal/repository/interfaces.go`:

```go
package repository

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

//go:generate mockgen -destination=mocks/mock_user_repo.go -package=mocks . UserRepository
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, userID uint64, passwordHash string) error
}

//go:generate mockgen -destination=mocks/mock_link_repo.go -package=mocks . LinkRepository
type LinkRepository interface {
	Create(ctx context.Context, link *model.Link) error
	GetByID(ctx context.Context, id uint64) (*model.Link, error)
	GetByShortCode(ctx context.Context, shortCode string) (*model.Link, error)
	ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error)
	CountByUserID(ctx context.Context, userID uint64) (int64, error)
	Update(ctx context.Context, link *model.Link) error
	Delete(ctx context.Context, id uint64) error
	ShortCodeExists(ctx context.Context, shortCode string) (bool, error)
}

//go:generate mockgen -destination=mocks/mock_click_repo.go -package=mocks . ClickRepository
type ClickRepository interface {
	BatchInsert(ctx context.Context, clicks []model.Click) error
	GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error)
	GetStatsByLinkID(ctx context.Context, linkID uint64) (*ClickStats, error)
	GetDailyStats(ctx context.Context, linkID uint64, days int) ([]DailyClickStats, error)
	GetTopReferrers(ctx context.Context, linkID uint64, limit int) ([]ReferrerStats, error)
	GetDeviceStats(ctx context.Context, linkID uint64) ([]DeviceStats, error)
	GetBrowserStats(ctx context.Context, linkID uint64) ([]BrowserStats, error)
}

// Stats types used by ClickRepository
type ClickStats struct {
	TotalClicks    int64 `db:"total_clicks"`
	UniqueVisitors int64 `db:"unique_visitors"`
}

type DailyClickStats struct {
	Date   string `db:"date" json:"date"`
	Clicks int64  `db:"clicks" json:"clicks"`
}

type ReferrerStats struct {
	Referrer string `db:"referrer" json:"referrer"`
	Count    int64  `db:"count" json:"count"`
}

type DeviceStats struct {
	DeviceType string `db:"device_type" json:"device_type"`
	Count      int64  `db:"count" json:"count"`
}

type BrowserStats struct {
	Browser string `db:"browser" json:"browser"`
	Count   int64  `db:"count" json:"count"`
}
```

### Step 2: Commit

```bash
git add backend/internal/repository/interfaces.go
git commit -m "feat(repo): add repository interfaces"
```

---

## Task 3: Refactor UserRepository to Implement Interface

**Files:**
- Modify: `backend/internal/repository/user_repo.go`

### Step 1: Rename struct and add interface check

Update `backend/internal/repository/user_repo.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/SeaCodeBase/urlshortener/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailExists = errors.New("email already exists")

// Compile-time check: UserRepositoryImpl implements UserRepository
var _ UserRepository = (*UserRepositoryImpl)(nil)

type UserRepositoryImpl struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (email, password_hash) VALUES (?, ?)`
	result, err := r.db.ExecContext(ctx, query, user.Email, user.PasswordHash)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrEmailExists
		}
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = uint64(id)
	return nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &user, query, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	query := `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = ?`
	err := r.db.GetContext(ctx, &user, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepositoryImpl) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &count, query, email)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *UserRepositoryImpl) UpdatePassword(ctx context.Context, userID uint64, passwordHash string) error {
	query := `UPDATE users SET password_hash = ?, updated_at = NOW() WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}
```

### Step 2: Commit

```bash
git add backend/internal/repository/user_repo.go
git commit -m "refactor(repo): UserRepositoryImpl implements UserRepository interface"
```

---

## Task 4: Refactor LinkRepository to Implement Interface

**Files:**
- Modify: `backend/internal/repository/link_repo.go`

### Step 1: Rename struct and add interface check

Update `backend/internal/repository/link_repo.go` to:
- Rename `LinkRepository` struct to `LinkRepositoryImpl`
- Add compile-time check: `var _ LinkRepository = (*LinkRepositoryImpl)(nil)`
- Update `NewLinkRepository` to return `*LinkRepositoryImpl`

### Step 2: Commit

```bash
git add backend/internal/repository/link_repo.go
git commit -m "refactor(repo): LinkRepositoryImpl implements LinkRepository interface"
```

---

## Task 5: Refactor ClickRepository to Implement Interface

**Files:**
- Modify: `backend/internal/repository/click_repo.go`

### Step 1: Rename struct and add stats methods

Update `backend/internal/repository/click_repo.go`:

```go
package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/SeaCodeBase/urlshortener/internal/model"
)

// Compile-time check: ClickRepositoryImpl implements ClickRepository
var _ ClickRepository = (*ClickRepositoryImpl)(nil)

type ClickRepositoryImpl struct {
	db *sqlx.DB
}

func NewClickRepository(db *sqlx.DB) *ClickRepositoryImpl {
	return &ClickRepositoryImpl{db: db}
}

func (r *ClickRepositoryImpl) BatchInsert(ctx context.Context, clicks []model.Click) error {
	if len(clicks) == 0 {
		return nil
	}

	query := `INSERT INTO clicks (link_id, clicked_at, ip_hash, user_agent, referrer, country, city, device_type, browser, utm_source, utm_medium, utm_campaign)
			  VALUES (:link_id, :clicked_at, :ip_hash, :user_agent, :referrer, :country, :city, :device_type, :browser, :utm_source, :utm_medium, :utm_campaign)`

	_, err := r.db.NamedExecContext(ctx, query, clicks)
	return err
}

func (r *ClickRepositoryImpl) GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &count, query, linkID)
	return count, err
}

func (r *ClickRepositoryImpl) GetStatsByLinkID(ctx context.Context, linkID uint64) (*ClickStats, error) {
	var stats ClickStats
	query := `SELECT COUNT(*) as total_clicks, COUNT(DISTINCT ip_hash) as unique_visitors FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &stats, query, linkID)
	return &stats, err
}

func (r *ClickRepositoryImpl) GetDailyStats(ctx context.Context, linkID uint64, days int) ([]DailyClickStats, error) {
	var stats []DailyClickStats
	query := `SELECT DATE(clicked_at) as date, COUNT(*) as clicks FROM clicks WHERE link_id = ? AND clicked_at >= DATE_SUB(NOW(), INTERVAL ? DAY) GROUP BY DATE(clicked_at) ORDER BY date DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID, days)
	return stats, err
}

func (r *ClickRepositoryImpl) GetTopReferrers(ctx context.Context, linkID uint64, limit int) ([]ReferrerStats, error) {
	var stats []ReferrerStats
	query := `SELECT COALESCE(NULLIF(referrer, ''), 'Direct') as referrer, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY referrer ORDER BY count DESC LIMIT ?`
	err := r.db.SelectContext(ctx, &stats, query, linkID, limit)
	return stats, err
}

func (r *ClickRepositoryImpl) GetDeviceStats(ctx context.Context, linkID uint64) ([]DeviceStats, error) {
	var stats []DeviceStats
	query := `SELECT device_type, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY device_type ORDER BY count DESC`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	return stats, err
}

func (r *ClickRepositoryImpl) GetBrowserStats(ctx context.Context, linkID uint64) ([]BrowserStats, error) {
	var stats []BrowserStats
	query := `SELECT COALESCE(NULLIF(browser, ''), 'Unknown') as browser, COUNT(*) as count FROM clicks WHERE link_id = ? GROUP BY browser ORDER BY count DESC LIMIT 10`
	err := r.db.SelectContext(ctx, &stats, query, linkID)
	return stats, err
}
```

### Step 2: Commit

```bash
git add backend/internal/repository/click_repo.go
git commit -m "refactor(repo): ClickRepositoryImpl implements ClickRepository interface"
```

---

## Task 6: Generate Repository Mocks

**Files:**
- Create: `backend/internal/repository/mocks/` directory and mock files

### Step 1: Create mocks directory and generate

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
mkdir -p internal/repository/mocks
go generate ./internal/repository/...
```

### Step 2: Verify mocks generated

Run: `ls -la internal/repository/mocks/`
Expected: `mock_user_repo.go`, `mock_link_repo.go`, `mock_click_repo.go`

### Step 3: Commit

```bash
git add backend/internal/repository/mocks/
git commit -m "feat(repo): generate repository mocks with mockgen"
```

---

## Task 7: Define Service Interfaces

**Files:**
- Create: `backend/internal/service/interfaces.go`

### Step 1: Create service interfaces file

Create `backend/internal/service/interfaces.go`:

```go
package service

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/model"
)

//go:generate mockgen -destination=mocks/mock_auth_service.go -package=mocks . AuthService
type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*AuthResponse, error)
	Login(ctx context.Context, input LoginInput) (*AuthResponse, error)
	GetUserByID(ctx context.Context, userID uint64) (*model.User, error)
	ValidateToken(tokenString string) (uint64, error)
	ChangePassword(ctx context.Context, userID uint64, input ChangePasswordInput) error
}

//go:generate mockgen -destination=mocks/mock_link_service.go -package=mocks . LinkService
type LinkService interface {
	Create(ctx context.Context, userID uint64, input CreateLinkInput) (*model.Link, error)
	GetByID(ctx context.Context, userID, linkID uint64) (*model.Link, error)
	List(ctx context.Context, userID uint64, params ListLinksParams) (*ListLinksResult, error)
	Update(ctx context.Context, userID, linkID uint64, input UpdateLinkInput) (*model.Link, error)
	Delete(ctx context.Context, userID, linkID uint64) error
}

//go:generate mockgen -destination=mocks/mock_stats_service.go -package=mocks . StatsService
type StatsService interface {
	GetLinkStats(ctx context.Context, userID, linkID uint64) (*LinkStatsResponse, error)
}

//go:generate mockgen -destination=mocks/mock_shortcode_service.go -package=mocks . ShortCodeService
type ShortCodeService interface {
	Generate(ctx context.Context) (string, error)
	IsValid(code string) bool
	IsAvailable(ctx context.Context, code string) (bool, error)
}
```

### Step 2: Commit

```bash
git add backend/internal/service/interfaces.go
git commit -m "feat(service): add service interfaces"
```

---

## Task 8: Refactor AuthService to Implement Interface

**Files:**
- Modify: `backend/internal/service/auth_service.go`

### Step 1: Rename struct and use repository interface

Update `backend/internal/service/auth_service.go`:

```go
package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already taken")
	ErrWrongPassword      = errors.New("current password is incorrect")
)

// Compile-time check: AuthServiceImpl implements AuthService
var _ AuthService = (*AuthServiceImpl)(nil)

type AuthServiceImpl struct {
	userRepo  repository.UserRepository  // Interface, not concrete type
	jwtSecret []byte
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
	}
}

type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required,min=8"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *AuthServiceImpl) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	exists, err := s.userRepo.EmailExists(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:        input.Email,
		PasswordHash: string(hash),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}

func (s *AuthServiceImpl) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}

func (s *AuthServiceImpl) GetUserByID(ctx context.Context, userID uint64) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthServiceImpl) ChangePassword(ctx context.Context, userID uint64, input ChangePasswordInput) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.OldPassword)); err != nil {
		return ErrWrongPassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(ctx, userID, string(newHash))
}

func (s *AuthServiceImpl) generateToken(userID uint64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthServiceImpl) ValidateToken(tokenString string) (uint64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, errors.New("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("invalid token: missing user_id")
	}

	return uint64(userIDFloat), nil
}
```

### Step 2: Commit

```bash
git add backend/internal/service/auth_service.go
git commit -m "refactor(service): AuthServiceImpl implements AuthService interface with DI"
```

---

## Task 9: Refactor LinkService to Implement Interface

**Files:**
- Modify: `backend/internal/service/link_service.go`

### Step 1: Rename struct and use repository interface

- Rename `LinkService` struct to `LinkServiceImpl`
- Change `linkRepo` field type from `*repository.LinkRepository` to `repository.LinkRepository` (interface)
- Add compile-time check: `var _ LinkService = (*LinkServiceImpl)(nil)`
- Update `NewLinkService` signature

### Step 2: Commit

```bash
git add backend/internal/service/link_service.go
git commit -m "refactor(service): LinkServiceImpl implements LinkService interface with DI"
```

---

## Task 10: Refactor ShortCodeService to Implement Interface

**Files:**
- Modify: `backend/internal/service/shortcode_service.go`

### Step 1: Rename struct and use repository interface

- Rename `ShortCodeService` struct to `ShortCodeServiceImpl`
- Change `linkRepo` field type to `repository.LinkRepository` (interface)
- Add compile-time check: `var _ ShortCodeService = (*ShortCodeServiceImpl)(nil)`

### Step 2: Commit

```bash
git add backend/internal/service/shortcode_service.go
git commit -m "refactor(service): ShortCodeServiceImpl implements ShortCodeService interface"
```

---

## Task 11: Generate Service Mocks

**Files:**
- Create: `backend/internal/service/mocks/` directory and mock files

### Step 1: Create mocks directory and generate

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
mkdir -p internal/service/mocks
go generate ./internal/service/...
```

### Step 2: Commit

```bash
git add backend/internal/service/mocks/
git commit -m "feat(service): generate service mocks with mockgen"
```

---

## Task 12: Refactor Handlers to Use Service Interfaces

**Files:**
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/handler/link.go`

### Step 1: Update AuthHandler

Update `backend/internal/handler/auth.go`:

```go
type AuthHandler struct {
	authService service.AuthService  // Interface, not *AuthServiceImpl
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}
```

### Step 2: Update LinkHandler

Update `backend/internal/handler/link.go`:

```go
type LinkHandler struct {
	linkService service.LinkService  // Interface, not *LinkServiceImpl
	baseURL     string
}

func NewLinkHandler(linkService service.LinkService, cfg *config.Config) *LinkHandler {
	return &LinkHandler{
		linkService: linkService,
		baseURL:     cfg.BaseURL,
	}
}
```

### Step 3: Commit

```bash
git add backend/internal/handler/auth.go backend/internal/handler/link.go
git commit -m "refactor(handler): handlers use service interfaces"
```

---

## Task 13: Update Middleware to Use Service Interface

**Files:**
- Modify: `backend/internal/middleware/auth.go`

### Step 1: Update AuthMiddleware

Change parameter type from `*service.AuthService` to `service.AuthService`:

```go
func AuthMiddleware(authService service.AuthService) gin.HandlerFunc {
	// ... existing code
}
```

### Step 2: Commit

```bash
git add backend/internal/middleware/auth.go
git commit -m "refactor(middleware): use AuthService interface"
```

---

## Task 14: Write Service Tests with Mockgen

**Files:**
- Create: `backend/internal/service/auth_service_test.go`

### Step 1: Write tests using generated mocks

Create `backend/internal/service/auth_service_test.go`:

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
	"golang.org/x/crypto/bcrypt"
)

func TestAuthServiceImpl_ChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	oldPass := "oldpassword123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(oldPass), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(user, nil)

	mockUserRepo.EXPECT().
		UpdatePassword(gomock.Any(), uint64(1), gomock.Any()).
		Return(nil)

	input := service.ChangePasswordInput{
		OldPassword: oldPass,
		NewPassword: "newpassword456",
	}

	err := svc.ChangePassword(context.Background(), uint64(1), input)
	assert.NoError(t, err)
}

func TestAuthServiceImpl_ChangePassword_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByID(gomock.Any(), uint64(1)).
		Return(user, nil)

	input := service.ChangePasswordInput{
		OldPassword: "wrongpassword",
		NewPassword: "newpassword456",
	}

	err := svc.ChangePassword(context.Background(), uint64(1), input)
	assert.ErrorIs(t, err, service.ErrWrongPassword)
}

func TestAuthServiceImpl_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	password := "testpassword123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := &model.User{ID: 1, Email: "test@example.com", PasswordHash: string(hash)}

	mockUserRepo.EXPECT().
		GetByEmail(gomock.Any(), "test@example.com").
		Return(user, nil)

	input := service.LoginInput{
		Email:    "test@example.com",
		Password: password,
	}

	resp, err := svc.Login(context.Background(), input)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, user.Email, resp.User.Email)
}

func TestAuthServiceImpl_Login_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	mockUserRepo.EXPECT().
		GetByEmail(gomock.Any(), "notfound@example.com").
		Return(nil, repository.ErrUserNotFound)

	input := service.LoginInput{
		Email:    "notfound@example.com",
		Password: "anypassword",
	}

	resp, err := svc.Login(context.Background(), input)
	assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	assert.Nil(t, resp)
}

func TestAuthServiceImpl_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	mockUserRepo.EXPECT().
		EmailExists(gomock.Any(), "new@example.com").
		Return(false, nil)

	mockUserRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, user *model.User) error {
			user.ID = 1
			return nil
		})

	input := service.RegisterInput{
		Email:    "new@example.com",
		Password: "password123",
	}

	resp, err := svc.Register(context.Background(), input)
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "new@example.com", resp.User.Email)
}

func TestAuthServiceImpl_Register_EmailTaken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(ctrl)
	svc := service.NewAuthService(mockUserRepo, "test-secret")

	mockUserRepo.EXPECT().
		EmailExists(gomock.Any(), "taken@example.com").
		Return(true, nil)

	input := service.RegisterInput{
		Email:    "taken@example.com",
		Password: "password123",
	}

	resp, err := svc.Register(context.Background(), input)
	assert.ErrorIs(t, err, service.ErrEmailTaken)
	assert.Nil(t, resp)
}
```

### Step 2: Run tests

Run: `cd /Users/jose/GolandProjects/urlshortener/backend && go test ./internal/service/... -v`
Expected: All tests PASS

### Step 3: Commit

```bash
git add backend/internal/service/auth_service_test.go
git commit -m "test(service): add AuthService tests using mockgen"
```

---

## Task 15: Update main.go for New Types

**Files:**
- Modify: `backend/cmd/server/main.go`

### Step 1: Update main.go

The `NewXxxRepository` and `NewXxxService` functions now return `*XxxImpl` types which implement the interfaces. No changes needed if you're passing them to functions that accept interfaces.

Verify build: `go build ./cmd/server`

### Step 2: Commit if changes needed

```bash
git add backend/cmd/server/main.go
git commit -m "refactor(main): update for new interface types"
```

---

## Final Verification

### Step 1: Run all tests

Run:
```bash
cd /Users/jose/GolandProjects/urlshortener/backend
go test ./... -v
```
Expected: All tests PASS

### Step 2: Build

Run: `go build ./...`
Expected: Build succeeds

### Step 3: Final commit if needed

```bash
git add -A
git commit -m "refactor: complete interfaces + DI + mockgen migration"
```

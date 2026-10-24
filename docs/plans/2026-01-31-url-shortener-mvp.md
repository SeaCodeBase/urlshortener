# URL Shortener MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a working URL shortener with user authentication, link management, redirect service, and basic analytics.

**Architecture:** Two-service architecture (Go backend + Next.js frontend) with MariaDB for persistence and Redis for caching. Backend uses layered architecture (Handler → Service → Repository). Frontend uses App Router with Zustand state management.

**Tech Stack:** Go 1.21+, Gin, sqlx, go-redis, bcrypt, golang-jwt | Next.js 14, TypeScript, Tailwind CSS, shadcn/ui, Zustand | MariaDB 10.11, Redis 7

---

## Phase 1: Backend Foundation

### Task 1: Initialize Go Module and Project Structure

**Files:**
- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/config/config.go`
- Create: `backend/pkg/logger/zap.go`

**Step 1: Create backend directory and initialize Go module**

```bash
mkdir -p backend/cmd/server backend/internal/config backend/pkg/logger
cd backend && go mod init github.com/jose/urlshortener
```

**Step 2: Add dependencies**

```bash
cd backend && go get github.com/gin-gonic/gin go.uber.org/zap github.com/jmoiron/sqlx github.com/go-sql-driver/mysql github.com/redis/go-redis/v9 github.com/golang-jwt/jwt/v5 golang.org/x/crypto github.com/joho/godotenv
```

**Step 3: Create config.go**

```go
// backend/internal/config/config.go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort  string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	RedisHost   string
	RedisPort   string
	JWTSecret   string
	BaseURL     string
}

func Load() *Config {
	return &Config{
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "3306"),
		DBUser:      getEnv("DB_USER", "urlshortener"),
		DBPassword:  getEnv("DB_PASSWORD", ""),
		DBName:      getEnv("DB_NAME", "urlshortener"),
		RedisHost:   getEnv("REDIS_HOST", "localhost"),
		RedisPort:   getEnv("REDIS_PORT", "6379"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-in-prod"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
```

**Step 4: Create zap.go logger**

```go
// backend/pkg/logger/zap.go
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.SugaredLogger

func Init(isDev bool) {
	var cfg zap.Config
	if isDev {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	Log = logger.Sugar()
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
```

**Step 5: Create main.go skeleton**

```go
// backend/cmd/server/main.go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/jose/urlshortener/internal/config"
	"github.com/jose/urlshortener/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger.Init(true)
	defer logger.Sync()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	logger.Log.Infof("Starting server on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		logger.Log.Fatalf("Failed to start server: %v", err)
	}
}
```

**Step 6: Verify it compiles and runs**

Run: `cd backend && go build ./cmd/server && ./server`
Expected: Server starts, health endpoint responds

**Step 7: Commit**

```bash
git add backend/
git commit -m "feat: initialize Go backend with config and logger"
```

---

### Task 2: Database Connection and Migration Setup

**Files:**
- Create: `backend/internal/database/db.go`
- Create: `backend/migrations/001_initial_schema.sql`
- Modify: `backend/cmd/server/main.go`

**Step 1: Create db.go**

```go
// backend/internal/database/db.go
package database

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/jose/urlshortener/internal/config"
	"github.com/jose/urlshortener/pkg/logger"
)

func Connect(cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Log.Info("Connected to MariaDB")
	return db, nil
}
```

**Step 2: Create initial migration**

```sql
-- backend/migrations/001_initial_schema.sql
CREATE TABLE IF NOT EXISTS users (
    id              BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS links (
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

CREATE TABLE IF NOT EXISTS clicks (
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

CREATE TABLE IF NOT EXISTS link_stats_daily (
    link_id         BIGINT UNSIGNED NOT NULL,
    date            DATE NOT NULL,
    total_clicks    INT UNSIGNED DEFAULT 0,
    unique_visitors INT UNSIGNED DEFAULT 0,
    PRIMARY KEY (link_id, date),
    FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
);
```

**Step 3: Update main.go to connect to database**

Add after config load:
```go
db, err := database.Connect(cfg)
if err != nil {
	logger.Log.Fatalf("Database connection failed: %v", err)
}
defer db.Close()
```

**Step 4: Commit**

```bash
git add backend/
git commit -m "feat: add database connection and initial schema migration"
```

---

### Task 3: Redis Connection

**Files:**
- Create: `backend/internal/cache/redis.go`
- Modify: `backend/cmd/server/main.go`

**Step 1: Create redis.go**

```go
// backend/internal/cache/redis.go
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/jose/urlshortener/internal/config"
	"github.com/jose/urlshortener/pkg/logger"
)

func Connect(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Log.Info("Connected to Redis")
	return client, nil
}
```

**Step 2: Update main.go**

Add after database connection:
```go
rdb, err := cache.Connect(cfg)
if err != nil {
	logger.Log.Fatalf("Redis connection failed: %v", err)
}
defer rdb.Close()
```

**Step 3: Commit**

```bash
git add backend/
git commit -m "feat: add Redis connection"
```

---

### Task 4: User Model and Repository

**Files:**
- Create: `backend/internal/model/user.go`
- Create: `backend/internal/repository/user_repo.go`
- Create: `backend/internal/repository/user_repo_test.go`

**Step 1: Create user model**

```go
// backend/internal/model/user.go
package model

import "time"

type User struct {
	ID           uint64    `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
```

**Step 2: Create user repository**

```go
// backend/internal/repository/user_repo.go
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/jose/urlshortener/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailExists = errors.New("email already exists")

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (email, password_hash) VALUES (?, ?)`
	result, err := r.db.ExecContext(ctx, query, user.Email, user.PasswordHash)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = uint64(id)
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
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

func (r *UserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
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

func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE email = ?`
	err := r.db.GetContext(ctx, &count, query, email)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
```

**Step 3: Create user repository test**

```go
// backend/internal/repository/user_repo_test.go
package repository_test

import (
	"context"
	"testing"

	"github.com/jose/urlshortener/internal/model"
	"github.com/jose/urlshortener/internal/repository"
	"github.com/jose/urlshortener/internal/testutil"
)

func TestUserRepository_Create(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	// Create user first
	user := &model.User{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}
	_ = repo.Create(ctx, user)

	// Get by email
	found, err := repo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}

	if found.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, found.Email)
	}
}

func TestUserRepository_GetByEmail_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	repo := repository.NewUserRepository(db)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if err != repository.ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}
}
```

**Step 4: Create test utility (for database tests)**

```go
// backend/internal/testutil/db.go
package testutil

import (
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func SetupTestDB(t *testing.T) *sqlx.DB {
	// Use test database - configure via TEST_DB_* env vars
	dsn := "urlshortener:password@tcp(localhost:3306)/urlshortener_test?parseTime=true&loc=UTC"
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Skipf("Skipping test: could not connect to test database: %v", err)
	}

	// Clean tables before each test
	_, _ = db.Exec("DELETE FROM clicks")
	_, _ = db.Exec("DELETE FROM links")
	_, _ = db.Exec("DELETE FROM users")

	return db
}
```

**Step 5: Run tests**

Run: `cd backend && go test ./internal/repository/... -v`
Expected: Tests pass (or skip if no test DB)

**Step 6: Commit**

```bash
git add backend/
git commit -m "feat: add user model and repository with tests"
```

---

### Task 5: Auth Service (Password Hashing + JWT)

**Files:**
- Create: `backend/internal/service/auth_service.go`
- Create: `backend/internal/service/auth_service_test.go`

**Step 1: Create auth service**

```go
// backend/internal/service/auth_service.go
package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jose/urlshortener/internal/model"
	"github.com/jose/urlshortener/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already taken")
)

type AuthService struct {
	userRepo  *repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(userRepo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
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

type AuthResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
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

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
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

func (s *AuthService) GetUserByID(ctx context.Context, userID uint64) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) generateToken(userID uint64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) ValidateToken(tokenString string) (uint64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := uint64(claims["user_id"].(float64))
		return userID, nil
	}

	return 0, errors.New("invalid token")
}
```

**Step 2: Create auth service test**

```go
// backend/internal/service/auth_service_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/jose/urlshortener/internal/repository"
	"github.com/jose/urlshortener/internal/service"
	"github.com/jose/urlshortener/internal/testutil"
)

func TestAuthService_Register(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()
	input := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	resp, err := authService.Register(ctx, input)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected token to be set")
	}

	if resp.User.Email != input.Email {
		t.Errorf("Expected email %s, got %s", input.Email, resp.User.Email)
	}
}

func TestAuthService_Login(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()

	// Register first
	registerInput := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, _ = authService.Register(ctx, registerInput)

	// Login
	loginInput := service.LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	resp, err := authService.Login(ctx, loginInput)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if resp.Token == "" {
		t.Error("Expected token to be set")
	}
}

func TestAuthService_Login_InvalidPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()

	// Register
	registerInput := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, _ = authService.Register(ctx, registerInput)

	// Login with wrong password
	loginInput := service.LoginInput{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}

	_, err := authService.Login(ctx, loginInput)
	if err != service.ErrInvalidCredentials {
		t.Errorf("Expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, "test-secret")

	ctx := context.Background()

	// Register to get a token
	input := service.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	resp, _ := authService.Register(ctx, input)

	// Validate token
	userID, err := authService.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if userID != resp.User.ID {
		t.Errorf("Expected user ID %d, got %d", resp.User.ID, userID)
	}
}
```

**Step 3: Run tests**

Run: `cd backend && go test ./internal/service/... -v`
Expected: Tests pass

**Step 4: Commit**

```bash
git add backend/
git commit -m "feat: add auth service with registration, login, and JWT"
```

---

### Task 6: Auth Middleware

**Files:**
- Create: `backend/internal/middleware/auth.go`

**Step 1: Create auth middleware**

```go
// backend/internal/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jose/urlshortener/internal/service"
)

func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		token := parts[1]
		userID, err := authService.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) uint64 {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	return userID.(uint64)
}
```

**Step 2: Commit**

```bash
git add backend/
git commit -m "feat: add JWT auth middleware"
```

---

### Task 7: Auth Handlers

**Files:**
- Create: `backend/internal/handler/auth.go`
- Modify: `backend/cmd/server/main.go`

**Step 1: Create auth handlers**

```go
// backend/internal/handler/auth.go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jose/urlshortener/internal/middleware"
	"github.com/jose/urlshortener/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), input)
	if errors.Is(err, service.ErrEmailTaken) {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), input)
	if errors.Is(err, service.ErrInvalidCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, user)
}
```

**Step 2: Wire up routes in main.go**

Update main.go to register auth routes:
```go
// Setup repositories
userRepo := repository.NewUserRepository(db)

// Setup services
authService := service.NewAuthService(userRepo, cfg.JWTSecret)

// Setup handlers
authHandler := handler.NewAuthHandler(authService)

// Routes
api := r.Group("/api")
{
	auth := api.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.GET("/me", middleware.AuthMiddleware(authService), authHandler.Me)
	}
}
```

**Step 3: Test manually**

```bash
# Register
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

**Step 4: Commit**

```bash
git add backend/
git commit -m "feat: add auth handlers for register, login, and me endpoints"
```

---

## Phase 2: Link Management

### Task 8: Link Model and Repository

**Files:**
- Create: `backend/internal/model/link.go`
- Create: `backend/internal/repository/link_repo.go`

**Step 1: Create link model**

```go
// backend/internal/model/link.go
package model

import (
	"database/sql"
	"time"
)

type Link struct {
	ID          uint64       `db:"id" json:"id"`
	UserID      uint64       `db:"user_id" json:"user_id"`
	ShortCode   string       `db:"short_code" json:"short_code"`
	OriginalURL string       `db:"original_url" json:"original_url"`
	Title       *string      `db:"title" json:"title,omitempty"`
	ExpiresAt   sql.NullTime `db:"expires_at" json:"expires_at,omitempty"`
	IsActive    bool         `db:"is_active" json:"is_active"`
	CreatedAt   time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at" json:"updated_at"`
}

type LinkWithStats struct {
	Link
	TotalClicks int64 `json:"total_clicks"`
}
```

**Step 2: Create link repository**

```go
// backend/internal/repository/link_repo.go
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/jose/urlshortener/internal/model"
)

var ErrLinkNotFound = errors.New("link not found")

type LinkRepository struct {
	db *sqlx.DB
}

func NewLinkRepository(db *sqlx.DB) *LinkRepository {
	return &LinkRepository{db: db}
}

func (r *LinkRepository) Create(ctx context.Context, link *model.Link) error {
	query := `INSERT INTO links (user_id, short_code, original_url, title, expires_at, is_active)
			  VALUES (?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		link.UserID, link.ShortCode, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	link.ID = uint64(id)
	return nil
}

func (r *LinkRepository) GetByID(ctx context.Context, id uint64) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, created_at, updated_at
			  FROM links WHERE id = ?`
	err := r.db.GetContext(ctx, &link, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	return &link, err
}

func (r *LinkRepository) GetByShortCode(ctx context.Context, code string) (*model.Link, error) {
	var link model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, created_at, updated_at
			  FROM links WHERE short_code = ?`
	err := r.db.GetContext(ctx, &link, query, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLinkNotFound
	}
	return &link, err
}

func (r *LinkRepository) ListByUserID(ctx context.Context, userID uint64, limit, offset int) ([]model.Link, error) {
	var links []model.Link
	query := `SELECT id, user_id, short_code, original_url, title, expires_at, is_active, created_at, updated_at
			  FROM links WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	err := r.db.SelectContext(ctx, &links, query, userID, limit, offset)
	return links, err
}

func (r *LinkRepository) Update(ctx context.Context, link *model.Link) error {
	query := `UPDATE links SET original_url = ?, title = ?, expires_at = ?, is_active = ?, updated_at = NOW()
			  WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, link.OriginalURL, link.Title, link.ExpiresAt, link.IsActive, link.ID)
	return err
}

func (r *LinkRepository) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM links WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *LinkRepository) ShortCodeExists(ctx context.Context, code string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM links WHERE short_code = ?`
	err := r.db.GetContext(ctx, &count, query, code)
	return count > 0, err
}

func (r *LinkRepository) CountByUserID(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM links WHERE user_id = ?`
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}
```

**Step 3: Commit**

```bash
git add backend/
git commit -m "feat: add link model and repository"
```

---

### Task 9: Short Code Generator Service

**Files:**
- Create: `backend/internal/service/shortcode_service.go`
- Create: `backend/internal/service/shortcode_service_test.go`

**Step 1: Create shortcode service**

```go
// backend/internal/service/shortcode_service.go
package service

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/jose/urlshortener/internal/repository"
)

const (
	// Base62 alphabet for short codes
	alphabet     = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	defaultLen   = 7
)

type ShortCodeService struct {
	linkRepo *repository.LinkRepository
}

func NewShortCodeService(linkRepo *repository.LinkRepository) *ShortCodeService {
	return &ShortCodeService{linkRepo: linkRepo}
}

func (s *ShortCodeService) Generate(ctx context.Context) (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		code, err := generateRandomCode(defaultLen)
		if err != nil {
			return "", err
		}

		exists, err := s.linkRepo.ShortCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}

	// If we exhausted attempts, try with longer code
	return generateRandomCode(defaultLen + 1)
}

func (s *ShortCodeService) IsValid(code string) bool {
	if len(code) < 4 || len(code) > 16 {
		return false
	}
	for _, c := range code {
		if !isAlphanumeric(c) {
			return false
		}
	}
	return true
}

func (s *ShortCodeService) IsAvailable(ctx context.Context, code string) (bool, error) {
	exists, err := s.linkRepo.ShortCodeExists(ctx, code)
	return !exists, err
}

func generateRandomCode(length int) (string, error) {
	result := make([]byte, length)
	alphabetLen := big.NewInt(int64(len(alphabet)))

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = alphabet[num.Int64()]
	}

	return string(result), nil
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
```

**Step 2: Create test**

```go
// backend/internal/service/shortcode_service_test.go
package service_test

import (
	"testing"

	"github.com/jose/urlshortener/internal/service"
)

func TestShortCodeService_IsValid(t *testing.T) {
	svc := &service.ShortCodeService{}

	tests := []struct {
		code  string
		valid bool
	}{
		{"abc123", true},
		{"Ab3XyZ", true},
		{"abc", false},       // too short
		{"abc_123", false},   // invalid char
		{"abc-123", false},   // invalid char
		{"", false},          // empty
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			// Note: IsValid is not exported, but we can test Generate
			if got := svc.IsValid(tt.code); got != tt.valid {
				t.Errorf("IsValid(%q) = %v, want %v", tt.code, got, tt.valid)
			}
		})
	}
}
```

**Step 3: Run test**

Run: `cd backend && go test ./internal/service/... -v -run ShortCode`
Expected: Tests pass

**Step 4: Commit**

```bash
git add backend/
git commit -m "feat: add short code generation service"
```

---

### Task 10: Link Service

**Files:**
- Create: `backend/internal/service/link_service.go`

**Step 1: Create link service**

```go
// backend/internal/service/link_service.go
package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jose/urlshortener/internal/model"
	"github.com/jose/urlshortener/internal/repository"
)

var (
	ErrLinkNotFound     = errors.New("link not found")
	ErrNotLinkOwner     = errors.New("not the owner of this link")
	ErrInvalidShortCode = errors.New("invalid short code")
	ErrShortCodeTaken   = errors.New("short code already taken")
)

type LinkService struct {
	linkRepo   *repository.LinkRepository
	shortCode  *ShortCodeService
}

func NewLinkService(linkRepo *repository.LinkRepository, shortCode *ShortCodeService) *LinkService {
	return &LinkService{
		linkRepo:  linkRepo,
		shortCode: shortCode,
	}
}

type CreateLinkInput struct {
	OriginalURL string     `json:"original_url" binding:"required,url"`
	CustomCode  string     `json:"custom_code,omitempty"`
	Title       string     `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type UpdateLinkInput struct {
	OriginalURL string     `json:"original_url,omitempty"`
	Title       string     `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    *bool      `json:"is_active,omitempty"`
}

type ListLinksParams struct {
	Page  int
	Limit int
}

type ListLinksResult struct {
	Links      []model.Link `json:"links"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	TotalPages int          `json:"total_pages"`
}

func (s *LinkService) Create(ctx context.Context, userID uint64, input CreateLinkInput) (*model.Link, error) {
	var shortCode string
	var err error

	if input.CustomCode != "" {
		if !s.shortCode.IsValid(input.CustomCode) {
			return nil, ErrInvalidShortCode
		}
		available, err := s.shortCode.IsAvailable(ctx, input.CustomCode)
		if err != nil {
			return nil, err
		}
		if !available {
			return nil, ErrShortCodeTaken
		}
		shortCode = input.CustomCode
	} else {
		shortCode, err = s.shortCode.Generate(ctx)
		if err != nil {
			return nil, err
		}
	}

	link := &model.Link{
		UserID:      userID,
		ShortCode:   shortCode,
		OriginalURL: input.OriginalURL,
		IsActive:    true,
	}

	if input.Title != "" {
		link.Title = &input.Title
	}

	if input.ExpiresAt != nil {
		link.ExpiresAt = sql.NullTime{Time: *input.ExpiresAt, Valid: true}
	}

	if err := s.linkRepo.Create(ctx, link); err != nil {
		return nil, err
	}

	return link, nil
}

func (s *LinkService) GetByID(ctx context.Context, userID, linkID uint64) (*model.Link, error) {
	link, err := s.linkRepo.GetByID(ctx, linkID)
	if errors.Is(err, repository.ErrLinkNotFound) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, err
	}

	if link.UserID != userID {
		return nil, ErrNotLinkOwner
	}

	return link, nil
}

func (s *LinkService) List(ctx context.Context, userID uint64, params ListLinksParams) (*ListLinksResult, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	offset := (params.Page - 1) * params.Limit

	links, err := s.linkRepo.ListByUserID(ctx, userID, params.Limit, offset)
	if err != nil {
		return nil, err
	}

	total, err := s.linkRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit > 0 {
		totalPages++
	}

	return &ListLinksResult{
		Links:      links,
		Total:      total,
		Page:       params.Page,
		TotalPages: totalPages,
	}, nil
}

func (s *LinkService) Update(ctx context.Context, userID, linkID uint64, input UpdateLinkInput) (*model.Link, error) {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return nil, err
	}

	if input.OriginalURL != "" {
		link.OriginalURL = input.OriginalURL
	}
	if input.Title != "" {
		link.Title = &input.Title
	}
	if input.ExpiresAt != nil {
		link.ExpiresAt = sql.NullTime{Time: *input.ExpiresAt, Valid: true}
	}
	if input.IsActive != nil {
		link.IsActive = *input.IsActive
	}

	if err := s.linkRepo.Update(ctx, link); err != nil {
		return nil, err
	}

	return link, nil
}

func (s *LinkService) Delete(ctx context.Context, userID, linkID uint64) error {
	link, err := s.GetByID(ctx, userID, linkID)
	if err != nil {
		return err
	}

	return s.linkRepo.Delete(ctx, link.ID)
}
```

**Step 2: Commit**

```bash
git add backend/
git commit -m "feat: add link service with CRUD operations"
```

---

### Task 11: Link Handlers

**Files:**
- Create: `backend/internal/handler/link.go`
- Modify: `backend/cmd/server/main.go`

**Step 1: Create link handlers**

```go
// backend/internal/handler/link.go
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jose/urlshortener/internal/config"
	"github.com/jose/urlshortener/internal/middleware"
	"github.com/jose/urlshortener/internal/service"
)

type LinkHandler struct {
	linkService *service.LinkService
	baseURL     string
}

func NewLinkHandler(linkService *service.LinkService, cfg *config.Config) *LinkHandler {
	return &LinkHandler{
		linkService: linkService,
		baseURL:     cfg.BaseURL,
	}
}

type linkResponse struct {
	*model.Link
	ShortURL string `json:"short_url"`
}

func (h *LinkHandler) toResponse(link *model.Link) linkResponse {
	return linkResponse{
		Link:     link,
		ShortURL: h.baseURL + "/" + link.ShortCode,
	}
}

func (h *LinkHandler) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var input service.CreateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.linkService.Create(c.Request.Context(), userID, input)
	if errors.Is(err, service.ErrInvalidShortCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid custom code"})
		return
	}
	if errors.Is(err, service.ErrShortCodeTaken) {
		c.JSON(http.StatusConflict, gin.H{"error": "short code already taken"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create link"})
		return
	}

	c.JSON(http.StatusCreated, h.toResponse(link))
}

func (h *LinkHandler) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	link, err := h.linkService.GetByID(c.Request.Context(), userID, linkID)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get link"})
		return
	}

	c.JSON(http.StatusOK, h.toResponse(link))
}

func (h *LinkHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	params := service.ListLinksParams{
		Page:  page,
		Limit: limit,
	}

	result, err := h.linkService.List(c.Request.Context(), userID, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list links"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *LinkHandler) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	var input service.UpdateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	link, err := h.linkService.Update(c.Request.Context(), userID, linkID, input)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update link"})
		return
	}

	c.JSON(http.StatusOK, h.toResponse(link))
}

func (h *LinkHandler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	linkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	err = h.linkService.Delete(c.Request.Context(), userID, linkID)
	if errors.Is(err, service.ErrLinkNotFound) || errors.Is(err, service.ErrNotLinkOwner) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete link"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
```

**Step 2: Add import for model package**

Add to the top of link.go:
```go
import (
	"github.com/jose/urlshortener/internal/model"
	// ... other imports
)
```

**Step 3: Wire up routes in main.go**

Add after auth routes:
```go
// Setup link services
linkRepo := repository.NewLinkRepository(db)
shortCodeSvc := service.NewShortCodeService(linkRepo)
linkService := service.NewLinkService(linkRepo, shortCodeSvc)

// Setup link handler
linkHandler := handler.NewLinkHandler(linkService, cfg)

// Link routes (protected)
links := api.Group("/links")
links.Use(middleware.AuthMiddleware(authService))
{
	links.POST("", linkHandler.Create)
	links.GET("", linkHandler.List)
	links.GET("/:id", linkHandler.Get)
	links.PUT("/:id", linkHandler.Update)
	links.DELETE("/:id", linkHandler.Delete)
}
```

**Step 4: Commit**

```bash
git add backend/
git commit -m "feat: add link CRUD handlers"
```

---

### Task 12: Redirect Handler with Redis Caching

**Files:**
- Create: `backend/internal/handler/redirect.go`
- Create: `backend/internal/service/redirect_service.go`
- Modify: `backend/cmd/server/main.go`

**Step 1: Create redirect service**

```go
// backend/internal/service/redirect_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/jose/urlshortener/internal/model"
	"github.com/jose/urlshortener/internal/repository"
)

var (
	ErrLinkExpired   = errors.New("link has expired")
	ErrLinkInactive  = errors.New("link is not active")
)

const linkCacheTTL = 1 * time.Hour

type RedirectService struct {
	linkRepo *repository.LinkRepository
	rdb      *redis.Client
}

func NewRedirectService(linkRepo *repository.LinkRepository, rdb *redis.Client) *RedirectService {
	return &RedirectService{
		linkRepo: linkRepo,
		rdb:      rdb,
	}
}

type cachedLink struct {
	OriginalURL string    `json:"url"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	IsActive    bool      `json:"is_active"`
	LinkID      uint64    `json:"link_id"`
}

func (s *RedirectService) Resolve(ctx context.Context, code string) (string, uint64, error) {
	// Try cache first
	cacheKey := "link:" + code
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var cl cachedLink
		if err := json.Unmarshal([]byte(cached), &cl); err == nil {
			return s.validateAndReturn(cl)
		}
	}

	// Cache miss - query DB
	link, err := s.linkRepo.GetByShortCode(ctx, code)
	if errors.Is(err, repository.ErrLinkNotFound) {
		return "", 0, ErrLinkNotFound
	}
	if err != nil {
		return "", 0, err
	}

	// Cache the result
	cl := cachedLink{
		OriginalURL: link.OriginalURL,
		IsActive:    link.IsActive,
		LinkID:      link.ID,
	}
	if link.ExpiresAt.Valid {
		cl.ExpiresAt = link.ExpiresAt.Time
	}

	data, _ := json.Marshal(cl)
	s.rdb.Set(ctx, cacheKey, data, linkCacheTTL)

	return s.validateAndReturn(cl)
}

func (s *RedirectService) validateAndReturn(cl cachedLink) (string, uint64, error) {
	if !cl.IsActive {
		return "", 0, ErrLinkInactive
	}
	if !cl.ExpiresAt.IsZero() && cl.ExpiresAt.Before(time.Now()) {
		return "", 0, ErrLinkExpired
	}
	return cl.OriginalURL, cl.LinkID, nil
}

func (s *RedirectService) InvalidateCache(ctx context.Context, code string) error {
	return s.rdb.Del(ctx, "link:"+code).Err()
}
```

**Step 2: Create redirect handler**

```go
// backend/internal/handler/redirect.go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jose/urlshortener/internal/service"
)

type RedirectHandler struct {
	redirectService *service.RedirectService
}

func NewRedirectHandler(redirectService *service.RedirectService) *RedirectHandler {
	return &RedirectHandler{redirectService: redirectService}
}

func (h *RedirectHandler) Redirect(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid short code"})
		return
	}

	originalURL, linkID, err := h.redirectService.Resolve(c.Request.Context(), code)
	if errors.Is(err, service.ErrLinkNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	if errors.Is(err, service.ErrLinkExpired) || errors.Is(err, service.ErrLinkInactive) {
		c.JSON(http.StatusGone, gin.H{"error": "link is no longer available"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve link"})
		return
	}

	// TODO: Log click event to Redis buffer (Task 13)
	_ = linkID

	c.Redirect(http.StatusFound, originalURL)
}
```

**Step 3: Wire up redirect route in main.go**

Add before API routes:
```go
// Redirect service and handler
redirectService := service.NewRedirectService(linkRepo, rdb)
redirectHandler := handler.NewRedirectHandler(redirectService)

// Redirect route (must be after API routes to avoid conflicts)
r.GET("/:code", redirectHandler.Redirect)
```

**Step 4: Test redirect**

```bash
# Create a link first via API, then:
curl -v http://localhost:8080/abc123
```

**Step 5: Commit**

```bash
git add backend/
git commit -m "feat: add redirect handler with Redis caching"
```

---

## Phase 3: Click Tracking

### Task 13: Click Event Buffer in Redis

**Files:**
- Modify: `backend/internal/handler/redirect.go`
- Create: `backend/internal/service/click_service.go`

**Step 1: Create click service**

```go
// backend/internal/service/click_service.go
package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type ClickEvent struct {
	LinkID     uint64    `json:"link_id"`
	ClickedAt  time.Time `json:"clicked_at"`
	IPHash     string    `json:"ip_hash"`
	UserAgent  string    `json:"user_agent"`
	Referrer   string    `json:"referrer"`
	UTMSource  string    `json:"utm_source,omitempty"`
	UTMMedium  string    `json:"utm_medium,omitempty"`
	UTMCampaign string   `json:"utm_campaign,omitempty"`
}

type ClickService struct {
	rdb *redis.Client
}

func NewClickService(rdb *redis.Client) *ClickService {
	return &ClickService{rdb: rdb}
}

func (s *ClickService) RecordClick(ctx context.Context, event ClickEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Buffer click event in Redis list
	bufferKey := "clicks:buffer"
	if err := s.rdb.LPush(ctx, bufferKey, data).Err(); err != nil {
		return err
	}

	// Increment real-time counter
	counterKey := "clicks:count:" + string(rune(event.LinkID))
	s.rdb.Incr(ctx, counterKey)

	return nil
}

func (s *ClickService) GetRealtimeCount(ctx context.Context, linkID uint64) (int64, error) {
	counterKey := "clicks:count:" + string(rune(linkID))
	return s.rdb.Get(ctx, counterKey).Int64()
}
```

**Step 2: Update redirect handler to record clicks**

Add to redirect.go after resolving the URL:
```go
// Record click event asynchronously
go func() {
	event := service.ClickEvent{
		LinkID:    linkID,
		ClickedAt: time.Now().UTC(),
		IPHash:    hashIP(c.ClientIP()),
		UserAgent: c.GetHeader("User-Agent"),
		Referrer:  c.GetHeader("Referer"),
		UTMSource: c.Query("utm_source"),
		UTMMedium: c.Query("utm_medium"),
		UTMCampaign: c.Query("utm_campaign"),
	}
	_ = h.clickService.RecordClick(context.Background(), event)
}()
```

**Step 3: Add IP hashing helper**

```go
func hashIP(ip string) string {
	h := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(h[:])
}
```

**Step 4: Update handler constructor**

```go
type RedirectHandler struct {
	redirectService *service.RedirectService
	clickService    *service.ClickService
}

func NewRedirectHandler(redirectService *service.RedirectService, clickService *service.ClickService) *RedirectHandler {
	return &RedirectHandler{
		redirectService: redirectService,
		clickService:    clickService,
	}
}
```

**Step 5: Commit**

```bash
git add backend/
git commit -m "feat: add click event buffering in Redis"
```

---

### Task 14: Click Flusher Worker

**Files:**
- Create: `backend/internal/worker/click_flusher.go`
- Create: `backend/internal/repository/click_repo.go`
- Modify: `backend/cmd/server/main.go`

**Step 1: Create click repository**

```go
// backend/internal/repository/click_repo.go
package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/jose/urlshortener/internal/model"
)

type ClickRepository struct {
	db *sqlx.DB
}

func NewClickRepository(db *sqlx.DB) *ClickRepository {
	return &ClickRepository{db: db}
}

func (r *ClickRepository) BatchInsert(ctx context.Context, clicks []model.Click) error {
	if len(clicks) == 0 {
		return nil
	}

	query := `INSERT INTO clicks (link_id, clicked_at, ip_hash, user_agent, referrer, country, city, device_type, browser, utm_source, utm_medium, utm_campaign)
			  VALUES (:link_id, :clicked_at, :ip_hash, :user_agent, :referrer, :country, :city, :device_type, :browser, :utm_source, :utm_medium, :utm_campaign)`

	_, err := r.db.NamedExecContext(ctx, query, clicks)
	return err
}

func (r *ClickRepository) GetTotalByLinkID(ctx context.Context, linkID uint64) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM clicks WHERE link_id = ?`
	err := r.db.GetContext(ctx, &count, query, linkID)
	return count, err
}
```

**Step 2: Create click model**

```go
// backend/internal/model/click.go
package model

import "time"

type Click struct {
	ID          uint64    `db:"id" json:"id"`
	LinkID      uint64    `db:"link_id" json:"link_id"`
	ClickedAt   time.Time `db:"clicked_at" json:"clicked_at"`
	IPHash      string    `db:"ip_hash" json:"-"`
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

**Step 3: Create click flusher worker**

```go
// backend/internal/worker/click_flusher.go
package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/jose/urlshortener/internal/model"
	"github.com/jose/urlshortener/internal/repository"
	"github.com/jose/urlshortener/internal/service"
	"github.com/jose/urlshortener/pkg/logger"
)

type ClickFlusher struct {
	rdb       *redis.Client
	clickRepo *repository.ClickRepository
	interval  time.Duration
	batchSize int
	stopCh    chan struct{}
}

func NewClickFlusher(rdb *redis.Client, clickRepo *repository.ClickRepository) *ClickFlusher {
	return &ClickFlusher{
		rdb:       rdb,
		clickRepo: clickRepo,
		interval:  30 * time.Second,
		batchSize: 100,
		stopCh:    make(chan struct{}),
	}
}

func (f *ClickFlusher) Start() {
	go f.run()
}

func (f *ClickFlusher) Stop() {
	close(f.stopCh)
}

func (f *ClickFlusher) run() {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.flush()
		case <-f.stopCh:
			f.flush() // Final flush before stopping
			return
		}
	}
}

func (f *ClickFlusher) flush() {
	ctx := context.Background()
	bufferKey := "clicks:buffer"

	for {
		// Get batch of events
		events, err := f.rdb.LRange(ctx, bufferKey, 0, int64(f.batchSize-1)).Result()
		if err != nil {
			logger.Log.Errorf("Failed to get click events from buffer: %v", err)
			return
		}

		if len(events) == 0 {
			return
		}

		// Parse events into clicks
		clicks := make([]model.Click, 0, len(events))
		for _, eventData := range events {
			var event service.ClickEvent
			if err := json.Unmarshal([]byte(eventData), &event); err != nil {
				logger.Log.Warnf("Failed to unmarshal click event: %v", err)
				continue
			}

			click := model.Click{
				LinkID:      event.LinkID,
				ClickedAt:   event.ClickedAt,
				IPHash:      event.IPHash,
				UserAgent:   event.UserAgent,
				Referrer:    event.Referrer,
				DeviceType:  "unknown", // TODO: Parse from user agent
				UTMSource:   event.UTMSource,
				UTMMedium:   event.UTMMedium,
				UTMCampaign: event.UTMCampaign,
			}
			clicks = append(clicks, click)
		}

		// Insert into database
		if err := f.clickRepo.BatchInsert(ctx, clicks); err != nil {
			logger.Log.Errorf("Failed to batch insert clicks: %v", err)
			return
		}

		// Remove processed events from buffer
		f.rdb.LTrim(ctx, bufferKey, int64(len(events)), -1)

		logger.Log.Infof("Flushed %d click events to database", len(clicks))

		if len(events) < f.batchSize {
			return // No more events to process
		}
	}
}
```

**Step 4: Start worker in main.go**

Add after service setup:
```go
// Start click flusher worker
clickRepo := repository.NewClickRepository(db)
clickFlusher := worker.NewClickFlusher(rdb, clickRepo)
clickFlusher.Start()
defer clickFlusher.Stop()
```

**Step 5: Commit**

```bash
git add backend/
git commit -m "feat: add click flusher background worker"
```

---

## Phase 4: Docker Setup

### Task 15: Dockerfile and Docker Compose

**Files:**
- Create: `backend/Dockerfile`
- Create: `docker-compose.yml`
- Create: `.env.example`

**Step 1: Create backend Dockerfile**

```dockerfile
# backend/Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./server"]
```

**Step 2: Create docker-compose.yml**

```yaml
# docker-compose.yml
version: '3.8'

services:
  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      - SERVER_PORT=8080
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

**Step 3: Create .env.example**

```
# .env.example
DB_PASSWORD=your_secure_password
DB_ROOT_PASSWORD=your_root_password
JWT_SECRET=your_jwt_secret_key
BASE_URL=http://localhost:8080
```

**Step 4: Commit**

```bash
git add backend/Dockerfile docker-compose.yml .env.example
git commit -m "feat: add Docker configuration"
```

---

## Phase 5: Frontend Foundation

### Task 16: Initialize Next.js Frontend

**Files:**
- Create: `frontend/` directory with Next.js app
- Create: `frontend/package.json`
- Create: `frontend/src/app/layout.tsx`
- Create: `frontend/src/app/page.tsx`

**Step 1: Initialize Next.js project**

```bash
npx create-next-app@latest frontend --typescript --tailwind --eslint --app --src-dir --import-alias "@/*"
```

**Step 2: Install dependencies**

```bash
cd frontend && npm install zustand axios @tanstack/react-query qrcode.react recharts
```

**Step 3: Install shadcn/ui**

```bash
cd frontend && npx shadcn-ui@latest init
```

Select: TypeScript, Default style, CSS variables

**Step 4: Add essential shadcn components**

```bash
cd frontend && npx shadcn-ui@latest add button card form input label toast dialog table
```

**Step 5: Commit**

```bash
git add frontend/
git commit -m "feat: initialize Next.js frontend with Tailwind and shadcn/ui"
```

---

### Task 17: API Client

**Files:**
- Create: `frontend/src/lib/api.ts`
- Create: `frontend/src/types/index.ts`

**Step 1: Create types**

```typescript
// frontend/src/types/index.ts
export interface User {
  id: number;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface Link {
  id: number;
  user_id: number;
  short_code: string;
  original_url: string;
  title?: string;
  expires_at?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  short_url: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface LinksListResponse {
  links: Link[];
  total: number;
  page: number;
  total_pages: number;
}
```

**Step 2: Create API client**

```typescript
// frontend/src/lib/api.ts
const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

class ApiClient {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || 'Request failed');
    }

    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // Auth
  async register(email: string, password: string) {
    return this.request<AuthResponse>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async login(email: string, password: string) {
    return this.request<AuthResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async me() {
    return this.request<User>('/api/auth/me');
  }

  // Links
  async getLinks(page = 1, limit = 20) {
    return this.request<LinksListResponse>(`/api/links?page=${page}&limit=${limit}`);
  }

  async createLink(data: {
    original_url: string;
    custom_code?: string;
    title?: string;
    expires_at?: string;
  }) {
    return this.request<Link>('/api/links', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async getLink(id: number) {
    return this.request<Link>(`/api/links/${id}`);
  }

  async updateLink(id: number, data: Partial<Link>) {
    return this.request<Link>(`/api/links/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteLink(id: number) {
    return this.request<void>(`/api/links/${id}`, {
      method: 'DELETE',
    });
  }
}

export const api = new ApiClient();
```

**Step 3: Commit**

```bash
git add frontend/
git commit -m "feat: add API client and types"
```

---

### Task 18: Auth Store with Zustand

**Files:**
- Create: `frontend/src/stores/auth.ts`

**Step 1: Create auth store**

```typescript
// frontend/src/stores/auth.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { api } from '@/lib/api';
import type { User } from '@/types';

interface AuthState {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
  checkAuth: () => Promise<void>;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      isLoading: true,

      login: async (email: string, password: string) => {
        const response = await api.login(email, password);
        api.setToken(response.token);
        set({ user: response.user, token: response.token });
      },

      register: async (email: string, password: string) => {
        const response = await api.register(email, password);
        api.setToken(response.token);
        set({ user: response.user, token: response.token });
      },

      logout: () => {
        api.setToken(null);
        set({ user: null, token: null });
      },

      checkAuth: async () => {
        const { token } = get();
        if (!token) {
          set({ isLoading: false });
          return;
        }

        api.setToken(token);
        try {
          const user = await api.me();
          set({ user, isLoading: false });
        } catch {
          set({ user: null, token: null, isLoading: false });
        }
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({ token: state.token }),
    }
  )
);
```

**Step 2: Commit**

```bash
git add frontend/
git commit -m "feat: add Zustand auth store with persistence"
```

---

### Task 19: Login Page

**Files:**
- Create: `frontend/src/app/(auth)/login/page.tsx`
- Create: `frontend/src/app/(auth)/layout.tsx`

**Step 1: Create auth layout**

```typescript
// frontend/src/app/(auth)/layout.tsx
export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full space-y-8 p-8">
        {children}
      </div>
    </div>
  );
}
```

**Step 2: Create login page**

```typescript
// frontend/src/app/(auth)/login/page.tsx
'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuthStore } from '@/stores/auth';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const login = useAuthStore((state) => state.login);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await login(email, password);
      router.push('/dashboard');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Login</CardTitle>
        <CardDescription>Enter your credentials to access your account</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="p-3 text-sm text-red-500 bg-red-50 rounded-md">
              {error}
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? 'Logging in...' : 'Login'}
          </Button>
        </form>
        <p className="mt-4 text-center text-sm text-gray-600">
          Don&apos;t have an account?{' '}
          <Link href="/register" className="text-blue-600 hover:underline">
            Register
          </Link>
        </p>
      </CardContent>
    </Card>
  );
}
```

**Step 3: Commit**

```bash
git add frontend/
git commit -m "feat: add login page"
```

---

### Task 20: Dashboard Layout and Link List

**Files:**
- Create: `frontend/src/app/dashboard/layout.tsx`
- Create: `frontend/src/app/dashboard/page.tsx`
- Create: `frontend/src/components/links/LinkTable.tsx`

**Step 1: Create dashboard layout**

```typescript
// frontend/src/app/dashboard/layout.tsx
'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { useAuthStore } from '@/stores/auth';

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const { user, isLoading, checkAuth, logout } = useAuthStore();

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  useEffect(() => {
    if (!isLoading && !user) {
      router.push('/login');
    }
  }, [isLoading, user, router]);

  if (isLoading) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  if (!user) {
    return null;
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <Link href="/dashboard" className="text-xl font-bold">
                URL Shortener
              </Link>
            </div>
            <div className="flex items-center gap-4">
              <span className="text-sm text-gray-600">{user.email}</span>
              <Button variant="outline" onClick={logout}>
                Logout
              </Button>
            </div>
          </div>
        </div>
      </nav>
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>
    </div>
  );
}
```

**Step 2: Create link table component**

```typescript
// frontend/src/components/links/LinkTable.tsx
'use client';

import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import type { Link } from '@/types';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Button } from '@/components/ui/button';

export function LinkTable() {
  const [links, setLinks] = useState<Link[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadLinks();
  }, []);

  const loadLinks = async () => {
    try {
      const data = await api.getLinks();
      setLinks(data.links || []);
    } catch (err) {
      console.error('Failed to load links:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this link?')) return;

    try {
      await api.deleteLink(id);
      setLinks(links.filter((l) => l.id !== id));
    } catch (err) {
      console.error('Failed to delete link:', err);
    }
  };

  const copyToClipboard = (url: string) => {
    navigator.clipboard.writeText(url);
  };

  if (loading) {
    return <div>Loading links...</div>;
  }

  if (links.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        No links yet. Create your first short link!
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Short URL</TableHead>
          <TableHead>Original URL</TableHead>
          <TableHead>Created</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {links.map((link) => (
          <TableRow key={link.id}>
            <TableCell>
              <div className="flex items-center gap-2">
                <a
                  href={link.short_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:underline"
                >
                  {link.short_code}
                </a>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => copyToClipboard(link.short_url)}
                >
                  Copy
                </Button>
              </div>
            </TableCell>
            <TableCell className="max-w-xs truncate">
              {link.original_url}
            </TableCell>
            <TableCell>
              {new Date(link.created_at).toLocaleDateString()}
            </TableCell>
            <TableCell>
              <span
                className={`px-2 py-1 rounded-full text-xs ${
                  link.is_active
                    ? 'bg-green-100 text-green-800'
                    : 'bg-red-100 text-red-800'
                }`}
              >
                {link.is_active ? 'Active' : 'Inactive'}
              </span>
            </TableCell>
            <TableCell>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => handleDelete(link.id)}
              >
                Delete
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
```

**Step 3: Create dashboard page**

```typescript
// frontend/src/app/dashboard/page.tsx
'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { LinkTable } from '@/components/links/LinkTable';
import { api } from '@/lib/api';

export default function DashboardPage() {
  const [url, setUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleQuickCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await api.createLink({ original_url: url });
      setUrl('');
      // Trigger table refresh
      window.location.reload();
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
          <CardTitle>Quick Create</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleQuickCreate} className="flex gap-4">
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
          </form>
          {error && (
            <p className="mt-2 text-sm text-red-500">{error}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Your Links</CardTitle>
        </CardHeader>
        <CardContent>
          <LinkTable />
        </CardContent>
      </Card>
    </div>
  );
}
```

**Step 4: Commit**

```bash
git add frontend/
git commit -m "feat: add dashboard with link table and quick create"
```

---

## Summary

This plan covers the MVP implementation:

**Phase 1 (Tasks 1-7):** Backend foundation - Go project, database, Redis, auth
**Phase 2 (Tasks 8-12):** Link management - models, services, handlers, redirect
**Phase 3 (Tasks 13-14):** Click tracking - Redis buffer, background flusher
**Phase 4 (Task 15):** Docker setup
**Phase 5 (Tasks 16-20):** Frontend foundation - Next.js, auth, dashboard

**Not included in MVP (future phases):**
- Analytics visualization (charts, geo maps)
- Custom domain management
- API keys
- QR code generation
- Bulk operations
- User-agent parsing for device detection
- GeoIP lookup

---

Plan complete and saved to `docs/plans/2026-01-31-url-shortener-mvp.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?

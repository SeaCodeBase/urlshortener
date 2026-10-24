# Backend Error Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add comprehensive structured logging to all error handling paths in the Go backend using JSON format for better observability and log aggregation.

**Architecture:** Create a context-aware logger wrapper around `*zap.Logger`. Automatically extract request_id, trace_id, user_id from context. Use JSON encoding for all environments. Log at appropriate levels with structured fields: `Error` for unexpected errors, `Warn` for expected error conditions.

**Tech Stack:** Zap Logger (structured, JSON output, context-aware wrapper)

---

## Task 0: Refactor Logger Package with Context Support

**Files:**
- Modify: `backend/pkg/logger/zap.go`

**Step 1: Update logger to use structured logging with context wrapper**

Replace the entire file with:

```go
// backend/pkg/logger/zap.go
package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Context keys for logging
type ctxKey string

const (
	RequestIDKey ctxKey = "request_id"
	TraceIDKey   ctxKey = "trace_id"
	UserIDKey    ctxKey = "user_id"
)

var log *zap.Logger

func Init(isDev bool) {
	var cfg zap.Config
	if isDev {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewProductionConfig()
	}
	// Always use JSON encoding for structured logs
	cfg.Encoding = "json"

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	log = logger
}

func Sync() {
	if log != nil {
		_ = log.Sync()
	}
}

// WithContext adds context fields to a new logger instance
func WithContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return log
	}

	fields := make([]zap.Field, 0, 3)

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if userID, ok := ctx.Value(UserIDKey).(uint64); ok && userID != 0 {
		fields = append(fields, zap.Uint64("user_id", userID))
	}

	if len(fields) == 0 {
		return log
	}
	return log.With(fields...)
}

// Helper functions that accept context

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Info(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Warn(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Error(msg, fields...)
}

func Debug(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Debug(msg, fields...)
}

func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	WithContext(ctx).Fatal(msg, fields...)
}

// Raw returns the underlying zap.Logger for cases without context
func Raw() *zap.Logger {
	return log
}
```

**Step 2: Run build to identify all places that need updates**

Run: `cd backend && go build ./... 2>&1 | head -50`
Expected: Build errors showing places using old logger methods

**Step 3: Commit**

```bash
git add backend/pkg/logger/zap.go
git commit -m "refactor(backend): add context-aware structured JSON logger"
```

---

## Task 1: Update Click Flusher Worker (existing logging)

**Files:**
- Modify: `backend/internal/worker/click_flusher.go`

**Step 1: Update logging calls to use context-aware functions**

Change from:
```go
logger.Log.Errorf("Failed to get click events from buffer: %v", err)
```

To:
```go
logger.Error(ctx, "failed to get click events from buffer",
	zap.Error(err),
)
```

Update all logging calls in the file:

```go
// Line ~71
logger.Error(ctx, "failed to get click events from buffer",
	zap.Error(err),
)

// Line ~84
logger.Warn(ctx, "failed to unmarshal click event",
	zap.Error(err),
)

// Line ~114
logger.Error(ctx, "failed to batch insert clicks",
	zap.Error(err),
)

// Line ~120
logger.Error(ctx, "failed to trim click buffer",
	zap.Error(err),
)

// Line ~123
logger.Info(ctx, "flushed click events to database",
	zap.Int("count", len(clicks)),
)
```

**Step 2: Add zap import**

Add `"go.uber.org/zap"` to imports.

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/worker/click_flusher.go
git commit -m "refactor(backend): update click flusher to context-aware logging"
```

---

## Task 2: Update Existing Logging in Services

**Files:**
- Modify: `backend/internal/service/redirect_service.go`
- Modify: `backend/internal/service/click_service.go`
- Modify: `backend/internal/handler/redirect.go`

**Step 1: Update redirect_service.go**

Add `"go.uber.org/zap"` to imports.

Change:
```go
logger.Log.Warnf("Failed to marshal cached link: %v", err)
```
To:
```go
logger.Warn(ctx, "failed to marshal cached link",
	zap.Uint64("link_id", link.ID),
	zap.Error(err),
)
```

**Step 2: Update click_service.go**

Add `"go.uber.org/zap"` to imports.

Change:
```go
logger.Log.Warnf("Failed to increment click counter: %v", err)
```
To:
```go
logger.Warn(ctx, "failed to increment click counter",
	zap.Uint64("link_id", linkID),
	zap.Error(err),
)
```

**Step 3: Update redirect.go handler**

Add `"go.uber.org/zap"` to imports.

Change:
```go
logger.Log.Warnf("Failed to record click: %v", err)
```
To:
```go
logger.Warn(ctx, "failed to record click",
	zap.Uint64("link_id", link.ID),
	zap.Error(err),
)
```

Note: In handler, use `c.Request.Context()` to get context.

**Step 4: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add backend/internal/service/redirect_service.go backend/internal/service/click_service.go backend/internal/handler/redirect.go
git commit -m "refactor(backend): update existing logging to context-aware format"
```

---

## Task 3: Add Logging to Auth Handler

**Files:**
- Modify: `backend/internal/handler/auth.go`

**Step 1: Add imports**

```go
import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)
```

**Step 2: Add logging to Register handler**

```go
func (h *AuthHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()
	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "register: invalid request body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Register(ctx, input)
	if errors.Is(err, service.ErrEmailTaken) {
		logger.Warn(ctx, "register: email already taken",
			zap.String("email", input.Email),
		)
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}
	if err != nil {
		logger.Error(ctx, "register: failed to register user",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}
```

**Step 3: Add logging to Login handler**

```go
func (h *AuthHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "login: invalid request body",
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Login(ctx, input)
	if errors.Is(err, service.ErrInvalidCredentials) {
		logger.Warn(ctx, "login: invalid credentials",
			zap.String("email", input.Email),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		logger.Error(ctx, "login: failed to authenticate",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		return
	}

	hasPasskeys, err := h.passkeyService.HasPasskeys(ctx, resp.User.ID)
	if err != nil {
		logger.Error(ctx, "login: failed to check passkeys",
			zap.Uint64("user_id", resp.User.ID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check passkeys"})
		return
	}

	if hasPasskeys {
		c.JSON(http.StatusOK, LoginResponse{
			RequiresPasskey: true,
			UserID:          resp.User.ID,
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}
```

**Step 4: Add logging to Me handler**

```go
func (h *AuthHandler) Me(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "me: failed to get user",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	c.JSON(http.StatusOK, user)
}
```

**Step 5: Add logging to ChangePassword handler**

```go
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)

	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Warn(ctx, "change-password: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.authService.ChangePassword(ctx, userID, input)
	if errors.Is(err, service.ErrWrongPassword) {
		logger.Warn(ctx, "change-password: wrong current password",
			zap.Uint64("user_id", userID),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	if err != nil {
		logger.Error(ctx, "change-password: failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}
```

**Step 6: Add logging to UpdateMe handler**

```go
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warn(ctx, "update-me: invalid request body",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.authService.UpdateDisplayName(ctx, userID, req.DisplayName); err != nil {
		logger.Error(ctx, "update-me: failed to update display name",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}
	user, err := h.authService.GetUserByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "update-me: failed to get user after update",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}
	c.JSON(http.StatusOK, user)
}
```

**Step 7: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 8: Commit**

```bash
git add backend/internal/handler/auth.go
git commit -m "feat(backend): add context-aware error logging to auth handler"
```

---

## Task 4: Add Logging to Link Handler

**Files:**
- Modify: `backend/internal/handler/link.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all error paths**

Pattern for each error (use `ctx := c.Request.Context()` at start of handler):
```go
// JSON binding errors
logger.Warn(ctx, "create-link: invalid request body",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)

// Service errors
logger.Error(ctx, "create-link: failed",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)

// Not found errors
logger.Warn(ctx, "get-link: not found",
	zap.Uint64("link_id", linkID),
	zap.Uint64("user_id", userID),
)
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/handler/link.go
git commit -m "feat(backend): add context-aware error logging to link handler"
```

---

## Task 5: Add Logging to Passkey Handlers

**Files:**
- Modify: `backend/internal/handler/passkey.go`
- Modify: `backend/internal/handler/passkey_verify.go`

**Step 1: Add imports to both files**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to passkey.go error paths**

Use `ctx := c.Request.Context()` at start of each handler:
```go
// Body read errors
logger.Warn(ctx, "passkey-register: failed to read request body",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)

// Service errors
logger.Error(ctx, "passkey-register: failed",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)
```

**Step 3: Add logging to passkey_verify.go error paths**

```go
// Verification errors
logger.Error(ctx, "passkey-verify: failed",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)
```

**Step 4: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add backend/internal/handler/passkey.go backend/internal/handler/passkey_verify.go
git commit -m "feat(backend): add context-aware error logging to passkey handlers"
```

---

## Task 6: Add Logging to Stats Handler

**Files:**
- Modify: `backend/internal/handler/stats.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to error paths**

Use `ctx := c.Request.Context()` at start of handler:
```go
// ID parse error
logger.Warn(ctx, "stats: invalid link ID",
	zap.String("link_id_param", c.Param("id")),
	zap.Error(err),
)

// Service error
logger.Error(ctx, "stats: failed to get stats",
	zap.Uint64("link_id", linkID),
	zap.Error(err),
)
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/handler/stats.go
git commit -m "feat(backend): add context-aware error logging to stats handler"
```

---

## Task 7: Add Logging to Auth Middleware

**Files:**
- Modify: `backend/internal/middleware/auth.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to token validation error**

```go
ctx := c.Request.Context()
userID, err := authService.ValidateToken(token)
if err != nil {
	logger.Warn(ctx, "auth: invalid or expired token",
		zap.Error(err),
	)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
	return
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/middleware/auth.go
git commit -m "feat(backend): add context-aware error logging to auth middleware"
```

---

## Task 8: Add Logging to Auth Service

**Files:**
- Modify: `backend/internal/service/auth_service.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to Register method**

Service methods already receive `ctx`, so use it directly:

```go
func (s *AuthServiceImpl) Register(ctx context.Context, input RegisterInput) (*AuthResponse, error) {
	exists, err := s.userRepo.EmailExists(ctx, input.Email)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to check email existence",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to hash password",
			zap.Error(err),
		)
		return nil, err
	}

	user := &model.User{
		Email:        input.Email,
		PasswordHash: string(hash),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logger.Error(ctx, "auth-service: failed to create user",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		return nil, err
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to generate token",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}
```

**Step 3: Add logging to Login method**

```go
func (s *AuthServiceImpl) Login(ctx context.Context, input LoginInput) (*AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		logger.Error(ctx, "auth-service: failed to get user by email",
			zap.String("email", input.Email),
			zap.Error(err),
		)
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to generate token",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}

	return &AuthResponse{Token: token, User: user}, nil
}
```

**Step 4: Add logging to ChangePassword method**

```go
func (s *AuthServiceImpl) ChangePassword(ctx context.Context, userID uint64, input ChangePasswordInput) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to get user for password change",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.OldPassword)); err != nil {
		return ErrWrongPassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error(ctx, "auth-service: failed to hash new password",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	return s.userRepo.UpdatePassword(ctx, userID, string(newHash))
}
```

**Step 5: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add backend/internal/service/auth_service.go
git commit -m "feat(backend): add context-aware error logging to auth service"
```

---

## Task 9: Add Logging to Link Service

**Files:**
- Modify: `backend/internal/service/link_service.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all error paths**

Pattern (service methods already have ctx parameter):
```go
if err != nil {
	logger.Error(ctx, "link-service: failed to create link",
		zap.Uint64("user_id", userID),
		zap.Error(err),
	)
	return nil, err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/service/link_service.go
git commit -m "feat(backend): add context-aware error logging to link service"
```

---

## Task 10: Add Logging to Passkey Service

**Files:**
- Modify: `backend/internal/service/passkey_service.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all error paths (~23 locations)**

Pattern (service methods already have ctx parameter):
```go
// User retrieval errors
logger.Error(ctx, "passkey-service: failed to get user",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)

// WebAuthn errors
logger.Error(ctx, "passkey-service: WebAuthn operation failed",
	zap.String("operation", "BeginRegistration"),
	zap.Uint64("user_id", userID),
	zap.Error(err),
)

// Credential storage errors
logger.Error(ctx, "passkey-service: failed to store credential",
	zap.Uint64("user_id", userID),
	zap.Error(err),
)
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/service/passkey_service.go
git commit -m "feat(backend): add context-aware error logging to passkey service"
```

---

## Task 11: Add Logging to Stats Service

**Files:**
- Modify: `backend/internal/service/stats_service.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all error paths**

Pattern (service methods already have ctx parameter):
```go
if err != nil {
	logger.Error(ctx, "stats-service: failed to get click stats",
		zap.Uint64("link_id", linkID),
		zap.Error(err),
	)
	return nil, err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/service/stats_service.go
git commit -m "feat(backend): add context-aware error logging to stats service"
```

---

## Task 12: Add Logging to Click Service

**Files:**
- Modify: `backend/internal/service/click_service.go`

**Step 1: Verify imports**

Ensure `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` are in imports.

**Step 2: Add logging to RecordClick method**

```go
func (s *ClickServiceImpl) RecordClick(ctx context.Context, event ClickEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		logger.Error(ctx, "click-service: failed to marshal click event",
			zap.Uint64("link_id", event.LinkID),
			zap.Error(err),
		)
		return err
	}

	if err := s.rdb.RPush(ctx, "clicks:buffer", data).Err(); err != nil {
		logger.Error(ctx, "click-service: failed to push click event to Redis",
			zap.Uint64("link_id", event.LinkID),
			zap.Error(err),
		)
		return err
	}

	return nil
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/service/click_service.go
git commit -m "feat(backend): add context-aware error logging to click service"
```

---

## Task 13: Add Logging to ShortCode Service

**Files:**
- Modify: `backend/internal/service/shortcode_service.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to error paths**

```go
// Crypto error
if err != nil {
	logger.Error(ctx, "shortcode-service: failed to generate random bytes",
		zap.Error(err),
	)
	return "", err
}

// Availability check error
if err != nil {
	logger.Error(ctx, "shortcode-service: failed to check code availability",
		zap.String("code", code),
		zap.Error(err),
	)
	return "", err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/service/shortcode_service.go
git commit -m "feat(backend): add context-aware error logging to shortcode service"
```

---

## Task 14: Add Logging to User Repository

**Files:**
- Modify: `backend/internal/repository/user_repo.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to database error paths**

Repository methods already have ctx parameter:

```go
// In Create method
if err != nil {
	if strings.Contains(err.Error(), "Duplicate entry") {
		return ErrEmailExists
	}
	logger.Error(ctx, "user-repo: failed to create user",
		zap.String("email", user.Email),
		zap.Error(err),
	)
	return err
}

// In GetByEmail method
if err != nil {
	logger.Error(ctx, "user-repo: failed to get user by email",
		zap.String("email", email),
		zap.Error(err),
	)
	return nil, err
}

// In GetByID method
if err != nil {
	logger.Error(ctx, "user-repo: failed to get user by ID",
		zap.Uint64("user_id", id),
		zap.Error(err),
	)
	return nil, err
}

// In UpdatePassword method
if err != nil {
	logger.Error(ctx, "user-repo: failed to update password",
		zap.Uint64("user_id", userID),
		zap.Error(err),
	)
	return err
}

// In UpdateDisplayName method
if err != nil {
	logger.Error(ctx, "user-repo: failed to update display name",
		zap.Uint64("user_id", userID),
		zap.Error(err),
	)
	return err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/repository/user_repo.go
git commit -m "feat(backend): add context-aware error logging to user repository"
```

---

## Task 15: Add Logging to Link Repository

**Files:**
- Modify: `backend/internal/repository/link_repo.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all database error paths**

Pattern (repository methods already have ctx parameter):
```go
if err != nil {
	logger.Error(ctx, "link-repo: failed to create link",
		zap.Uint64("user_id", link.UserID),
		zap.Error(err),
	)
	return err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/repository/link_repo.go
git commit -m "feat(backend): add context-aware error logging to link repository"
```

---

## Task 16: Add Logging to Click Repository

**Files:**
- Modify: `backend/internal/repository/click_repo.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all database error paths**

Pattern (repository methods already have ctx parameter):
```go
if err != nil {
	logger.Error(ctx, "click-repo: failed to batch insert clicks",
		zap.Int("count", len(clicks)),
		zap.Error(err),
	)
	return err
}

if err != nil {
	logger.Error(ctx, "click-repo: failed to get clicks by day",
		zap.Uint64("link_id", linkID),
		zap.Error(err),
	)
	return nil, err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/repository/click_repo.go
git commit -m "feat(backend): add context-aware error logging to click repository"
```

---

## Task 17: Add Logging to Passkey Repository

**Files:**
- Modify: `backend/internal/repository/passkey_repo.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to all database error paths**

Pattern (repository methods already have ctx parameter):
```go
if err != nil {
	logger.Error(ctx, "passkey-repo: failed to create passkey",
		zap.Uint64("user_id", passkey.UserID),
		zap.Error(err),
	)
	return err
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/repository/passkey_repo.go
git commit -m "feat(backend): add context-aware error logging to passkey repository"
```

---

## Task 18: Add Logging to GeoIP Utility

**Files:**
- Modify: `backend/internal/util/geoip.go`

**Step 1: Add imports**

Add `"github.com/SeaCodeBase/urlshortener/pkg/logger"` and `"go.uber.org/zap"` to imports.

**Step 2: Add logging to error paths**

Note: GeoIP utility functions may not have context. Use `logger.Raw()` for these cases:

```go
// Initialization error (no context available)
if err != nil {
	logger.Raw().Error("geoip: failed to open database",
		zap.String("path", path),
		zap.Error(err),
	)
	return
}

// Lookup error (no context available)
if err != nil {
	logger.Raw().Warn("geoip: failed to lookup IP",
		zap.String("ip", ip),
		zap.Error(err),
	)
	return GeoResult{}
}
```

**Step 3: Run build**

Run: `cd backend && go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add backend/internal/util/geoip.go
git commit -m "feat(backend): add error logging to geoip utility"
```

---

## Task 19: Final Verification

**Step 1: Run full build**

Run: `cd backend && go build ./...`
Expected: Build succeeds with no errors

**Step 2: Run all tests**

Run: `cd backend && go test ./...`
Expected: All tests pass

**Step 3: Verify JSON output**

Run: `cd backend && go run cmd/server/main.go 2>&1 | head -5`
Expected: See JSON formatted log output like:
```json
{"level":"info","ts":"2026-02-03T...","msg":"..."}
```

**Step 4: Create final commit if any files were missed**

```bash
git add -A
git commit -m "feat(backend): complete structured error logging implementation"
```

---

## Summary

This plan:

1. **Refactors logger** to use context-aware wrapper around `*zap.Logger` with JSON encoding

2. **Context support features**:
   - Automatic extraction of `request_id`, `trace_id`, `user_id` from context
   - Helper functions: `logger.Info(ctx, ...)`, `logger.Warn(ctx, ...)`, `logger.Error(ctx, ...)`
   - Fallback `logger.Raw()` for code without context

3. **Adds structured logging** to:
   - 6 handlers (auth, link, passkey, passkey_verify, stats, redirect)
   - 1 middleware (auth)
   - 6 services (auth, link, passkey, stats, click, shortcode)
   - 4 repositories (user, link, click, passkey)
   - 1 utility (geoip)

4. **Logging format changes**:
   - From: `logger.Log.Errorf("failed to do X: %v", err)`
   - To: `logger.Error(ctx, "failed to do X", zap.Uint64("user_id", id), zap.Error(err))`

5. **Output format**: JSON for all environments (dev and prod)

Example output:
```json
{"level":"error","ts":"2026-02-03T10:15:30.123Z","msg":"auth-service: failed to create user","request_id":"abc123","user_id":42,"email":"test@example.com","error":"connection refused"}
```

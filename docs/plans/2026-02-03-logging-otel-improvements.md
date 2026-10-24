# Logging & OpenTelemetry Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix caller stack trace in logs, add request/response logging middleware, integrate OpenTelemetry tracing with Gin, and include trace_id/span_id in all log output.

**Architecture:** Add `AddCallerSkip(1)` to zap logger initialization. Create a new LogMiddleware that captures URL, method, IP, latency, request body, and response body. Integrate `otelgin` middleware for distributed tracing. Modify the logger to extract trace_id and span_id from the OpenTelemetry span context.

**Tech Stack:** go.uber.org/zap, go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin, go.opentelemetry.io/otel/sdk

---

## Task 1: Fix Caller Skip in Logger

**Files:**
- Modify: `backend/pkg/logger/zap.go:33`

**Step 1: Update logger build to skip caller frames**

The current logger shows `zap.go:87` as caller because the wrapper functions add an extra stack frame. Fix by adding `AddCallerSkip(1)`.

```go
// In Init() function, change:
logger, err := cfg.Build()

// To:
logger, err := cfg.Build(zap.AddCallerSkip(1))
```

**Step 2: Run existing tests to verify no regression**

Run: `cd backend && go test ./pkg/logger/... -v`
Expected: PASS (or no tests exist yet)

**Step 3: Manual verification**

Run the server and trigger an error to verify caller now shows handler/service location instead of zap.go.

**Step 4: Commit**

```bash
git add backend/pkg/logger/zap.go
git commit -m "fix(logger): add caller skip to show correct call site in logs"
```

---

## Task 2: Add OpenTelemetry Dependencies

**Files:**
- Modify: `backend/go.mod`

**Step 1: Add OpenTelemetry dependencies**

Run:
```bash
cd backend && go get go.opentelemetry.io/otel@latest \
  go.opentelemetry.io/otel/sdk@latest \
  go.opentelemetry.io/otel/trace@latest \
  go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin@latest
```

**Step 2: Tidy dependencies**

Run: `cd backend && go mod tidy`
Expected: go.mod and go.sum updated with OpenTelemetry packages

**Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore(deps): add OpenTelemetry dependencies for tracing"
```

---

## Task 3: Create OpenTelemetry Initialization

**Files:**
- Create: `backend/pkg/telemetry/otel.go`

**Step 1: Create telemetry package with initialization function**

Create file `backend/pkg/telemetry/otel.go`:

```go
// backend/pkg/telemetry/otel.go
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init initializes OpenTelemetry trace provider without an exporter.
// Spans are created for trace_id/span_id in logs but not exported.
// Returns a shutdown function that should be deferred.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create trace provider without exporter (spans created but not exported)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
```

**Step 2: Verify compilation**

Run: `cd backend && go build ./pkg/telemetry/...`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/pkg/telemetry/otel.go
git commit -m "feat(telemetry): add OpenTelemetry initialization with stdout exporter"
```

---

## Task 4: Update Logger to Extract Trace/Span IDs from OpenTelemetry

**Files:**
- Modify: `backend/pkg/logger/zap.go`

**Step 1: Add OpenTelemetry trace import and update WithContext**

Update the imports and `WithContext` function to extract trace_id and span_id from the OpenTelemetry span context:

```go
// backend/pkg/logger/zap.go
package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Context keys for logging
type ctxKey string

const (
	RequestIDKey ctxKey = "request_id"
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

	logger, err := cfg.Build(zap.AddCallerSkip(1))
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

// WithContext adds context fields to a new logger instance.
// Extracts trace_id and span_id from OpenTelemetry span context if present.
func WithContext(ctx context.Context) *zap.Logger {
	l := log
	if l == nil {
		// Return no-op logger if not initialized (e.g., during tests)
		l = zap.NewNop()
	}

	if ctx == nil {
		return l
	}

	fields := make([]zap.Field, 0, 4)

	// Extract trace_id and span_id from OpenTelemetry span context
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		fields = append(fields, zap.String("trace_id", spanCtx.TraceID().String()))
		fields = append(fields, zap.String("span_id", spanCtx.SpanID().String()))
	}

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if userID, ok := ctx.Value(UserIDKey).(uint64); ok && userID != 0 {
		fields = append(fields, zap.Uint64("user_id", userID))
	}

	if len(fields) == 0 {
		return l
	}
	return l.With(fields...)
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
```

Note: Removed the unused `TraceIDKey` constant since we now extract trace_id directly from OpenTelemetry span context.

**Step 2: Verify compilation**

Run: `cd backend && go build ./pkg/logger/...`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/pkg/logger/zap.go
git commit -m "feat(logger): extract trace_id and span_id from OpenTelemetry span context"
```

---

## Task 5: Create Request/Response Logging Middleware

**Files:**
- Create: `backend/internal/middleware/logging.go`

**Step 1: Create response body capture wrapper**

Create file `backend/internal/middleware/logging.go`:

```go
// backend/internal/middleware/logging.go
package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// responseWriter wraps gin.ResponseWriter to capture the response body
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// LogMiddleware logs request and response details including:
// URL, method, IP, latency, request body, response body, status code
func LogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ctx := c.Request.Context()

		// Capture request body (only for non-GET requests to avoid logging large bodies)
		var requestBody string
		if c.Request.Method != "GET" && c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				requestBody = string(bodyBytes)
				// Restore body for downstream handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Wrap response writer to capture body
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = rw

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		responseBody := rw.body.String()

		// Build log fields
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("ip", c.ClientIP()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.Int("response_size", rw.body.Len()),
		}

		// Add request body for non-GET requests
		if requestBody != "" {
			fields = append(fields, zap.String("request_body", requestBody))
		}

		// Add response body
		fields = append(fields, zap.String("response_body", responseBody))

		logger.Info(ctx, "http request", fields...)
	}
}
```

**Step 2: Verify compilation**

Run: `cd backend && go build ./internal/middleware/...`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/internal/middleware/logging.go
git commit -m "feat(middleware): add request/response logging middleware"
```

---

## Task 6: Integrate OpenTelemetry and Logging Middleware in Main

**Files:**
- Modify: `backend/cmd/server/main.go`

**Step 1: Initialize OpenTelemetry and add middleware**

Update `main.go` to:
1. Initialize OpenTelemetry tracing
2. Add `otelgin` middleware for automatic span creation
3. Add `LogMiddleware` for request/response logging

```go
// backend/cmd/server/main.go
package main

import (
	"context"
	"sync"

	"github.com/SeaCodeBase/urlshortener/internal/cache"
	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/database"
	"github.com/SeaCodeBase/urlshortener/internal/handler"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/util"
	"github.com/SeaCodeBase/urlshortener/internal/worker"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/SeaCodeBase/urlshortener/pkg/telemetry"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		logger.Init(true)
		logger.Fatal(ctx, "configuration error", zap.Error(err))
	}
	logger.Init(true)
	defer logger.Sync()

	// Initialize OpenTelemetry
	shutdown, err := telemetry.Init(ctx, "urlshortener")
	if err != nil {
		logger.Fatal(ctx, "failed to initialize telemetry", zap.Error(err))
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			logger.Error(ctx, "failed to shutdown telemetry", zap.Error(err))
		}
	}()

	// Initialize GeoIP (optional - logs warning if path invalid)
	if err := util.InitGeoIP(ctx, cfg.GeoIP.Path); err != nil {
		logger.Warn(ctx, "geoip initialization failed - location lookups disabled",
			zap.String("path", cfg.GeoIP.Path),
			zap.Error(err),
		)
	}
	defer util.CloseGeoIP()

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		logger.Fatal(ctx, "database connection failed", zap.Error(err))
	}
	defer db.Close()

	rdb, err := cache.Connect(ctx, cfg)
	if err != nil {
		logger.Fatal(ctx, "redis connection failed", zap.Error(err))
	}
	defer rdb.Close()

	// Setup repositories
	userRepo := repository.NewUserRepository(db)
	linkRepo := repository.NewLinkRepository(db)
	clickRepo := repository.NewClickRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	domainRepo := repository.NewDomainRepository(db)

	// Start click flusher worker
	clickFlusher := worker.NewClickFlusher(rdb, clickRepo)
	clickFlusher.Start()
	defer clickFlusher.Stop()

	// Setup services
	authService := service.NewAuthService(userRepo, cfg.JWT.Secret)
	shortCodeSvc := service.NewShortCodeService(linkRepo)
	linkService := service.NewLinkService(linkRepo, shortCodeSvc)
	statsService := service.NewStatsService(clickRepo, linkRepo)
	passkeyService, err := service.NewPasskeyService(passkeyRepo, userRepo, cfg.WebAuthn.RPID, cfg.WebAuthn.RPOrigin, "URL Shortener")
	if err != nil {
		logger.Fatal(ctx, "failed to create passkey service", zap.Error(err))
	}

	// Setup handlers
	authHandler := handler.NewAuthHandler(authService, passkeyService, cfg)
	linkHandler := handler.NewLinkHandler(linkService, domainRepo, cfg)
	statsHandler := handler.NewStatsHandler(statsService)
	passkeyHandler := handler.NewPasskeyHandler(passkeyService)
	passkeyVerifyHandler := handler.NewPasskeyVerifyHandler(passkeyService, authService)
	domainHandler := handler.NewDomainHandler(domainRepo)

	// Click service
	clickService := service.NewClickService(rdb)

	// Redirect service and handler
	redirectService := service.NewRedirectService(linkRepo, domainRepo, rdb)
	redirectHandler := handler.NewRedirectHandler(redirectService, clickService, cfg.JWT.Secret)

	// Redirect Router (public, minimal - for URL redirects)
	redirectRouter := gin.New()
	redirectRouter.Use(gin.Recovery())
	redirectRouter.Use(otelgin.Middleware("redirect-server"))
	redirectRouter.Use(middleware.LogMiddleware())
	redirectRouter.GET("/:code", redirectHandler.Redirect)
	redirectRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "server": "redirect"})
	})

	// API Router (authenticated, full functionality)
	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())
	apiRouter.Use(otelgin.Middleware("api-server"))
	apiRouter.Use(middleware.LogMiddleware())
	apiRouter.Use(middleware.CORSMiddleware(cfg.Server.AllowOrigins))

	apiRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "server": "api"})
	})

	// API Routes
	api := apiRouter.Group("/api")
	{
		authMiddleware := middleware.AuthMiddleware(authService)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", authMiddleware, authHandler.Me)
			auth.PUT("/me", authMiddleware, authHandler.UpdateMe)
			auth.PUT("/password", authMiddleware, authHandler.ChangePassword)

			// Passkey routes (protected)
			auth.GET("/passkeys", authMiddleware, passkeyHandler.List)
			auth.POST("/passkeys/register/begin", authMiddleware, passkeyHandler.BeginRegistration)
			auth.POST("/passkeys/register/finish", authMiddleware, passkeyHandler.FinishRegistration)
			auth.PUT("/passkeys/:id", authMiddleware, passkeyHandler.Rename)
			auth.DELETE("/passkeys/:id", authMiddleware, passkeyHandler.Delete)

			// Passkey verification routes (public - used during login flow)
			auth.POST("/passkeys/verify/begin", passkeyVerifyHandler.BeginVerify)
			auth.POST("/passkeys/verify/finish", passkeyVerifyHandler.FinishVerify)
		}

		// Link routes (protected)
		links := api.Group("/links")
		links.Use(authMiddleware)
		{
			links.POST("", linkHandler.Create)
			links.GET("", linkHandler.List)
			links.GET("/:id", linkHandler.Get)
			links.PUT("/:id", linkHandler.Update)
			links.DELETE("/:id", linkHandler.Delete)
			links.GET("/:id/stats", statsHandler.GetLinkStats)
		}

		// Domain routes (protected)
		domains := api.Group("/domains")
		domains.Use(authMiddleware)
		{
			domains.GET("", domainHandler.List)
			domains.POST("", domainHandler.Create)
			domains.DELETE("/:id", domainHandler.Delete)
		}
	}

	// Start both servers
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		addr := ":" + cfg.Server.RedirectPort
		logger.Info(ctx, "starting redirect server", zap.String("addr", addr))
		if err := redirectRouter.Run(addr); err != nil {
			logger.Fatal(ctx, "redirect server failed", zap.Error(err))
		}
	}()

	go func() {
		defer wg.Done()
		addr := ":" + cfg.Server.APIPort
		logger.Info(ctx, "starting API server", zap.String("addr", addr))
		if err := apiRouter.Run(addr); err != nil {
			logger.Fatal(ctx, "API server failed", zap.Error(err))
		}
	}()

	wg.Wait()
}
```

**Step 2: Verify compilation**

Run: `cd backend && go build ./cmd/server/...`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(main): integrate OpenTelemetry and logging middleware"
```

---

## Task 7: Test End-to-End

**Step 1: Build and run the server**

Run: `cd backend && go build -o server ./cmd/server && ./server`
Expected: Server starts without errors

**Step 2: Make test requests**

In another terminal:
```bash
# Health check
curl http://localhost:8081/health

# Login (to generate logs with different status codes)
curl -X POST http://localhost:8081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test"}'
```

**Step 3: Verify log output**

Check that logs now include:
- Correct caller location (handler/service file, not zap.go)
- `trace_id` and `span_id` fields
- `method`, `path`, `ip`, `latency`, `status` fields
- `request_body` and `response_body` for non-GET requests

Example expected log output:
```json
{
  "L": "INFO",
  "T": "2026-02-03T...",
  "C": "internal/handler/auth.go:45",
  "M": "http request",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "method": "POST",
  "path": "/api/auth/login",
  "ip": "127.0.0.1",
  "status": 200,
  "latency": "5.2ms",
  "request_body": "{\"email\":\"test@example.com\",\"password\":\"test\"}",
  "response_body": "{\"token\":\"...\"}"
}
```

**Step 4: Final commit**

```bash
git add -A
git commit -m "docs: verify logging and OpenTelemetry integration"
```

---

## Summary

| Task | Description | Files Modified |
|------|-------------|----------------|
| 1 | Fix caller skip in logger | `pkg/logger/zap.go` |
| 2 | Add OpenTelemetry dependencies | `go.mod`, `go.sum` |
| 3 | Create OpenTelemetry initialization | `pkg/telemetry/otel.go` (new) |
| 4 | Update logger for trace/span extraction | `pkg/logger/zap.go` |
| 5 | Create request/response logging middleware | `internal/middleware/logging.go` (new) |
| 6 | Integrate middleware in main | `cmd/server/main.go` |
| 7 | End-to-end testing | Manual verification |

## Dependencies Added

- `go.opentelemetry.io/otel` - Core OpenTelemetry API
- `go.opentelemetry.io/otel/sdk` - OpenTelemetry SDK
- `go.opentelemetry.io/otel/trace` - Trace API
- `go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin` - Gin middleware

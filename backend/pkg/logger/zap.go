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
	l := log
	if l == nil {
		// Return no-op logger if not initialized (e.g., during tests)
		l = zap.NewNop()
	}

	if ctx == nil {
		return l
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

// Raw returns the underlying zap.Logger for cases without context
func Raw() *zap.Logger {
	if log == nil {
		return zap.NewNop()
	}
	return log
}

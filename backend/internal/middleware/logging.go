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

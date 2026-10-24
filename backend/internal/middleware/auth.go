// backend/internal/middleware/auth.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthMiddleware accepts either a JWT or a personal API token ("us_" prefix)
// as the bearer token and stores the authenticated user under "user_id".
func AuthMiddleware(authService service.AuthService, tokenSvc service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
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
		var userID uint64
		var err error
		if strings.HasPrefix(token, service.APITokenPrefix) {
			userID, err = tokenSvc.Validate(ctx, token)
		} else {
			userID, err = authService.ValidateToken(token)
		}
		if err != nil {
			logger.Warn(ctx, "auth: invalid or expired token",
				zap.Error(err),
			)
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

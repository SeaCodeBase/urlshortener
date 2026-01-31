// backend/cmd/server/main.go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/jose/urlshortener/internal/cache"
	"github.com/jose/urlshortener/internal/config"
	"github.com/jose/urlshortener/internal/database"
	"github.com/jose/urlshortener/internal/handler"
	"github.com/jose/urlshortener/internal/middleware"
	"github.com/jose/urlshortener/internal/repository"
	"github.com/jose/urlshortener/internal/service"
	"github.com/jose/urlshortener/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger.Init(true)
	defer logger.Sync()

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	rdb, err := cache.Connect(cfg)
	if err != nil {
		logger.Log.Fatalf("Redis connection failed: %v", err)
	}
	defer rdb.Close()

	r := gin.Default()

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

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	logger.Log.Infof("Starting server on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		logger.Log.Fatalf("Failed to start server: %v", err)
	}
}

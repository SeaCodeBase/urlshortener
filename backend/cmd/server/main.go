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
	"github.com/jose/urlshortener/internal/worker"
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
	linkRepo := repository.NewLinkRepository(db)
	clickRepo := repository.NewClickRepository(db)

	// Start click flusher worker
	clickFlusher := worker.NewClickFlusher(rdb, clickRepo)
	clickFlusher.Start()
	defer clickFlusher.Stop()

	// Setup services
	authService := service.NewAuthService(userRepo, cfg.JWTSecret)
	shortCodeSvc := service.NewShortCodeService(linkRepo)
	linkService := service.NewLinkService(linkRepo, shortCodeSvc)

	// Setup handlers
	authHandler := handler.NewAuthHandler(authService)
	linkHandler := handler.NewLinkHandler(linkService, cfg)

	// Click service
	clickService := service.NewClickService(rdb)

	// Redirect service and handler
	redirectService := service.NewRedirectService(linkRepo, rdb)
	redirectHandler := handler.NewRedirectHandler(redirectService, clickService, cfg.JWTSecret)

	// Routes
	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", middleware.AuthMiddleware(authService), authHandler.Me)
		}

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
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Redirect route (must be after API routes to avoid conflicts)
	r.GET("/:code", redirectHandler.Redirect)

	logger.Log.Infof("Starting server on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		logger.Log.Fatalf("Failed to start server: %v", err)
	}
}

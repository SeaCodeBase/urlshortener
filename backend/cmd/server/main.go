// backend/cmd/server/main.go
package main

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/SeaCodeBase/urlshortener/internal/cache"
	"github.com/SeaCodeBase/urlshortener/internal/config"
	"github.com/SeaCodeBase/urlshortener/internal/database"
	"github.com/SeaCodeBase/urlshortener/internal/handler"
	"github.com/SeaCodeBase/urlshortener/internal/middleware"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/internal/worker"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Init(true)
		logger.Log.Fatalf("Configuration error: %v", err)
	}
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

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://frontend:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

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
	statsService := service.NewStatsService(clickRepo, linkRepo)

	// Setup handlers
	authHandler := handler.NewAuthHandler(authService)
	linkHandler := handler.NewLinkHandler(linkService, cfg)
	statsHandler := handler.NewStatsHandler(statsService)

	// Click service
	clickService := service.NewClickService(rdb)

	// Redirect service and handler
	redirectService := service.NewRedirectService(linkRepo, rdb)
	redirectHandler := handler.NewRedirectHandler(redirectService, clickService, cfg.JWTSecret)

	// Routes
	api := r.Group("/api")
	{
		authMiddleware := middleware.AuthMiddleware(authService)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", authMiddleware, authHandler.Me)
			auth.PUT("/password", authMiddleware, authHandler.ChangePassword)
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

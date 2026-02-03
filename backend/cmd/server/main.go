// backend/cmd/server/main.go
package main

import (
	"context"

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
	"github.com/gin-gonic/gin"
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

	r := gin.Default()

	// Use configurable CORS middleware
	r.Use(middleware.CORSMiddleware(cfg.Server.AllowOrigins))

	// Setup repositories
	userRepo := repository.NewUserRepository(db)
	linkRepo := repository.NewLinkRepository(db)
	clickRepo := repository.NewClickRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)

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
	linkHandler := handler.NewLinkHandler(linkService, cfg)
	statsHandler := handler.NewStatsHandler(statsService)
	passkeyHandler := handler.NewPasskeyHandler(passkeyService)
	passkeyVerifyHandler := handler.NewPasskeyVerifyHandler(passkeyService, authService)

	// Click service
	clickService := service.NewClickService(rdb)

	// Redirect service and handler
	redirectService := service.NewRedirectService(linkRepo, rdb)
	redirectHandler := handler.NewRedirectHandler(redirectService, clickService, cfg.JWT.Secret)

	// Routes
	api := r.Group("/api")
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
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Redirect route (must be after API routes to avoid conflicts)
	r.GET("/:code", redirectHandler.Redirect)

	logger.Info(ctx, "starting server", zap.String("port", cfg.Server.Port))
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		logger.Fatal(ctx, "failed to start server", zap.Error(err))
	}
}

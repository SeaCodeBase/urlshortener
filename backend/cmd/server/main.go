// backend/cmd/server/main.go
package main

import (
	"context"
	"net/url"
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

	applied, err := database.Migrate(ctx, db, cfg.Database.MigrationsPath)
	if err != nil {
		logger.Fatal(ctx, "database migration failed",
			zap.String("path", cfg.Database.MigrationsPath), zap.Error(err))
	}
	logger.Info(ctx, "database migrations up to date", zap.Strings("applied", applied))

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
	groupRepo := repository.NewGroupRepository(db)
	routingRuleRepo := repository.NewRoutingRuleRepository(db)
	apiTokenRepo := repository.NewAPITokenRepository(db)
	tagRepo := repository.NewTagRepository(db)
	qrRepo := repository.NewQRRepository(db)
	activityRepo := repository.NewActivityRepository(db)
	pageRepo := repository.NewPageRepository(db)
	pageViewRepo := repository.NewPageViewRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)

	// Outbound webhook requests (verify pings and deliveries): 10 s timeout,
	// ≤3 redirects, refuses non-public addresses.
	webhookClient := service.NewWebhookHTTPClient()

	// Start workers: the click flusher queues webhook events for every
	// stored click; the dispatcher delivers them. Deferred stops run in
	// reverse, so the dispatcher finishes in-flight deliveries first.
	clickFlusher := worker.NewClickFlusher(rdb, clickRepo)
	clickFlusher.SetEnqueuer(worker.NewWebhookEnqueuer(rdb, webhookRepo, linkRepo, tagRepo, domainRepo, cfg.URLs.BaseURL))
	clickFlusher.Start()
	defer clickFlusher.Stop()
	webhookDispatcher := worker.NewWebhookDispatcher(rdb, webhookRepo, webhookClient, cfg.Workers.Webhooks.PollEvery)
	webhookDispatcher.Start()
	defer webhookDispatcher.Stop()

	// Setup services
	authService := service.NewAuthService(userRepo, groupRepo, cfg.JWT.Secret)
	groupService := service.NewGroupService(groupRepo, userRepo, domainRepo)
	tokenService := service.NewTokenService(apiTokenRepo, userRepo)
	tagService := service.NewTagService(tagRepo, qrRepo)
	activityService := service.NewActivityService(activityRepo)
	webhookService := service.NewWebhookService(service.WebhookServiceDeps{
		Webhooks: webhookRepo,
		Activity: activityService,
		Client:   webhookClient,
		Resolver: service.NewNetResolver(),
	})
	shortCodeSvc := service.NewShortCodeService(linkRepo)
	linkService := service.NewLinkService(service.LinkServiceDeps{
		Links:      linkRepo,
		ShortCodes: shortCodeSvc,
		Tags:       tagRepo,
		Rules:      routingRuleRepo,
		Domains:    domainRepo,
		Groups:     groupRepo,
		Clicks:     clickRepo,
		QRs:        qrRepo,
		Activity:   activityService,
		BaseURL:    cfg.URLs.BaseURL,
	})
	qrService := service.NewQRService(service.QRServiceDeps{
		QRs:      qrRepo,
		Links:    linkService,
		LinkRepo: linkRepo,
		Activity: activityService,
		Domains:  domainRepo,
		BaseURL:  cfg.URLs.BaseURL,
	})
	pageService := service.NewPageService(service.PageServiceDeps{
		Pages:    pageRepo,
		Views:    pageViewRepo,
		Links:    linkRepo,
		LinkSvc:  linkService,
		Domains:  domainRepo,
		Activity: activityService,
		BaseURL:  cfg.URLs.BaseURL,
	})
	// Page views hash IPs with the same salt as clicks (the redirect handler's).
	pagePublicService := service.NewPagePublicService(pageRepo, pageViewRepo, clickRepo, linkRepo, domainRepo,
		cfg.URLs.BaseURL, cfg.JWT.Secret)
	statsService := service.NewStatsService(clickRepo, linkRepo, domainRepo, cfg.URLs.BaseURL)
	analyticsService := service.NewAnalyticsService(clickRepo, linkRepo, qrRepo, domainRepo, cfg.URLs.BaseURL)
	dashboardService := service.NewDashboardService(service.DashboardServiceDeps{
		Dashboards: repository.NewDashboardRepository(db),
		Links:      linkRepo,
		QRs:        qrRepo,
		Activity:   activityService,
	})
	uploadService, err := service.NewUploadService(cfg.Uploads.Path)
	if err != nil {
		logger.Fatal(ctx, "upload directory setup failed", zap.Error(err))
	}
	passkeyService, err := service.NewPasskeyService(passkeyRepo, userRepo, cfg.WebAuthn.RPID, cfg.WebAuthn.RPOrigin, "URL Shortener")
	if err != nil {
		logger.Fatal(ctx, "failed to create passkey service", zap.Error(err))
	}
	baseHost := ""
	if u, err := url.Parse(cfg.URLs.BaseURL); err == nil {
		baseHost = u.Hostname()
	}
	frontendHost := ""
	if u, err := url.Parse(cfg.URLs.FrontendURL); err == nil {
		frontendHost = u.Hostname()
	}
	// The service's own hosts can never be claimed as custom domains.
	domainService := service.NewDomainService(domainRepo, groupRepo, linkRepo, service.NewNetResolver(),
		cfg.URLs.RedirectHost, baseHost, frontendHost)
	redirectService := service.NewRedirectService(linkRepo, domainRepo, routingRuleRepo, rdb, baseHost,
		cfg.URLs.RedirectHost, frontendHost)
	domainService.SetCacheInvalidator(redirectService)

	// Legacy domain rows for the service's own hosts (backfilled as "ok" by
	// migration 002) must not stay usable.
	if n, err := domainService.DemoteReservedDomains(ctx); err != nil {
		logger.Error(ctx, "failed to demote reserved-host domains", zap.Error(err))
	} else if n > 0 {
		logger.Warn(ctx, "demoted custom domains that use reserved hosts", zap.Int("count", n))
	}

	// Setup handlers
	authHandler := handler.NewAuthHandler(authService, passkeyService, cfg)
	linkHandler := handler.NewLinkHandler(linkService, redirectService, domainRepo, cfg)
	linkHandler.SetPageIDs(pageService)
	pageHandler := handler.NewPageHandler(pageService, pagePublicService)
	publicPageHandler := handler.NewPublicPageHandler(pagePublicService)
	qrHandler := handler.NewQRHandler(qrService, redirectService, domainRepo, statsService, cfg)
	statsHandler := handler.NewStatsHandler(statsService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	passkeyHandler := handler.NewPasskeyHandler(passkeyService)
	passkeyVerifyHandler := handler.NewPasskeyVerifyHandler(passkeyService, authService)
	domainHandler := handler.NewDomainHandler(domainService, activityService)
	tokenHandler := handler.NewTokenHandler(tokenService, activityService)
	groupHandler := handler.NewGroupHandler(groupService, activityService)
	searchHandler := handler.NewSearchHandler(linkRepo, qrRepo)
	metaHandler := handler.NewMetaHandler(cfg)
	tagHandler := handler.NewTagHandler(tagService)
	activityHandler := handler.NewActivityHandler(activityService)
	analyticsHandler := handler.NewAnalyticsHandler(statsService, analyticsService)
	dashboardHandler := handler.NewDashboardHandler(dashboardService, analyticsService)
	webhookHandler := handler.NewWebhookHandler(webhookService)

	// Click service
	clickService := service.NewClickService(rdb)

	// Redirect handler
	redirectHandler := handler.NewRedirectHandler(redirectService, clickService, domainRepo, cfg.JWT.Secret, cfg.URLs.FrontendURL,
		baseHost, cfg.URLs.RedirectHost, frontendHost)

	// Redirect Router (public, minimal - for URL redirects)
	redirectRouter := gin.New()
	redirectRouter.Use(gin.Recovery())
	redirectRouter.Use(otelgin.Middleware("redirect-server"))
	redirectRouter.Use(middleware.LogMiddleware())
	redirectRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "server": "redirect"})
	})
	redirectRouter.GET("/", redirectHandler.Root)
	redirectRouter.GET("/:code", redirectHandler.Redirect)
	// Shareable page URLs {BaseURL}/m/{slug} hop to the frontend's page route.
	// HEAD too, so link checkers and `curl -I` see the 302.
	redirectRouter.GET("/m/:slug", redirectHandler.PageHop)
	redirectRouter.HEAD("/m/:slug", redirectHandler.PageHop)

	// API Router (authenticated, full functionality)
	apiRouter := gin.New()
	apiRouter.Use(gin.Recovery())
	apiRouter.Use(otelgin.Middleware("api-server"))
	apiRouter.Use(middleware.LogMiddleware())
	apiRouter.Use(middleware.CORSMiddleware(cfg.Server.AllowOrigins))

	apiRouter.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "server": "api"})
	})

	// Uploaded images (public, no directory listing)
	uploads := apiRouter.Group("/uploads", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	uploads.StaticFS("/", gin.Dir(cfg.Uploads.Path, false))

	// API Routes
	api := apiRouter.Group("/api")
	{
		// Public, non-secret configuration for the web UI.
		api.GET("/meta", metaHandler.Get)

		authMiddleware := middleware.AuthMiddleware(authService, tokenService)
		groupMW := middleware.GroupMiddleware(groupService)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.GET("/me", authMiddleware, authHandler.Me)
			auth.PUT("/me", authMiddleware, authHandler.UpdateMe)
			auth.PUT("/password", authMiddleware, authHandler.ChangePassword)
			auth.PUT("/email", authMiddleware, authHandler.UpdateEmail)

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
		links.Use(authMiddleware, groupMW)
		{
			links.POST("", linkHandler.Create)
			links.GET("", linkHandler.List)
			links.GET("/export", linkHandler.Export)
			links.PATCH("/bulk", linkHandler.Bulk)
			links.GET("/:id", linkHandler.Get)
			links.PUT("/:id", linkHandler.Update)
			links.DELETE("/:id", linkHandler.Delete)
			links.POST("/:id/back-half", linkHandler.EditBackHalf)
			links.PUT("/:id/tags", linkHandler.SetTags)
			links.GET("/:id/routing-rules", linkHandler.GetRoutingRules)
			links.PUT("/:id/routing-rules", linkHandler.SetRoutingRules)
			links.GET("/:id/stats", statsHandler.GetLinkStats)
		}

		// QR code routes (protected)
		qrCodes := api.Group("/qr-codes")
		qrCodes.Use(authMiddleware, groupMW)
		{
			qrCodes.GET("", qrHandler.List)
			qrCodes.POST("", qrHandler.Create)
			qrCodes.GET("/export", qrHandler.Export)
			qrCodes.PATCH("/bulk", qrHandler.Bulk)
			qrCodes.GET("/:id", qrHandler.Get)
			qrCodes.PUT("/:id", qrHandler.Update)
			qrCodes.DELETE("/:id", qrHandler.Delete)
			qrCodes.PUT("/:id/tags", qrHandler.SetTags)
			qrCodes.GET("/:id/image", qrHandler.Image)
			qrCodes.GET("/:id/stats", qrHandler.Stats)
		}

		// Image uploads (QR code logos); user-level, no group.
		api.POST("/uploads", authMiddleware, uploadHandler.Create)

		// API token routes (tokens are user-level; the active group is only
		// used to file token.create/token.revoke in the activity log)
		apiTokens := api.Group("/api-tokens")
		apiTokens.Use(authMiddleware, groupMW)
		{
			apiTokens.GET("", tokenHandler.List)
			apiTokens.POST("", tokenHandler.Create)
			apiTokens.DELETE("/:id", tokenHandler.Revoke)
		}

		// Groups (protected, not group-scoped: the path names the group)
		groups := api.Group("/groups")
		groups.Use(authMiddleware)
		{
			groups.GET("", groupHandler.List)
			groups.POST("", groupHandler.Create)
			groups.GET("/:id", groupHandler.Get)
			groups.PUT("/:id", groupHandler.Update)
		}

		// Global search (protected, group-scoped)
		api.GET("/search", authMiddleware, groupMW, searchHandler.Search)

		// Tags and activity log (protected, group-scoped)
		api.GET("/tags", authMiddleware, groupMW, tagHandler.List)
		api.GET("/activity-log", authMiddleware, groupMW, activityHandler.List)

		// Group-level analytics (protected, group-scoped)
		analytics := api.Group("/analytics")
		analytics.Use(authMiddleware, groupMW)
		{
			analytics.GET("/today", analyticsHandler.Today)
			analytics.GET("/overview", analyticsHandler.Overview)
			analytics.GET("/dashboards", dashboardHandler.List)
			analytics.POST("/dashboards", dashboardHandler.Create)
			analytics.GET("/dashboards/:id", dashboardHandler.Get)
			analytics.PATCH("/dashboards/:id", dashboardHandler.Update)
			analytics.POST("/dashboards/:id/duplicate", dashboardHandler.Duplicate)
			analytics.DELETE("/dashboards/:id", dashboardHandler.Delete)
			analytics.GET("/dashboards/:id/data", dashboardHandler.Data)
			analytics.GET("/dashboards/:id/export", dashboardHandler.Export)
		}

		// Link-in-bio pages (protected, group-scoped)
		handler.RegisterPageRoutes(api.Group("/pages", authMiddleware, groupMW), pageHandler)

		// Webhooks (protected, group-scoped)
		handler.RegisterWebhookRoutes(api, webhookHandler, authMiddleware, groupMW)

		// Published pages (public: no auth, no group; drafts are never served)
		public := api.Group("/public")
		{
			public.GET("/pages/:slug", publicPageHandler.Get)
			public.POST("/pages/:slug/view", publicPageHandler.View)
		}

		// Domain routes (protected)
		domains := api.Group("/domains")
		domains.Use(authMiddleware, groupMW)
		{
			domains.GET("", domainHandler.List)
			domains.POST("", domainHandler.Create)
			domains.PUT("/:id", domainHandler.Update)
			domains.POST("/:id/verify", domainHandler.Verify)
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

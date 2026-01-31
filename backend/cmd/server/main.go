// backend/cmd/server/main.go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/jose/urlshortener/internal/config"
	"github.com/jose/urlshortener/internal/database"
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

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	logger.Log.Infof("Starting server on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		logger.Log.Fatalf("Failed to start server: %v", err)
	}
}

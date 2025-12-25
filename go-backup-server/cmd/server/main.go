package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/ilker/backup-server/internal/config"
	"github.com/ilker/backup-server/internal/handlers"
	"github.com/ilker/backup-server/internal/middleware"
	"github.com/ilker/backup-server/internal/repository"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize database
	db, err := repository.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize JWT auth
	jwtAuth := middleware.NewJWTAuth(cfg.JWT.Secret, cfg.JWT.ExpireHour)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, jwtAuth)
	userHandler := handlers.NewUserHandler(db)
	deviceHandler := handlers.NewDeviceHandler(db)
	backupHandler := handlers.NewBackupHandler(db, cfg.Storage.BasePath)
	accountHandler := handlers.NewAccountHandler(db)

	// Setup router
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
		}

		// Protected auth routes
		authProtected := v1.Group("/auth")
		authProtected.Use(jwtAuth.Middleware())
		{
			authProtected.POST("/logout", authHandler.Logout)
			authProtected.GET("/me", authHandler.Me)
		}

		// User management (admin only)
		users := v1.Group("/users")
		users.Use(jwtAuth.Middleware(), middleware.AdminMiddleware())
		{
			users.GET("", userHandler.List)
			users.POST("", userHandler.Create)
			users.GET("/:id", userHandler.Get)
			users.PATCH("/:id", userHandler.Update)
			users.DELETE("/:id", userHandler.Delete)
			users.POST("/:id/approve", userHandler.Approve)
		}

		// Device routes
		devices := v1.Group("/devices")
		devices.Use(jwtAuth.Middleware())
		{
			devices.GET("", deviceHandler.List)
			devices.POST("", deviceHandler.Create)
			devices.GET("/:id", deviceHandler.Get)
			devices.PATCH("/:id", deviceHandler.Update)
			devices.DELETE("/:id", deviceHandler.Delete)

			// Backup routes (nested under devices)
			devices.GET("/:id/backups", backupHandler.List)
			devices.POST("/:id/backups", backupHandler.Upload)
			devices.GET("/:id/backups/latest", backupHandler.Latest)
			devices.GET("/:id/backups/:backupId", backupHandler.Get)
			devices.GET("/:id/backups/:backupId/download", backupHandler.Download)
			devices.DELETE("/:id/backups/:backupId", backupHandler.Delete)

			// Catalog routes (encrypted SQLite dumps for zero-knowledge recovery)
			devices.GET("/:id/catalogs", backupHandler.ListCatalogs)
			devices.POST("/:id/catalogs", backupHandler.UploadCatalog)
			devices.GET("/:id/catalogs/:catalogId/download", backupHandler.DownloadCatalog)

			// File restore (Time Machine style - restore specific files at specific dates)
			devices.POST("/:id/restore-files", backupHandler.RestoreFiles)
		}

		// Account routes
		account := v1.Group("/account")
		account.Use(jwtAuth.Middleware())
		{
			account.GET("/quota", accountHandler.Quota)
			account.GET("/usage", accountHandler.Usage)
		}
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

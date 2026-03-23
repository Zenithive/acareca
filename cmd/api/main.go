package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/iamarpitzala/acareca/docs"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	"github.com/iamarpitzala/acareca/internal/shared/middleware"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/iamarpitzala/acareca/route"
	"github.com/joho/godotenv"
)

// @title Backend API
// @version 1.0.0
// @description Backend API for acareca
// @contact.name API Support

// @securityDefinitions.apikey BearerToken
// @in header
// @name Authorization
// @description Type "Bearer <your_token>" to authenticate

// @BasePath /api/v1
// @schemes http https
// @consumes application/json
// @produces application/json
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.NewConfig()

	// --- DYNAMIC SWAGGER CONFIGURATION ---
	// This overrides the static comments based on the environment
	docs.SwaggerInfo.BasePath = "/api/v1"
	if os.Getenv("GIN_MODE") == "release" {
		docs.SwaggerInfo.Host = "acareca-bam8.onrender.com"
		docs.SwaggerInfo.Schemes = []string{"https"}
	} else {
		// Use server port from config or default 8080
		docs.SwaggerInfo.Host = "localhost:" + cfg.ServerPort
		docs.SwaggerInfo.Schemes = []string{"http"}
	}
	// -------------------------------------

	dbConn, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	if err := db.RunMigrations(dbConn.DB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations applied successfully")

	// Set Gin mode; prefer env GIN_MODE over hardcoded
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	if gin.Mode() != gin.ReleaseMode {
		log.Printf("[GIN-debug] [WARNING] Running in %q mode. Switch to \"release\" mode in production.\n", gin.Mode())
		log.Print(" - using env:   export GIN_MODE=release\n")
		log.Print(" - using code:  gin.SetMode(gin.ReleaseMode)\n\n")
	}

	// ALLOWED_ORIGINS must be explicitly set in production.
	// e.g. "http://localhost:5173,https://your-frontend.com"
	//
	// AllowCredentials: true requires specific origins — browsers reject the
	// combination of wildcard ("*") + credentials, so we only enable credentials
	// when real origins are configured. In development (no env var set) the
	// wildcard is kept for convenience but credentials are disabled.
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	var origins []string
	allowCredentials := false
	if allowedOrigins == "" {
		origins = []string{"*"}
		log.Println("[WARN] ALLOWED_ORIGINS not set — CORS running in wildcard mode, credentials disabled")
	} else {
		origins = strings.Split(allowedOrigins, ",")
		allowCredentials = true
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Authorization", "token"},
		AllowCredentials: allowCredentials,
		MaxAge:           12 * time.Hour,
	}))
	r.Use(middleware.ClientInfo())
	auditSvc := route.RegisterRoutes(r, cfg)

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}

	auditSvc.Shutdown()
	log.Println("server exited")
}

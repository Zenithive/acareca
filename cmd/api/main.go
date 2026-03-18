package main

import (
	"log"
	_ "net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/iamarpitzala/acareca/docs"
	_ "github.com/iamarpitzala/acareca/docs"
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

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS") // e.g. "http://localhost:5173,https://your-frontend.com"
	var origins []string
	if allowedOrigins == "" {
		origins = []string{"*"}
	} else {
		origins = strings.Split(allowedOrigins, ",")
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Authorization", "token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.Use(middleware.ClientInfo())
	route.RegisterRoutes(r, cfg)

	log.Printf("server starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

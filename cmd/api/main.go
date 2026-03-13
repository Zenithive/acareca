package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

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
// @host localhost:8080
// @BasePath /api/v1
// @schemes http
// @consumes application/json
// @produces application/json
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.NewConfig()

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
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
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

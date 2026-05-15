package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"

	"github.com/iamarpitzala/acareca/docs"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	"github.com/iamarpitzala/acareca/internal/shared/db"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"
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

	// ============================================
	// NATS SETUP
	// ============================================
	var events sharedEvents.IEvent
	var nc *nats.Conn

	// Connect to NATS
	nc, err = nats.Connect(
		cfg.NATSUrl,
		nats.Name("acareca-notification-service"),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("⚠️  NATS disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("✅ NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Println("❌ NATS connection closed")
		}),
	)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to connect to NATS at %s: %v", cfg.NATSUrl, err)
		log.Println("📝 Notifications will not be processed asynchronously")
		log.Println("💡 To enable NATS: Set NATS_URL in .env or start NATS server")
		events = nil
	} else {
		defer nc.Close()
		log.Printf("✅ Connected to NATS at %s", cfg.NATSUrl)

		// Setup Event System with JetStream
		events, err = sharedEvents.NewEvent(
			nc,
			5,                               // maxDeliver - retry up to 5 times
			100,                             // maxAckPending - process up to 100 messages concurrently
			512,                             // maxWaiting - queue up to 512 pull requests
			30*time.Second,                  // ackWait - wait 30s for acknowledgment
			"DLQ",                           // dlqPrefix - dead letter queue prefix
			notification.StreamNotification, // stream name
			[]string{ // subjects
				notification.SubjectNotificationInApp,
				notification.SubjectNotificationEmail,
				notification.SubjectNotificationPush,
			},
		)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to setup JetStream: %v", err)
			log.Println("📝 Notifications will not be processed asynchronously")
			events = nil
		} else {
			log.Println("✅ JetStream initialized successfully")
		}
	}

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

	r.Use(middleware.CORS(cfg))
	r.Use(middleware.ClientInfo())
	// r.Use(middleware.RateLimitMiddleware(1000, 1000))
	auditSvc, notifier, notificationRepo, fileUploadWorker, _, notificationConsumer := route.RegisterRoutes(r, cfg, events)

	// ============================================
	// START NATS CONSUMERS (if NATS is available)
	// ============================================
	if events != nil {
		// Start notification creation consumer
		go func() {
			log.Println("🚀 Starting notification create consumer...")
			if err := notificationConsumer.StartNotificationCreateConsumer(context.Background()); err != nil {
				log.Printf("❌ Notification create consumer stopped: %v", err)
			}
		}()

		// Start email consumer
		go func() {
			log.Println("🚀 Starting email delivery consumer...")
			if err := notificationConsumer.StartEmailConsumer(context.Background()); err != nil {
				log.Printf("❌ Email consumer stopped: %v", err)
			}
		}()

		// Start push consumer
		go func() {
			log.Println("🚀 Starting push notification consumer...")
			if err := notificationConsumer.StartPushConsumer(context.Background()); err != nil {
				log.Printf("❌ Push consumer stopped: %v", err)
			}
		}()

		log.Println("✅ All NATS consumers started")
	}

	// ============================================
	// START BACKGROUND WORKERS
	// ============================================
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	// Start the in_app delivery retry worker
	go notification.StartRetryWorker(workerCtx, notificationRepo, notifier)
	log.Println("✅ Notification retry worker started")

	// Start file upload worker
	go fileUploadWorker.Start(workerCtx)
	log.Println("✅ File upload worker started")

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
	log.Println("🛑 Shutting down server...")

	// Cancel worker context first
	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Server forced to shutdown: %v", err)
	}

	auditSvc.Shutdown()
	log.Println("✅ Server exited gracefully")
}

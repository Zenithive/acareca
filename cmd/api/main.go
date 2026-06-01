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

	docs.SwaggerInfo.BasePath = "/api/v1"
	if os.Getenv("GIN_MODE") == "release" {
		docs.SwaggerInfo.Host = "acareca-bam8.onrender.com"
		docs.SwaggerInfo.Schemes = []string{"https"}
	} else {
		docs.SwaggerInfo.Host = "localhost:" + cfg.ServerPort
		docs.SwaggerInfo.Schemes = []string{"http"}
	}

	dbConn, err := db.DBConn(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	if err := db.RunMigrations(dbConn.DB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations applied successfully")

	var events sharedEvents.IEvent
	var nc *nats.Conn

	nc, err = nats.Connect(
		cfg.NATSUrl,
		nats.Name("acareca-notification-service"),
		nats.MaxReconnects(-1),
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
		log.Printf("⚠️  Warning: Failed to connect to NATS: %v", err)
		log.Println("💡 To enable NATS: Set NATS_URL in .env")
		events = nil
	} else {
		defer nc.Close()
		log.Printf("✅ Connected to NATS at %s", cfg.NATSUrl)

		events, err = sharedEvents.NewEvent(
			nc,
			5,
			100,
			512,
			30*time.Second,
			"DLQ",
			notification.StreamNotification,
			[]string{
				notification.SubjectNotificationInApp,
				notification.SubjectNotificationEmail,
				notification.SubjectNotificationPush,
			},
		)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to setup JetStream: %v", err)
			events = nil
		} else {
			log.Println("✅ JetStream initialized successfully")
		}
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.ClientInfo())

	auditSvc, notifier, notificationRepo, _, notificationConsumer := route.RegisterRoutes(r, cfg, events)

	if events != nil {
		go func() {
			log.Println("🚀 Starting notification create consumer...")
			if err := notificationConsumer.StartNotificationCreateConsumer(context.Background()); err != nil {
				log.Printf("❌ Notification create consumer stopped: %v", err)
			}
		}()

		go func() {
			log.Println("🚀 Starting email delivery consumer...")
			if err := notificationConsumer.StartEmailConsumer(context.Background()); err != nil {
				log.Printf("❌ Email consumer stopped: %v", err)
			}
		}()

		go func() {
			log.Println("🚀 Starting push notification consumer...")
			if err := notificationConsumer.StartPushConsumer(context.Background()); err != nil {
				log.Printf("❌ Push consumer stopped: %v", err)
			}
		}()

		log.Println("✅ All NATS consumers started")
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go notification.StartRetryWorker(workerCtx, notificationRepo, notifier)
	log.Println("✅ Notification retry worker started")

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

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

	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Server forced to shutdown: %v", err)
	}

	auditSvc.Shutdown()
	log.Println("✅ Server exited gracefully")
}

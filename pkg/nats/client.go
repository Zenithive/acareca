package nats

import (
	"log"
	"time"

	"github.com/iamarpitzala/acareca/internal/modules/notification"
	sharedEvents "github.com/iamarpitzala/acareca/internal/shared/events"

	"github.com/nats-io/nats.go"
)

func NewNats(url string) (*nats.Conn, sharedEvents.IEvent, error) {
	var events sharedEvents.IEvent
	nc, err := nats.Connect(
		url,
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
		log.Printf("✅ Connected to NATS at %s", url)

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

	return nc, events, nil
}

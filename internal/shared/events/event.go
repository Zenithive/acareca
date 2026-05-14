package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Event struct {
	js            jetstream.JetStream
	maxDeliver    int
	maxAckPending int
	maxWaiting    int
	ackWait       time.Duration
	dlqPrefix     string
	stream        string
	subject       []string
}

type IEvent interface {
	Publish(ctx context.Context, subject string, payload interface{}) error
	Consume(ctx context.Context, stream string, consumer string, subject string, handler func(jetstream.Msg) error) error
	DLQ(ctx context.Context, msg jetstream.Msg) error
}

func NewEvent(nc *nats.Conn, maxDeliver, maxAckPending, maxWaiting int, ackWait time.Duration, dlqPrefix string, stream string, subject []string) (IEvent, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	// Add DLQ subjects to capture dead letter queue messages
	allSubjects := append(subject, dlqPrefix+".*")

	e := &Event{
		js:            js,
		maxDeliver:    maxDeliver,
		maxAckPending: maxAckPending,
		maxWaiting:    maxWaiting,
		ackWait:       ackWait,
		dlqPrefix:     dlqPrefix,
		stream:        stream,
		subject:       allSubjects,
	}

	// exists before any Publish or Consume calls are made.
	setupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.SetupStream(setupCtx, js); err != nil {
		return nil, fmt.Errorf("failed to setup stream: %w", err)
	}

	return e, nil
}

func (e *Event) SetupStream(ctx context.Context, js jetstream.JetStream) error {
	log.Println("Setting up NOTIFICATIONS stream...")

	streamConfig := jetstream.StreamConfig{
		Name:         e.stream,
		Description:  "Stream for notification events",
		Subjects:     e.subject,
		Retention:    jetstream.WorkQueuePolicy,
		MaxAge:       24 * time.Hour,
		MaxMsgs:      1_000_000,
		MaxBytes:     1024 * 1024 * 1024,
		Discard:      jetstream.DiscardOld,
		Storage:      jetstream.FileStorage,
		Replicas:     1,
		NoAck:        false,
		Duplicates:   1 * time.Minute,
		MaxConsumers: -1,
	}

	stream, err := js.CreateStream(ctx, streamConfig)
	if err != nil {
		stream, err = js.UpdateStream(ctx, streamConfig)
		if err != nil {
			return fmt.Errorf("failed to create/update stream: %w", err)
		}
		log.Println("NOTIFICATIONS stream updated successfully")
	} else {
		log.Println("NOTIFICATIONS stream created successfully")
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	log.Printf("Stream info: %d messages, %d bytes, %d consumers",
		info.State.Msgs, info.State.Bytes, info.State.Consumers)

	return nil
}

func (e *Event) Publish(ctx context.Context, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal error (%s): %w", subject, err)
	}

	_, err = e.js.Publish(ctx, subject, data)
	if err != nil {
		return err
	}

	return nil
}

func (e *Event) Consume(ctx context.Context, stream string, consumer string, subject string, handler func(jetstream.Msg) error) error {
	// long-lived shutdown context which will cancel during normal operation.
	setupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cons, err := e.js.CreateOrUpdateConsumer(
		setupCtx,
		stream,
		jetstream.ConsumerConfig{
			Name:          consumer,
			Durable:       consumer,
			FilterSubject: subject,
			AckPolicy:     jetstream.AckExplicitPolicy,
			AckWait:       e.ackWait,
			MaxDeliver:    e.maxDeliver,
			MaxAckPending: e.maxAckPending,
			MaxWaiting:    e.maxWaiting,
		},
	)
	if err != nil {
		return err
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		nwCtx, cancel := context.WithTimeout(context.Background(), e.ackWait)
		defer cancel()

		if err := handler(msg); err != nil {
			// retries naturally. Send to DLQ only on final attempt.
			meta, metaErr := msg.Metadata()
			if metaErr == nil && meta.NumDelivered >= uint64(e.maxDeliver) {
				if dlqErr := e.DLQ(nwCtx, msg); dlqErr != nil {
					// DLQ publish failed: Nak so NATS keeps the message alive.
					log.Printf("failed to send to DLQ, naking message: %v", dlqErr)
					_ = msg.Nak()
				}
				// DLQ succeeded: Ack is already called inside DLQ().
			} else {
				// Still has retries left: Nak and let NATS redeliver.
				_ = msg.Nak()
			}
		} else {
			_ = msg.Ack()
		}
	})
	if err != nil {
		return err
	}

	<-ctx.Done()
	cc.Stop()

	return nil
}

func (e *Event) DLQ(ctx context.Context, msg jetstream.Msg) error {
	dlqSubject := fmt.Sprintf("%s.%s", e.dlqPrefix, msg.Subject())

	_, err := e.js.Publish(ctx, dlqSubject, msg.Data())
	if err != nil {
		return fmt.Errorf("failed to publish to DLQ (%s): %w", dlqSubject, err)
	}

	// Moved below the error check so a failed publish doesn't silently drop the message.
	_ = msg.Ack()
	return nil
}

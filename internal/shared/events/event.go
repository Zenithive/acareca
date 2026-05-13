package notification

import (
	"context"
	"encoding/json"
	"fmt"
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
}

type IEvent interface {
	Publish(ctx context.Context, subject string, payload interface{}) error
	Consume(ctx context.Context, stream string, consumer string, subject string, handler func(jetstream.Msg) error) error
	DLQ(ctx context.Context, msg jetstream.Msg) error
}

func NewEvent(nc *nats.Conn, maxDeliver, maxAckPending, maxWaiting int, ackWait time.Duration, dlqPrefix string) (IEvent, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}
	return &Event{
		js:            js,
		maxDeliver:    maxDeliver,
		maxAckPending: maxAckPending,
		maxWaiting:    maxWaiting,
		ackWait:       ackWait,
		dlqPrefix:     dlqPrefix,
	}, nil
}

// Publish implements [IEvent].
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

// Consume implements [IEvent].
func (e *Event) Consume(ctx context.Context, stream string, consumer string, subject string, handler func(jetstream.Msg) error) error {
	cons, err := e.js.CreateOrUpdateConsumer(
		ctx,
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
			if dlqErr := e.DLQ(nwCtx, msg); dlqErr != nil {
				fmt.Printf("failed to send to DLQ: %v\n", dlqErr)
				return
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

// DLQ implements [IEvent].
func (e *Event) DLQ(ctx context.Context, msg jetstream.Msg) error {
	dlqSubject := fmt.Sprintf("%s.%s", e.dlqPrefix, msg.Subject())

	_, err := e.js.Publish(ctx, dlqSubject, msg.Data())
	if err != nil {
		return fmt.Errorf("failed to publish to DLQ (%s): %w", dlqSubject, err)
	}

	_ = msg.Ack()
	return nil
}

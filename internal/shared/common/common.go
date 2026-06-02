package common

import (
	"context"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// RecipientWithPreferences holds recipient info and their notification preferences
// Deprecated: Use sharednotification.RecipientWithPreferences instead
type RecipientWithPreferences = sharednotification.RecipientWithPreferences

// PublishNotification publishes notifications to recipients based on their preferences
// Deprecated: Use sharednotification.Publisher instead for better flexibility
func PublishNotification(ctx context.Context, notificationSvc notification.Service, recipients []RecipientWithPreferences, senderID uuid.UUID, senderType util.ActorType, senderName string, eventType util.EventType, entityType util.EntityType, entityID uuid.UUID, entityKey string, title string, body string) {
	publisher := sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc))
	
	_ = publisher.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   senderID,
		SenderType: senderType,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: entityType,
		EntityID:   entityID,
		EntityKey:  entityKey,
		Title:      title,
		Body:       body,
	})
}

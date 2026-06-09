package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type RecipientWithPreferences struct {
	RecipientID   uuid.UUID
	RecipientType util.ActorType
	UserID        uuid.UUID
}

type NotificationService interface {
	Publish(ctx context.Context, rq NotificationRequest) error
	GetPreferences(ctx context.Context, userID uuid.UUID) (NotificationPreferences, error)
}

type NotificationRequest struct {
	ID            uuid.UUID
	RecipientID   uuid.UUID
	RecipientType util.ActorType
	SenderID      *uuid.UUID
	SenderType    *util.ActorType
	EventType     util.EventType
	EntityType    util.EntityType
	EntityID      uuid.UUID
	Status        util.Status
	Payload       []byte
	Channels      []util.Channel
	CreatedAt     time.Time
}

type NotificationPreferences struct {
	EventType []util.NotificationEventType
	Channels  []util.Channel
}

type NotificationPayload struct {
	Title      string                  `json:"title"`
	Body       json.RawMessage         `json:"body"`
	SenderName *string                 `json:"sender_name,omitempty"`
	Metadata   *map[string]interface{} `json:"metadata,omitempty"`
	ExtraData  *map[string]interface{} `json:"extra_data,omitempty"`
}

type PublishRequest struct {
	Recipients []RecipientWithPreferences
	SenderID   uuid.UUID
	SenderType util.ActorType
	SenderName string
	EventType  util.EventType
	EntityType util.EntityType
	EntityID   uuid.UUID
	EntityKey  string
	Title      string
	Body       string
	ExtraData  map[string]interface{}
}

type Publisher struct {
	notificationSvc NotificationService
	adminRepo       admin.Repository
}

func NewPublisher(notificationSvc NotificationService, adminRepo admin.Repository) *Publisher {
	return &Publisher{notificationSvc: notificationSvc, adminRepo: adminRepo}
}

func (p *Publisher) appendAdminRecipients(ctx context.Context, recipients []RecipientWithPreferences) []RecipientWithPreferences {
	if p.adminRepo == nil {
		return recipients
	}

	admins, err := p.adminRepo.GetAllAdmins(ctx)
	if err != nil {
		log.Printf("[WARN] failed to get admin users: %v", err)
		return recipients
	}

	for _, a := range admins {
		recipients = append(recipients, RecipientWithPreferences{
			RecipientID:   a.ID,
			RecipientType: util.ActorAdmin,
			UserID:        a.User.ID,
		})
	}

	return recipients
}

func (p *Publisher) Publish(ctx context.Context, req PublishRequest) error {
	if p.notificationSvc == nil {
		return fmt.Errorf("notification service is nil")
	}

	recipients := p.appendAdminRecipients(ctx, append([]RecipientWithPreferences{}, req.Recipients...))

	for _, recipient := range recipients {
		prefs, err := p.notificationSvc.GetPreferences(ctx, recipient.UserID)

		if err != nil {
			log.Printf("[ERROR] failed to get preferences for user %s: %v", recipient.UserID, err)
			continue
		}
		fmt.Println("pref=========================================", len(prefs.EventType))

		prefMap := make(map[util.NotificationEventType]struct{}, len(prefs.EventType))
		for _, et := range prefs.EventType {
			prefMap[et] = struct{}{}
		}

		if _, ok := prefMap[util.NotificationEventType(req.EventType)]; !ok {
			mappedTypes := util.MapEventTypeToNotificationEventType(req.EventType)
			hasMatch := false
			for _, mappedType := range mappedTypes {
				if _, ok = prefMap[mappedType]; ok {
					hasMatch = true
					break
				}
			}
			if !hasMatch {
				continue
			}
		}

		channels := make([]util.Channel, 0, len(prefs.Channels))
		for _, ch := range prefs.Channels {
			channels = append(channels, ch)
		}

		if len(channels) == 0 {
			log.Printf("[INFO] no enabled channels for user %s, event %s", recipient.UserID, req.EventType)
			continue
		}
		extraData := map[string]interface{}{req.EntityKey: req.EntityID.String()}
		if req.ExtraData != nil {
			maps.Copy(extraData, req.ExtraData)
		}

		payload := NotificationPayload{
			Title:      req.Title,
			Body:       json.RawMessage(fmt.Sprintf(`"%s"`, req.Body)),
			SenderName: &req.SenderName,
			ExtraData:  &extraData,
		}
		payloadBytes, _ := json.Marshal(payload)
		notifReq := NotificationRequest{
			ID:            uuid.New(),
			RecipientID:   recipient.RecipientID,
			RecipientType: recipient.RecipientType,
			SenderID:      &req.SenderID,
			SenderType:    &req.SenderType,
			EventType:     req.EventType,
			EntityType:    req.EntityType,
			EntityID:      req.EntityID,
			Status:        util.StatusUnread,
			Payload:       payloadBytes,
			Channels:      channels,
			CreatedAt:     time.Now(),
		}
		if err := p.notificationSvc.Publish(ctx, notifReq); err != nil {
			log.Printf("[ERROR] failed to publish %s notification to %s: %v", req.EventType, recipient.RecipientID, err)
		} else {
			log.Printf("[INFO] published %s notification to %s via channels: %v", req.EventType, recipient.RecipientID, channels)
		}
	}
	return nil
}

func (p *Publisher) PublishToMultiple(ctx context.Context, recipients []uuid.UUID, recipientType util.ActorType, senderID *uuid.UUID, senderType *util.ActorType, eventType util.EventType, entityType util.EntityType, entityID uuid.UUID, title, body string, extraData *map[string]interface{}) error {
	if p.notificationSvc == nil {
		return fmt.Errorf("notification service is nil")
	}
	bodyJSON, _ := json.Marshal(body)
	notifPayload := NotificationPayload{Title: title, Body: bodyJSON, ExtraData: extraData}
	payloadBytes, err := json.Marshal(notifPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	for _, recipientID := range recipients {
		req := NotificationRequest{
			ID:            uuid.New(),
			RecipientID:   recipientID,
			RecipientType: recipientType,
			SenderID:      senderID,
			SenderType:    senderType,
			EventType:     eventType,
			EntityType:    entityType,
			EntityID:      entityID,
			Status:        util.StatusUnread,
			Payload:       payloadBytes,
			Channels:      []util.Channel{util.ChannelInApp},
			CreatedAt:     time.Now(),
		}
		go func(r NotificationRequest) {
			pCtx, pCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer pCancel()
			if err := p.notificationSvc.Publish(pCtx, r); err != nil {
				log.Printf("[ERROR] failed to publish notification to %s: %v", r.RecipientID, err)
			}
		}(req)
	}
	return nil
}

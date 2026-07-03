package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
)

// ISyncService handles default template synchronization
type ISyncService interface {
	BulkSyncDefaults(ctx context.Context) error
}

type SyncService struct {
	templateRepo ITemplateRepository
	settingRepo  ISettingRepository
	encryption   IEncryptionService
}

func NewSyncService(
	templateRepo ITemplateRepository,
	settingRepo ISettingRepository,
	encryption IEncryptionService,
) ISyncService {
	return &SyncService{
		templateRepo: templateRepo,
		settingRepo:  settingRepo,
		encryption:   encryption,
	}
}

func (s *SyncService) BulkSyncDefaults(ctx context.Context) error {
	freshTemplates := template.DefaultTemplates()
	existingList, err := s.templateRepo.List(ctx, "")
	if err != nil {
		return fmt.Errorf("failed listing global templates: %w", err)
	}

	// Build map of existing templates by name
	existingMap := make(map[string]template.RsTemplate)
	if existingList != nil && existingList.Items != nil {
		switch items := existingList.Items.(type) {
		case []template.RsTemplate:
			for _, item := range items {
				existingMap[item.Name] = item
			}
		case []interface{}:
			for _, v := range items {
				if item, ok := v.(template.RsTemplate); ok {
					existingMap[item.Name] = item
				}
			}
		default:
			return fmt.Errorf("unexpected Items type in list response: %T", existingList.Items)
		}
	}

	// Update or create templates
	for _, freshRq := range freshTemplates {
		htmlBlob, cssBlob, err := s.encryption.EncryptTemplate(
			string(fixHTMLValue(freshRq.Html)),
			string(fixHTMLValue(freshRq.Css)),
		)
		if err != nil {
			return err
		}

		if existingMatched, exists := existingMap[freshRq.Name]; exists {
			// Update existing template
			t := template.Template{
				Id:        existingMatched.Id,
				Name:      freshRq.Name,
				Html:      htmlBlob,
				Css:       cssBlob,
				IsDefault: freshRq.IsDefault,
				IsActive:  freshRq.IsActive,
			}
			if err := s.templateRepo.Update(ctx, &t); err != nil {
				return fmt.Errorf("failed updating template '%s': %w", freshRq.Name, err)
			}
		} else {
			// Create new template
			t := template.Template{
				Name:      freshRq.Name,
				Html:      htmlBlob,
				Css:       cssBlob,
				IsDefault: freshRq.IsDefault,
				IsActive:  freshRq.IsActive,
			}
			if err := s.templateRepo.Create(ctx, &t); err != nil {
				return fmt.Errorf("failed creating template: %w", err)
			}
		}
	}

	// Sync global settings
	existingGlobalSetting, err := s.settingRepo.Get(ctx, uuid.Nil)
	if err != nil {
		return fmt.Errorf("failed to fetch global settings: %w", err)
	}

	globalSetting := template.DefaultSettings(uuid.Nil)
	globalSetting.InvoiceId = nil

	if existingGlobalSetting != nil {
		// Use existing ID for update
		globalSetting.Id = existingGlobalSetting.Id
	} else {
		// Create new ID
		globalSetting.Id = uuid.New()
	}

	if err := s.settingRepo.Update(ctx, &globalSetting, uuid.Nil); err != nil {
		return fmt.Errorf("failed to sync global settings: %w", err)
	}

	return nil
}

func fixHTMLValue(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	if bytes, ok := v.([]byte); ok {
		return string(bytes)
	}
	return ""
}

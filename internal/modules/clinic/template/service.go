package template

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/chromepdf"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IService interface {
	BulkCreate(ctx context.Context) (*[]RsTemplate, error)
	Create(ctx context.Context, rq RqGlobalTemplate) (*RsTemplate, error)
	Update(ctx context.Context, id uuid.UUID, rq RqGlobalTemplate) (*RsTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsTemplate, error)
	List(ctx context.Context) (*util.RsList, error)
	GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error)
	GetInvoiceSetting(ctx context.Context, clinicId, invoiceId, templateId uuid.UUID) (*RsSetting, error)
	UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error)
	GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error)
	DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error)
	BulkUpdateDefaults(ctx context.Context) error
}

type Service struct {
	repo          IRepository
	cfg           *config.Config
	encryptionKey []byte
}

func NewService(repo IRepository, cfg *config.Config) IService {
	if len(cfg.TemplateEncryptionKey) != 32 {
		panic(fmt.Sprintf("template service configuration error: key must be exactly 32 chars, got %d", len(cfg.TemplateEncryptionKey)))
	}
	return &Service{
		repo:          repo,
		cfg:           cfg,
		encryptionKey: []byte(cfg.TemplateEncryptionKey),
	}
}

func (s *Service) Create(ctx context.Context, rq RqGlobalTemplate) (*RsTemplate, error) {
	html, err := crypto.EncryptAndCompress(rq.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt layout stream: %w", err)
	}
	css, err := crypto.EncryptAndCompress(rq.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt styling stream: %w", err)
	}

	t := Template{
		Name:        rq.Name,
		Description: nil,
		Html:        html,
		Css:         css,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}

	if err := s.repo.Create(ctx, &t); err != nil {
		return nil, err
	}

	st := DefaultSettings(t.Id)
	st.MappingId = nil
	if err := s.repo.CreateSetting(ctx, &st); err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, rq RqGlobalTemplate) (*RsTemplate, error) {
	html, err := crypto.EncryptAndCompress(rq.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt layout stream: %w", err)
	}
	css, err := crypto.EncryptAndCompress(rq.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt styling stream: %w", err)
	}

	t := Template{
		Id:        id,
		Name:      rq.Name,
		Html:      html,
		Css:       css,
		IsDefault: rq.IsDefault,
		IsActive:  rq.IsActive,
	}

	if err := s.repo.Update(ctx, &t); err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*RsTemplate, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	rs := t.ToRs()
	rs.Html = base64.StdEncoding.EncodeToString(t.Html)
	rs.Css = base64.StdEncoding.EncodeToString(t.Css)
	return &rs, nil
}

// Internal helper function to enrich media file structures from cloud IDs
func (s *Service) enrichSettingDocuments(ctx context.Context, st *Setting) error {
	if st.LogoId != nil {
		logo, err := s.repo.GetDocumentByID(ctx, *st.LogoId)
		if err == nil && logo != nil {
			st.Logo = logo
		}
	}
	if st.LetterHeadId != nil {
		lh, err := s.repo.GetDocumentByID(ctx, *st.LetterHeadId)
		if err == nil && lh != nil {
			st.LetterHead = lh
		}
	}
	if st.FooterId != nil {
		f, err := s.repo.GetDocumentByID(ctx, *st.FooterId)
		if err == nil && f != nil {
			st.Footer = f
		}
	}
	return nil
}

// GetSetting retrieves the default template settings
func (s *Service) GetSetting(ctx context.Context, templateId uuid.UUID) (*RsSetting, error) {
	st, err := s.repo.GetSetting(ctx, templateId)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("global default setting configuration not provisioned for template: %s", templateId)
	}

	if err := s.enrichSettingDocuments(ctx, st); err != nil {
		return nil, err
	}

	rs := st.ToRs()
	return &rs, nil
}

// GetInvoiceSetting retrieves custom settings for an invoice template
func (s *Service) GetInvoiceSetting(ctx context.Context, clinicId, invoiceId, templateId uuid.UUID) (*RsSetting, error) {
	st, err := s.repo.GetInvoiceSetting(ctx, clinicId, invoiceId, templateId)
	if err != nil {
		return nil, fmt.Errorf("failed looking up configuration hierarchy matching target: %w", err)
	}
	if st == nil {
		return nil, fmt.Errorf("no operational setting baseline or default profiles discovered for parameters")
	}

	if err := s.enrichSettingDocuments(ctx, st); err != nil {
		return nil, err
	}

	rs := st.ToRs()
	return &rs, nil
}

func (s *Service) UpdateSetting(ctx context.Context, rq RqUpdateSetting) (*RsSetting, error) {
	st := rq.ToDB()
	if err := s.repo.UpdateSetting(ctx, &st, rq.TemplateId); err != nil {
		return nil, err
	}
	rs := st.ToRs()
	return &rs, nil
}

func (s *Service) BulkCreate(ctx context.Context) (*[]RsTemplate, error) {
	rqs := DefaultTemplates()
	templates := make([]Template, 0, len(rqs))

	for _, rq := range rqs {
		html, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(rq.Html)), s.encryptionKey)
		if err != nil {
			return nil, err
		}
		cssBlob, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(rq.Css)), s.encryptionKey)
		if err != nil {
			return nil, err
		}

		templates = append(templates, Template{
			Name:      rq.Name,
			Html:      html,
			Css:       cssBlob,
			IsDefault: rq.IsDefault,
			IsActive:  rq.IsActive,
		})
	}

	if err := s.repo.BulkCreate(ctx, templates); err != nil {
		return nil, err
	}

	for _, t := range templates {
		st := DefaultSettings(t.Id)
		st.MappingId = nil
		if err := s.repo.CreateSetting(ctx, &st); err != nil {
			return nil, err
		}
	}

	rs := make([]RsTemplate, 0, len(templates))
	for _, t := range templates {
		rsItem := t.ToRs()
		rsItem.Html = base64.StdEncoding.EncodeToString(t.Html)
		rsItem.Css = base64.StdEncoding.EncodeToString(t.Css)
		rs = append(rs, rsItem)
	}
	return &rs, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) (*util.RsList, error) {
	return s.repo.List(ctx)
}

func (s *Service) GeneratePDF(ctx context.Context, rq RqGeneratePDF) ([]byte, error) {
	t, err := s.repo.Get(ctx, rq.TemplateId)
	if err != nil {
		return nil, err
	}

	html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt html: %w", err)
	}
	css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt css: %w", err)
	}

	st, err := s.repo.GetSetting(ctx, rq.TemplateId)
	if err != nil {
		return nil, err
	}

	if st != nil {
		rq.Data.PrimaryColor = st.PrimaryColor
		rq.Data.AccentColor = st.AccentColor
		rq.Data.BodyFontFamily = st.BodyFontFamily
		rq.Data.HeaderFontFamily = st.HeaderFontFamily
		rq.Data.ShowTax = st.IsTax
		if st.TableStyle != nil {
			rq.Data.TableStyleClass = *st.TableStyle
		}
		if st.IsWaterMark && st.WaterMarkText != nil {
			rq.Data.WatermarkEnabled = true
			rq.Data.WatermarkText = *st.WaterMarkText
		}
	}

	dataMap, err := invoiceDataToMap(rq.Data)
	if err != nil {
		return nil, fmt.Errorf("failed mapping dynamic payload context: %w", err)
	}

	fullHTML, err := chromepdf.Render(html, css, dataMap)
	if err != nil {
		return nil, err
	}
	return chromepdf.Generate(ctx, fullHTML)
}

func (s *Service) DownloadPDF(ctx context.Context, clinicId uuid.UUID, templateId uuid.UUID, invoiceId uuid.UUID) ([]byte, string, error) {
	inv, err := s.repo.GetInvoice(ctx, clinicId, invoiceId)
	if err != nil {
		// FIXED: Surface explicit mismatch or query execution bubble-up errors cleanly
		if errors.Is(err, ErrInvoiceNotFound) {
			return nil, "", fmt.Errorf("failed to fetch invoice: invoice record matching id %s not found for clinic %s", invoiceId, clinicId)
		}
		return nil, "", fmt.Errorf("failed to fetch invoice: %w", err)
	}

	t, err := s.repo.Get(ctx, templateId)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch base template structural data: %w", err)
	}

	html, err := crypto.DecryptAndDecompress(t.Html, s.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt html: %w", err)
	}
	css, err := crypto.DecryptAndDecompress(t.Css, s.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt css: %w", err)
	}

	st, err := s.repo.GetInvoiceSetting(ctx, clinicId, invoiceId, templateId)
	if err != nil {
		return nil, "", err
	}

	data := invoiceToData(inv)

	if st != nil {
		data.PrimaryColor = st.PrimaryColor
		data.AccentColor = st.AccentColor
		data.BodyFontFamily = st.BodyFontFamily
		data.HeaderFontFamily = st.HeaderFontFamily
		data.ShowTax = st.IsTax
		if st.TableStyle != nil {
			data.TableStyleClass = *st.TableStyle
		}
		if st.IsWaterMark && st.WaterMarkText != nil {
			data.WatermarkEnabled = true
			data.WatermarkText = *st.WaterMarkText
		}
		if st.TermText != nil {
			data.Notes = *st.TermText
		}
		if st.IsLogo {
			data.ShowLogo = true
			if st.LogoId != nil {
				logo, err := s.repo.GetDocumentByID(ctx, *st.LogoId)
				if err == nil && logo != nil {
					data.ShowLogoImage = true
					data.LogoURL = s.cfg.R2StoragePrefix + logo.ToRsDocument().FileKey
				}
			} else if len(inv.ClinicName) > 0 {
				data.LogoInitial = string([]rune(inv.ClinicName)[0])
			}
		}
	}

	dataMap, err := invoiceDataToMap(data)
	if err != nil {
		return nil, "", fmt.Errorf("failed mapping static properties schema payload: %w", err)
	}

	fullHTML, err := chromepdf.Render(html, css, dataMap)
	if err != nil {
		return nil, "", err
	}

	pdf, err := chromepdf.Generate(ctx, fullHTML)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("INVOICE %s", inv.InvoiceNumber)
	return pdf, filename, nil
}

func (s *Service) BulkUpdateDefaults(ctx context.Context) error {
	freshTemplates := DefaultTemplates()
	existingList, err := s.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed listing global blueprints: %w", err)
	}

	existingMap := make(map[string]RsTemplate)
	if existingList != nil && existingList.Items != nil {
		if itemsSlice, ok := existingList.Items.([]RsTemplate); ok {
			for _, item := range itemsSlice {
				existingMap[item.Name] = item
			}
		}
	}

	for _, freshRq := range freshTemplates {
		htmlBlob, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(freshRq.Html)), s.encryptionKey)
		if err != nil {
			return err
		}
		cssBlob, err := crypto.EncryptAndCompress(string(freshRqHTMLFix(freshRq.Css)), s.encryptionKey)
		if err != nil {
			return err
		}

		if existingMatched, exists := existingMap[freshRq.Name]; exists {
			// CASE 1: OVERWRITE EXISTING TEMPLATE BLUEPRINT
			t := Template{
				Id:        existingMatched.Id,
				Name:      freshRq.Name,
				Html:      htmlBlob,
				Css:       cssBlob,
				IsDefault: freshRq.IsDefault,
				IsActive:  freshRq.IsActive,
			}
			if err := s.repo.Update(ctx, &t); err != nil {
				return fmt.Errorf("failed overwriting central design template text context '%s': %w", freshRq.Name, err)
			}

		} else {
			// CASE 2: PROVISION NEW TEMPLATE BLUEPRINT
			t := Template{
				Name:      freshRq.Name,
				Html:      htmlBlob,
				Css:       cssBlob,
				IsDefault: freshRq.IsDefault,
				IsActive:  freshRq.IsActive,
			}
			if err := s.repo.Create(ctx, &t); err != nil {
				return fmt.Errorf("failed tracking fresh template layout baseline: %w", err)
			}

			mappingID := uuid.New()
			settingID := uuid.New()

			st := DefaultSettings(t.Id)
			st.Id = settingID
			st.MappingId = &mappingID

			if err := s.repo.CreateSetting(ctx, &st); err != nil {
				return fmt.Errorf("failed tracking default options values profiles: %w", err)
			}

			m := Mapping{
				ID:         mappingID,
				InvoiceID:  nil, // Explicitly NULL because it represents a default blueprint configuration
				TemplateID: t.Id,
				SettingID:  settingID,
				ClinicID:   nil, // System-wide global default
			}

			if err := s.repo.CreateMapping(ctx, &m); err != nil {
				return fmt.Errorf("failed creating global default mapping linkage: %w", err)
			}
		}
	}
	return nil
}

func freshRqHTMLFix(v interface{}) string {
	if str, ok := v.(string); ok {
		return str
	}
	if bytes, ok := v.([]byte); ok {
		return string(bytes)
	}
	return ""
}

func invoiceDataToMap(data InvoiceData) (map[string]interface{}, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	if err := json.Unmarshal(bytes, &res); err != nil {
		return nil, err
	}
	return res, nil
}

package invoice

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	clinicauth "github.com/iamarpitzala/acareca/internal/modules/clinic/auth"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/section"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type IService interface {
	Create(ctx context.Context, invoice *RqInvoice) error
	Update(ctx context.Context, invoice *RqUpdateInvoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error)
	List(ctx context.Context, ft *Filter) (*util.RsList, error)
	GetClinicTemplate(ctx context.Context, clinicID uuid.UUID) (*RsInvoiceMailTemplate, error)
	SaveClinicTemplate(ctx context.Context, clinicID uuid.UUID, rq *RqSaveMailTemplate) error
	ResendInvoiceEmail(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	db         *sqlx.DB
	repo       IRepository
	cfg        *config.Config
	mailer     *mail.Client
	tplService template.IService
	clinicSvc  clinicauth.Service
	tplRepo    template.IRepository
	itemRepo   item.IRepository
}

func NewService(db *sqlx.DB, repo IRepository, cfg *config.Config, tplService template.IService, clinicSvc clinicauth.Service, tplRepo template.IRepository) IService {
	return &Service{
		db:         db,
		repo:       repo,
		cfg:        cfg,
		mailer:     mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
		tplService: tplService,
		clinicSvc:  clinicSvc,
		tplRepo:    tplRepo,
		itemRepo:   item.NewRepository(db),
	}
}

func (s *Service) Create(ctx context.Context, invoice *RqInvoice) error {
	if err := invoice.Validate(); err != nil {
		return err
	}

	inv := invoice.ToInvoice()

	if len(inv.Sections) == 0 {
		var cs section.CalculationStatement

		built, err := cs.Build(ctx, &inv.ID)
		if err != nil {
			return err
		}
		inv.Sections = []section.Section{built}
	}

	sections := make([]section.Section, 0, len(invoice.Sections))
	for _, rqSec := range invoice.Sections {
		sec := rqSec.ToSection()
		sections = append(sections, *sec)
	}

	// Flatten all entries (including nested children) for formula evaluation
	allEntries := make([]*item.Item, 0)
	for _, section := range sections {
		allEntries = append(allEntries, s.flattenEntries(section.Entries)...)
	}

	if len(allEntries) > 0 {
		if err := s.itemRepo.EvaluateFormulas(ctx, allEntries); err != nil {
			return fmt.Errorf("formula evaluation failed: %w", err)
		}
	}

	if err := s.repo.Create(ctx, inv); err != nil {
		return err
	}

	if invoice.Settings != nil {
		for _, sec := range sections {
			if sec.TemplateID != uuid.Nil {
				if err := s.syncTemplateSettings(ctx, inv.ID, inv.ClinicID, sec.TemplateID, invoice.Settings); err != nil {
					log.Printf("[SETTINGS-WARN] Failed propagating structural setting values: %v", err)
				}
			}
		}
	}

	return nil
}

// flattenEntries recursively flattens nested item structures into a flat slice
func (s *Service) flattenEntries(entries []*item.Item) []*item.Item {
	result := make([]*item.Item, 0)
	for _, entry := range entries {
		result = append(result, entry)
		if len(entry.Children) > 0 {
			result = append(result, s.flattenEntries(entry.Children)...)
		}
	}
	return result
}

// Delete implements [IService].
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Get implements [IService].
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error) {
	invoice, err := s.repo.GetByID(ctx, s.db, id)
	if err != nil {
		return nil, err
	}

	return invoice.ToRsInvoice(), nil
}

// List implements [IService].
func (s *Service) List(ctx context.Context, filter *Filter) (*util.RsList, error) {

	ft := filter.MapToFilter()
	invoices, total, err := s.repo.List(ctx, ft)
	if err != nil {
		return nil, err
	}

	rsInvoices := make([]*RsInvoiceSummary, 0, len(invoices))
	for _, invoice := range invoices {
		rsInvoices = append(rsInvoices, invoice.ToRsInvoiceSummary())
	}

	var rsList util.RsList
	rsList.MapToList(rsInvoices, int(total), *filter.Offset, *filter.Limit)
	return &rsList, nil
}

func (s *Service) Update(ctx context.Context, invoice *RqUpdateInvoice) error {
	if err := invoice.Validate(); err != nil {
		return err
	}

	existing, err := s.repo.GetByID(ctx, s.db, *invoice.ID)
	if err != nil {
		return err
	}

	updatedInvoice := invoice.ApplyToInvoice(existing)

	sections := make([]section.Section, 0, len(invoice.Sections))
	deleteItemIDs := make(map[uuid.UUID][]uuid.UUID)

	for _, rqSec := range invoice.Sections {
		sec := rqSec.ToSection()
		sec.InvoiceID = invoice.ID
		sections = append(sections, *sec)

		if len(rqSec.DeleteEntries) > 0 {
			deleteItemIDs[sec.ID] = rqSec.DeleteEntries
		}
	}

	// Flatten all entries (including nested children) for formula evaluation
	allEntries := make([]*item.Item, 0)
	for _, section := range sections {
		allEntries = append(allEntries, s.flattenEntries(section.Entries)...)
	}

	if len(allEntries) > 0 {
		if err := s.itemRepo.EvaluateFormulas(ctx, allEntries); err != nil {
			return fmt.Errorf("formula evaluation failed: %w", err)
		}
	}

	err = s.repo.UpdateWithSections(ctx, updatedInvoice, sections, invoice.DeleteSections, deleteItemIDs)
	if err != nil {
		return err
	}

	if invoice.Settings != nil {
		for _, sec := range sections {
			if sec.TemplateID != uuid.Nil {
				if err := s.syncTemplateSettings(ctx, *invoice.ID, invoice.ClinicID, sec.TemplateID, invoice.Settings); err != nil {
					log.Printf("[SETTINGS-WARN] Failed syncing updated layout template adjustments: %v", err)
				}
			}
		}
	}

	return nil
}

func (s *Service) GetClinicTemplate(ctx context.Context, clinicID uuid.UUID) (*RsInvoiceMailTemplate, error) {
	dbSubject, dbBody, err := s.repo.GetSavedClinicMailTemplate(ctx, clinicID)
	if err != nil {
		dbSubject, dbBody = "", ""
	}

	subject, body, isCustom := mail.GetTemplateContext(dbSubject, dbBody)

	return &RsInvoiceMailTemplate{
		Subject:  subject,
		Body:     body,
		IsCustom: isCustom,
	}, nil
}

func (s *Service) SaveClinicTemplate(ctx context.Context, clinicID uuid.UUID, rq *RqSaveMailTemplate) error {
	return s.repo.SaveClinicMailTemplate(ctx, clinicID, rq.Subject, rq.Body)
}

func (s *Service) ResendInvoiceEmail(ctx context.Context, id uuid.UUID) error {
	hydrated, err := s.repo.GetByID(ctx, s.db, id)
	if err != nil {
		return err
	}

	rsInvoice := hydrated.ToRsInvoice()

	if rsInvoice.ContactTo == nil || rsInvoice.ContactTo.Email == "" {
		return errors.New("cannot resend: missing contact email field")
	}

	pdfBase64, err := s.compileInvoicePDF(ctx, rsInvoice)
	if err != nil {
		return fmt.Errorf("failed to generate invoice attachment document: %w", err)
	}

	dbSubject, dbBody, err := s.repo.GetSavedClinicMailTemplate(ctx, rsInvoice.ClinicID)
	if err != nil {
		dbSubject, dbBody = "", ""
	}

	chosenSubject, chosenBody, _ := mail.GetTemplateContext(dbSubject, dbBody)
	name := rsInvoice.ContactTo.Fname + " " + rsInvoice.ContactTo.Lname

	var documentNumber string
	for _, sec := range rsInvoice.Sections {
		documentNumber = sec.DocumentNumber
	}

	subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, documentNumber)

	go func(to, invNum, sub, html, pdf string) {
		if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
			log.Printf("[MAIL-ERR] Running async template mail worker failed processing invoice task context: %v", err)
		}
	}(rsInvoice.ContactTo.Email, documentNumber, subject, htmlBody, pdfBase64)

	return nil
}

func (s *Service) compileInvoicePDF(ctx context.Context, inv *RsInvoice) (string, error) {
	templateIDs := make([]uuid.UUID, 0, len(inv.Sections))
	for _, sec := range inv.Sections {
		if sec.TemplateID != uuid.Nil {
			templateIDs = append(templateIDs, sec.TemplateID)
		}
	}

	if len(templateIDs) == 0 {
		return "", errors.New("invoice has no template configured")
	}

	pdfBytes, _, err := s.tplService.DownloadPDF(ctx, inv.ClinicID, templateIDs, inv.ID)
	if err != nil {
		return "", fmt.Errorf("failed calling shared uniform download method engine: %w", err)
	}

	return base64.StdEncoding.EncodeToString(pdfBytes), nil
}

// Internal bridge mapper to parse and route structural customization configurations safely
func (s *Service) syncTemplateSettings(ctx context.Context, invoiceID uuid.UUID, clinicID uuid.UUID, templateID uuid.UUID, src *RqInvoiceSetting) error {

	existingSetting, err := s.tplRepo.GetInvoiceSetting(ctx, clinicID, invoiceID, []uuid.UUID{templateID})
	if err != nil {
		return fmt.Errorf("failed verifying database settings mapping context: %w", err)
	}

	var existingMapping *template.Mapping
	if existingSetting != nil && existingSetting.MappingId != nil {
		existingMapping, err = s.tplRepo.GetMapping(ctx, *existingSetting.MappingId)
		if err != nil {
			return fmt.Errorf("failed verifying transactional database junction state: %w", err)
		}
	}

	var targetSettingID uuid.UUID
	var targetMappingID uuid.UUID
	isNewOverride := true

	if existingMapping != nil && existingMapping.InvoiceID != nil && *existingMapping.InvoiceID == invoiceID {
		isNewOverride = false
		targetMappingID = existingMapping.ID
		targetSettingID = existingMapping.SettingID
	} else {
		targetSettingID = uuid.New()
		targetMappingID = uuid.New()
	}

	dbSetting := template.Setting{
		Id:               targetSettingID,
		TemplateId:       templateID,
		PrimaryColor:     lo.FromPtr(src.PrimaryColor),
		AccentColor:      lo.FromPtr(src.AccentColor),
		BodyFontFamily:   lo.FromPtr(src.BodyFontFamily),
		HeaderFontFamily: lo.FromPtr(src.HeaderFontFamily),
		IsLogo:           lo.FromPtr(src.IsLogo),
		LogoId:           src.LogoID,
		LetterHeadId:     src.LetterheadID,
		FooterId:         src.FooterID,
		TermText:         src.TermsText,
		IsWaterMark:      lo.FromPtr(src.IsWatermark),
		WaterMarkText:    src.WatermarkText,
		IsTax:            lo.FromPtr(src.IsTax),
		TableStyle:       src.TableStyle,
		MappingId:        &targetMappingID,
	}

	if isNewOverride {
		if err := s.tplRepo.CreateSetting(ctx, &dbSetting); err != nil {
			return fmt.Errorf("failed allocating specific template layout profile: %w", err)
		}

		m := template.Mapping{
			ID:         targetMappingID,
			InvoiceID:  &invoiceID,
			TemplateID: templateID,
			SettingID:  targetSettingID,
			ClinicID:   &clinicID,
		}
		if err := s.tplRepo.CreateMapping(ctx, &m); err != nil {
			return fmt.Errorf("failed linking relational structural mapping context: %w", err)
		}
	} else {
		if err := s.tplRepo.UpdateSetting(ctx, &dbSetting, targetSettingID); err != nil {
			return fmt.Errorf("failed overwriting specific layout configuration settings profile: %w", err)
		}
	}

	return nil
}

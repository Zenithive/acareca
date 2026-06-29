package invoice

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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
	}
}

func (s *Service) Create(ctx context.Context, invoice *RqInvoice) error {
	inv := invoice.ToInvoice()

	if len(inv.Sections) == 0 {
		currentYear := strconv.Itoa(time.Now().Year())

		docString, err := s.repo.GetNextSequenceForYear(ctx, "CS", currentYear)
		if err != nil {
			return fmt.Errorf("failed calculating consecutive invoice numbers: %w", err)
		}

		cs := section.CalculationStatement{
			DocumentNumber: docString,
			Entries:        []*item.Item{},
		}

		built, err := cs.Build(ctx, &inv.ID, docString)
		if err != nil {
			return err
		}
		inv.Sections = []section.Section{built}
	}

	itemRepo := item.NewRepository(s.db)
	allEntries := make([]*item.Item, 0)
	for i := range inv.Sections {
		allEntries = append(allEntries, inv.Sections[i].Entries...)
	}
	if len(allEntries) > 0 {
		if err := itemRepo.EvaluateFormulas(ctx, allEntries); err != nil {
			return fmt.Errorf("formula evaluation failed: %w", err)
		}
	}

	if err := s.repo.Create(ctx, inv); err != nil {
		return err
	}

	if invoice.Settings != nil {
		for _, sec := range inv.Sections {
			if sec.TemplateID != uuid.Nil {
				if err := s.syncTemplateSettings(ctx, inv.ID, inv.ClinicID, sec.TemplateID, invoice.Settings); err != nil {
					log.Printf("[SETTINGS-WARN] Failed propagating structural setting values: %v", err)
				}
			}
		}
	}

	return nil
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

	page := 1
	if ft.Offset != nil && ft.Limit != nil && *ft.Limit > 0 {
		page = (*ft.Offset / *ft.Limit) + 1
	}

	var rsList util.RsList
	rsList.MapToList(rsInvoices, int(total), page, *ft.Limit)
	return &rsList, nil
}

func (s *Service) Update(ctx context.Context, invoice *RqUpdateInvoice) error {
	existing, err := s.repo.GetByID(ctx, s.db, *invoice.ID)
	if err != nil {
		return err
	}

	var wasPaid bool
	if existing.Status != nil {
		wasPaid = (*existing.Status == "paid")
	}

	updatedInvoice := invoice.ApplyToInvoice(existing)

	sections := make([]section.Section, 0)
	deleteItemIDs := make(map[uuid.UUID][]uuid.UUID)

	for _, rqSec := range invoice.Sections {
		sec := rqSec.ToSection()
		sec.InvoiceID = invoice.ID
		sections = append(sections, *sec)

		if len(rqSec.DeleteEntries) > 0 {
			deleteItemIDs[sec.ID] = rqSec.DeleteEntries
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

	// Fetch fully loaded data row from db to get client fields securely
	hydrated, err := s.repo.GetByID(ctx, s.db, *invoice.ID)
	if err != nil {
		return err
	}

	// AUTOMATED TRIGGER: Fires when state flips to paid
	if hydrated.Status != nil && *hydrated.Status == "paid" && !wasPaid {
		if hydrated.ContactTo != nil && hydrated.ContactTo.Email != "" {

			rsInvoice := hydrated.ToRsInvoice()

			pdfBase64, err := s.compileInvoicePDF(ctx, rsInvoice)
			if err != nil {
				log.Printf("[PDF-WARN] Skipping attachment compilation error trace: %v", err)
			}

			name := rsInvoice.ContactTo.Fname + " " + rsInvoice.ContactTo.Lname
			dbSubject, dbBody, _ := s.repo.GetSavedClinicMailTemplate(ctx, rsInvoice.ClinicID)
			chosenSubject, chosenBody, _ := mail.GetTemplateContext(dbSubject, dbBody)
			var documentNumber string
			for _, sec := range rsInvoice.Sections {
				if strings.ToUpper(strings.TrimSpace(string(sec.SectionType))) == "CALCULATION_STATEMENT" && sec.DocumentNumber != "" {
					documentNumber = sec.DocumentNumber
					break
				}
			}
			// Fallback if Calculation Statement is unpopulated
			if documentNumber == "" {
				if len(rsInvoice.Sections) > 0 && rsInvoice.Sections[0].DocumentNumber != "" {
					documentNumber = rsInvoice.Sections[0].DocumentNumber
				} else {
					documentNumber = rsInvoice.ID.String()[:8]
				}
			}
			subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, documentNumber)

			go func(to, invNum, sub, html, pdf string) {
				if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
					log.Printf("[MAIL-ERR] Firing automated payment confirmation receipt failed: %v", err)
				}
			}(rsInvoice.ContactTo.Email, documentNumber, subject, htmlBody, pdfBase64)
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
		if strings.ToUpper(strings.TrimSpace(string(sec.SectionType))) == "CALCULATION_STATEMENT" && sec.DocumentNumber != "" {
			documentNumber = sec.DocumentNumber
			break
		}
	}

	// Fallback if Calculation Statement is unpopulated
	if documentNumber == "" {
		if len(rsInvoice.Sections) > 0 && rsInvoice.Sections[0].DocumentNumber != "" {
			documentNumber = rsInvoice.Sections[0].DocumentNumber
		} else {
			documentNumber = rsInvoice.ID.String()[:8]
		}
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
	parseUUID := func(str *string) *uuid.UUID {
		if str == nil || *str == "" {
			return nil
		}
		if parsed, err := uuid.Parse(*str); err == nil {
			return &parsed
		}
		return nil
	}

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
		LogoId:           parseUUID(src.LogoID),
		LetterHeadId:     parseUUID(src.LetterheadID),
		FooterId:         parseUUID(src.FooterID),
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

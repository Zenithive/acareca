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
}

func NewService(db *sqlx.DB, repo IRepository, cfg *config.Config, tplService template.IService, clinicSvc clinicauth.Service) IService {
	return &Service{
		db:         db,
		repo:       repo,
		cfg:        cfg,
		mailer:     mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
		tplService: tplService,
		clinicSvc:  clinicSvc,
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

	allItems := make([]*item.Item, 0)
	for i := range inv.Sections {
		allItems = append(allItems, inv.Sections[i].Entries...)
	}

	if len(allItems) > 0 {
		itemRepo := item.NewRepository(s.db)
		if err := itemRepo.EvaluateFormulas(ctx, allItems); err != nil {
			return fmt.Errorf("formula evaluation failed: %w", err)
		}
	}

	return s.repo.Create(ctx, inv)
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
			if len(rsInvoice.Sections) > 0 && rsInvoice.Sections[0].DocumentNumber != "" {
				documentNumber = rsInvoice.Sections[0].DocumentNumber
			} else {
				documentNumber = rsInvoice.ID.String()[:8] // Fallback
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
	if len(rsInvoice.Sections) > 0 && rsInvoice.Sections[0].DocumentNumber != "" {
		documentNumber = rsInvoice.Sections[0].DocumentNumber
	} else {
		documentNumber = rsInvoice.ID.String()[:8] // Fallback
	}

	subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, documentNumber)

	go func(to, invNum, sub, html, pdf string) {
		if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
			log.Printf("[MAIL-ERR] Running async template mail worker failed processing invoice task context: %v", err)
		}
	}(rsInvoice.ContactTo.Email, documentNumber, subject, htmlBody, pdfBase64)

	return nil
}

// Internal Backend Helper: Maps RsInvoice data models directly into template.RqGeneratePDF configurations
func (s *Service) compileInvoicePDF(ctx context.Context, inv *RsInvoice) (string, error) {
	// --- BillTo from contact ---
	var billToName, billToEmail, billToPhone, billToABN, billToAddress string
	if inv.ContactTo != nil {
		billToName = strings.TrimSpace(inv.ContactTo.Fname + " " + inv.ContactTo.Lname)
		billToEmail = inv.ContactTo.Email
		billToPhone = inv.ContactTo.Phone
		billToABN = inv.ContactTo.ABN
		if len(inv.ContactTo.Address) > 0 {
			addr := inv.ContactTo.Address[0]
			for _, a := range inv.ContactTo.Address {
				if a.IsPrimary {
					addr = a
					break
				}
			}
			parts := []string{addr.AddressLine1}
			if addr.AddressLine2 != nil && *addr.AddressLine2 != "" {
				parts = append(parts, *addr.AddressLine2)
			}
			parts = append(parts, addr.City, addr.State, addr.PostalCode, addr.Country)
			billToAddress = strings.Join(parts, ", ")
		}
	}

	// --- BillFrom + clinic meta from clinicSvc ---
	var clinicName, billFromName, billFromABN, billFromEmail, billFromPhone, billFromAddress string
	if s.clinicSvc != nil {
		clinic, err := s.clinicSvc.GetProfile(ctx, inv.ClinicID)
		if err == nil && clinic != nil {
			clinicName = clinic.ClinicName
			billFromName = clinic.ClinicName
			if clinic.ABN != nil {
				billFromABN = *clinic.ABN
			}
			if len(clinic.Addresses) > 0 {
				addr := clinic.Addresses[0]
				for _, a := range clinic.Addresses {
					if a.IsPrimary {
						addr = a
						break
					}
				}
				parts := []string{}
				if addr.Address != "" {
					parts = append(parts, addr.Address)
				}
				if addr.City != "" {
					parts = append(parts, addr.City)
				}
				if addr.State != "" {
					parts = append(parts, addr.State)
				}
				if addr.Postcode != "" {
					parts = append(parts, addr.Postcode)
				}
				billFromAddress = strings.Join(parts, ", ")
			}
			for _, c := range clinic.Contacts {
				if c.ContactType == "PHONE" {
					if billFromPhone == "" {
						billFromPhone = c.Value
					}
				}
			}
			billFromEmail = clinic.Email
		}
	}

	// --- Line items ---
	var pdfItems []template.LineItem
	var grandTotal float64

	for _, sec := range inv.Sections {
		for _, it := range sec.Entries {
			var desc string
			if it.Description != nil {
				desc = *it.Description
			}
			pdfItems = append(pdfItems, template.LineItem{
				Name:        it.Name,
				Description: desc,
				LineTotal:   it.Amount,
			})
			grandTotal += it.Amount
		}
	}

	// --- Template settings: logo, letterhead, footer, terms ---
	var showLogo, showLogoImage bool
	var logoURL, logoInitial, letterheadHTML, footerHTML, notes string

	tplSetting, err := s.tplService.GetSetting(ctx, inv.TemplateID)
	if err != nil {
		// Log the warning instead of breaking the entire execution context pipeline
		log.Printf("[PDF-WARN] Specified TemplateID %s not found, falling back to default theme layout. Err: %v", inv.TemplateID, err)

		// Resilient Fallback: Generate generic text initials for the header logo identity
		showLogo = true
		if clinicName != "" {
			runes := []rune(clinicName)
			if len(runes) > 0 {
				logoInitial = string(runes[0])
			}
		}
	} else if tplSetting != nil {
		if tplSetting.IsLogo {
			showLogo = true
			if tplSetting.Logo != nil && tplSetting.Logo.FileKey != "" {
				logoURL = strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + tplSetting.Logo.FileKey
				showLogoImage = true
			} else if clinicName != "" {
				runes := []rune(clinicName)
				if len(runes) > 0 {
					logoInitial = string(runes[0])
				}
			}
		}
		if tplSetting.LetterHead != nil && tplSetting.LetterHead.FileKey != "" {
			letterheadHTML = `<img src="` + strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + tplSetting.LetterHead.FileKey + `" style="width:100%;" />`
		}
		if tplSetting.Footer != nil && tplSetting.Footer.FileKey != "" {
			footerHTML = `<img src="` + strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + tplSetting.Footer.FileKey + `" style="width:100%;" />`
		}
		if tplSetting.TermText != nil {
			notes = *tplSetting.TermText
		}
	}

	issueDateFormatted := inv.IssueDate.Format("02 January 2006")
	dueDateFormatted := ""
	if inv.DueDate != nil {
		dueDateFormatted = inv.DueDate.Format("02 January 2006")
	}
	billingPeriodFormatted := inv.BillingPeriodFrom + " to " + inv.BillingPeriodTo

	tableStyleClass := ""
	if tplSetting != nil {
		tableStyleClass = tplSetting.TableStyle
	}

	pdfRq := template.RqGeneratePDF{
		ClinicId:   inv.ClinicID,
		TemplateId: inv.TemplateID,
		Data: template.InvoiceData{
			ClinicName:       clinicName,
			IssueDateDisplay: issueDateFormatted,
			DueDateDisplay:   dueDateFormatted,
			BillingPeriod:    billingPeriodFormatted,
			InvoiceFrequency: lo.FromPtrOr(inv.InvoiceFrequency, ""),
			ShowLogo:         showLogo,
			ShowLogoImage:    showLogoImage,
			LogoURL:          logoURL,
			LogoInitial:      logoInitial,
			LetterheadHTML:   letterheadHTML,
			FooterHTML:       footerHTML,
			Notes:            notes,
			BillFrom: template.PartyInfo{
				Name:    billFromName,
				Address: billFromAddress,
				ABN:     billFromABN,
				Email:   billFromEmail,
				Phone:   billFromPhone,
			},
			BillTo: template.PartyInfo{
				Name:    billToName,
				Address: billToAddress,
				ABN:     billToABN,
				Email:   billToEmail,
				Phone:   billToPhone,
			},
			Items:      pdfItems,
			GrandTotal: grandTotal,

			TotalsAmountsCaption: "All amounts in AUD · Tax inclusive (GST included)",
			TotalsGrandLabel:     "Total (AUD)",
			WatermarkEnabled:     tplSetting.IsWaterMark,
			WatermarkText:        lo.FromPtr(tplSetting.WaterMarkText),
			ShowTax:              tplSetting.IsTax,
			TableStyleClass:      tableStyleClass,

			// Core CSS Layout variable bindings
			PrimaryColor:     tplSetting.PrimaryColor,
			AccentColor:      tplSetting.AccentColor,
			BodyFontFamily:   tplSetting.BodyFontFamily,
			HeaderFontFamily: tplSetting.HeaderFontFamily,
		},
	}

	pdfBytes, err := s.tplService.GeneratePDF(ctx, pdfRq)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pdfBytes), nil
}

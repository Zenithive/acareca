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

	itemRepo := item.NewRepository(s.db)
	for i := range inv.Sections {
		if len(inv.Sections[i].Entries) > 0 {
			if err := itemRepo.EvaluateFormulas(ctx, inv.Sections[i].Entries); err != nil {
				return fmt.Errorf("formula evaluation failed for section %d: %w", i, err)
			}
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

func (s *Service) compileInvoicePDF(ctx context.Context, inv *RsInvoice) (string, error) {
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

	var patientFeeItems []map[string]interface{}
	var serviceFeeItems []map[string]interface{}
	var settlementItems []map[string]interface{}
	var taxInvoiceItems []map[string]interface{}
	var remittanceItems []map[string]interface{}

	var globalLineItems []template.LineItem
	var subtotal, taxTotal, grandTotal float64
	var customFeeRate string = "0"

	var invoiceNumber string = inv.Name
	var paymentMethod, accountName, bsb, accountNumber, paymentDate, paymentRef string

	for _, sec := range inv.Sections {
		if sec.DocumentNumber != "" {
			invoiceNumber = sec.DocumentNumber
		}
		if sec.PaymentMethod != nil {
			paymentMethod = *sec.PaymentMethod
		}
		if sec.AccountName != nil {
			accountName = *sec.AccountName
		}
		if sec.Bsb != nil {
			bsb = *sec.Bsb
		}
		if sec.AccountNumber != nil {
			accountNumber = *sec.AccountNumber
		}
		if sec.PaymentDate != nil {
			paymentDate = *sec.PaymentDate
		}
		if sec.PaymentReference != nil {
			paymentRef = *sec.PaymentReference
		}

		for _, it := range sec.Entries {
			var desc string
			if it.Description != nil {
				desc = *it.Description
			}
			var basStr string
			if it.BASCode != nil {
				basStr = string(*it.BASCode)
			}
			var typeStr string
			if it.EntryType != nil {
				typeStr = string(*it.EntryType)
			}
			var keyStr string
			if it.FieldKey != nil {
				keyStr = *it.FieldKey
			}

			isCredit := strings.ToUpper(typeStr) == "CREDIT"

			itemMap := map[string]interface{}{
				"label":       it.Name,
				"description": desc,
				"amount":      it.Amount,
				"bas_code":    basStr,
				"entry_type":  typeStr,
				"row_class":   "",
				"value_class": "",
			}

			if it.IsFinal {
				itemMap["row_class"] = "row-final-balance"
			}

			globalLineItems = append(globalLineItems, template.LineItem{
				Name:         it.Name,
				Description:  desc,
				Amount:       it.Amount,
				RunningTotal: grandTotal + it.Amount,
			})

			switch sec.SectionType {
			case "CALCULATION_STATEMENT":
				if strings.Contains(strings.ToUpper(keyStr), "FACILITY") || strings.Contains(strings.ToUpper(keyStr), "SERVICE") {
					if strings.Contains(strings.ToUpper(keyStr), "RATE") && customFeeRate == "0" {
						customFeeRate = fmt.Sprintf("%.1f", it.Amount)
					}
					serviceFeeItems = append(serviceFeeItems, itemMap)
				} else if strings.Contains(strings.ToUpper(keyStr), "SETTLE") || strings.Contains(strings.ToUpper(keyStr), "NET") || it.IsFinal {
					itemMap["is_bold"] = true
					if isCredit {
						itemMap["is_negative"] = true
					}
					settlementItems = append(settlementItems, itemMap)
				} else {
					patientFeeItems = append(patientFeeItems, itemMap)
				}

			case "SFA_INVOICE":
				itemGst := 0.0
				itemSubtotal := it.Amount
				if basStr == "G1" {
					itemSubtotal = it.Amount / 1.1
					itemGst = it.Amount - itemSubtotal
				}

				taxInvoiceItems = append(taxInvoiceItems, map[string]interface{}{
					"description": fmt.Sprintf("<strong>%s</strong><br/>%s", it.Name, desc),
					"amount":      itemSubtotal,
					"gst":         itemGst,
					"row_class":   itemMap["row_class"],
				})

				subtotal += itemSubtotal
				taxTotal += itemGst
				grandTotal += it.Amount

			case "REMITTANCE_INVOICE":
				if isCredit {
					itemMap["is_negative"] = true
				}
				remittanceItems = append(remittanceItems, itemMap)
			}
		}
	}

	if len(taxInvoiceItems) == 0 {
		for _, sec := range inv.Sections {
			for _, it := range sec.Entries {
				var desc string
				if it.Description != nil {
					desc = *it.Description
				}
				var basStr string
				if it.BASCode != nil {
					basStr = string(*it.BASCode)
				}

				itemGst := 0.0
				itemSubtotal := it.Amount
				if basStr == "G1" {
					itemSubtotal = it.Amount / 1.1
					itemGst = it.Amount - itemSubtotal
				}

				taxInvoiceItems = append(taxInvoiceItems, map[string]interface{}{
					"description": fmt.Sprintf("<strong>%s</strong><br/>%s", it.Name, desc),
					"amount":      itemSubtotal,
					"gst":         itemGst,
				})
				subtotal += itemSubtotal
				taxTotal += itemGst
				grandTotal += it.Amount
			}
		}
	}

	var primaryTemplateID uuid.UUID
	if len(inv.Sections) > 0 {
		primaryTemplateID = inv.Sections[0].TemplateID
	}

	var showLogo, showLogoImage bool
	var logoURL, logoInitial, letterheadHTML, footerHTML, notes string
	var tableStyleClass string

	watermarkText := "PAID"
	watermarkEnabled := false
	showTax := true
	primaryColor := "#1f4e5f"
	accentColor := "#1f4e5f"
	bodyFontFamily := "Arial"
	headerFontFamily := "Arial"

	tplSetting, err := s.tplService.GetSetting(ctx, primaryTemplateID)
	if err != nil {
		log.Printf("[PDF-WARN] Specified Template settings not found, error: %v", err)
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

		tableStyleClass = tplSetting.TableStyle
		watermarkEnabled = tplSetting.IsWaterMark
		if tplSetting.WaterMarkText != nil {
			watermarkText = *tplSetting.WaterMarkText
		}
		showTax = tplSetting.IsTax
		primaryColor = tplSetting.PrimaryColor
		accentColor = tplSetting.AccentColor
		bodyFontFamily = tplSetting.BodyFontFamily
		headerFontFamily = tplSetting.HeaderFontFamily
	}

	issueDateFormatted := inv.IssueDate.Format("02 January 2006")
	dueDateFormatted := ""
	if inv.DueDate != nil {
		dueDateFormatted = inv.DueDate.Format("02 January 2006")
	}
	billingPeriodFormatted := template.FormatDateString(inv.BillingPeriodFrom) + " to " + template.FormatDateString(inv.BillingPeriodTo)

	templateSettingsPayload := map[string]interface{}{
		"is_logo":            showLogo,
		"primary_color":      primaryColor,
		"accent_color":       accentColor,
		"body_font_family":   bodyFontFamily,
		"header_font_family": headerFontFamily,
		"is_watermark":       watermarkEnabled,
		"watermark_text":     watermarkText,
		"is_tax":             showTax,
		"terms_text":         notes,
	}

	if paymentRef == "" {
		paymentRef = invoiceNumber
	}

	pdfRq := template.RqGeneratePDF{
		TemplateId: primaryTemplateID,
		ClinicId:   inv.ClinicID,
		Data: template.InvoiceData{
			InvoiceNumber:    invoiceNumber,
			ClinicName:       clinicName,
			IssueDateDisplay: issueDateFormatted,
			DueDateDisplay:   dueDateFormatted,
			BillingPeriod:    billingPeriodFormatted,
			InvoiceFrequency: lo.FromPtrOr(inv.InvoiceFrequency, "MONTHLY"),
			ShowLogo:         showLogo,
			ShowLogoImage:    showLogoImage,
			LogoURL:          logoURL,
			LogoInitial:      logoInitial,
			WatermarkEnabled: watermarkEnabled,
			WatermarkText:    watermarkText,
			ShowTax:          showTax,
			LetterheadHTML:   letterheadHTML,
			FooterHTML:       footerHTML,
			Notes:            notes,
			TableStyleClass:  tableStyleClass,
			TemplateSettings: templateSettingsPayload,
			PrimaryColor:     primaryColor,
			AccentColor:      accentColor,
			BodyFontFamily:   bodyFontFamily,
			HeaderFontFamily: headerFontFamily,

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

			Items:                globalLineItems,
			GrandTotal:           grandTotal,
			Subtotal:             subtotal,
			TaxTotal:             taxTotal,
			TotalsAmountsCaption: "All amounts in AUD · Tax inclusive (GST included)",
			TotalsGrandLabel:     "Total (AUD)",

			PatientFeeItems: patientFeeItems,
			ServiceFeeItems: serviceFeeItems,
			SettlementItems: settlementItems,
			CustomFeeRate:   customFeeRate,
			TaxInvoiceItems: taxInvoiceItems,
			TermsText:       notes,

			RemittanceItems:          remittanceItems,
			CustomPaymentMethod:      paymentMethod,
			PaymentMethodLabel:       paymentMethod,
			CustomPaymentAccountName: accountName,
			CustomPaymentBsb:         bsb,
			CustomPaymentAccount:     accountNumber,
			PaymentDateDisplay:       template.FormatDateString(paymentDate),
		},
	}

	// We use paymentRef context mapping to safely ensure it isn't labeled as an unused compilation item
	if pdfRq.Data.TemplateSettings != nil {
		pdfRq.Data.TemplateSettings["payment_reference_id"] = paymentRef
	}

	pdfBytes, err := s.tplService.GeneratePDF(ctx, pdfRq)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pdfBytes), nil
}

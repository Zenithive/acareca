package invoice

import (
	"context"

	"github.com/google/uuid"
	clinicauth "github.com/iamarpitzala/acareca/internal/modules/clinic/auth"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	Create(ctx context.Context, invoice *RqInvoice) error
	Update(ctx context.Context, invoice *RqUpdateInvoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error)
	List(ctx context.Context, clinicID uuid.UUID, ft *Filter) (*util.RsList, error)
	// GetClinicTemplate(ctx context.Context, clinicID uuid.UUID) (*RsInvoiceMailTemplate, error)
	// SaveClinicTemplate(ctx context.Context, clinicID uuid.UUID, rq *RqSaveMailTemplate) error
	// ResendInvoiceEmail(ctx context.Context, id uuid.UUID) error
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

// Create implements [IService].
func (s *Service) Create(ctx context.Context, invoice *RqInvoice) error {
	return s.repo.Create(ctx, invoice.ToInvoice())
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
func (s *Service) List(ctx context.Context, clinicID uuid.UUID, filter *Filter) (*util.RsList, error) {
	ft := filter.MapToFilter()

	invoices, total, err := s.repo.List(ctx, clinicID, ft)
	if err != nil {
		return nil, err
	}

	rsInvoices := make([]*RsInvoice, 0, len(invoices))
	for _, invoice := range invoices {
		rsInvoices = append(rsInvoices, invoice.ToRsInvoice())
	}

	var rsList util.RsList
	rsList.MapToList(rsInvoices, int(total), *ft.Offset, *ft.Limit)
	return &rsList, nil
}

func (s *Service) Update(ctx context.Context, invoice *RqUpdateInvoice) error {
	existing, err := s.repo.GetByID(ctx, s.db, *invoice.ID)
	if err != nil {
		return err
	}

	// var wasPaid bool
	// if existing.Status != nil {
	// 	wasPaid = (*existing.Status == "paid")
	// }

	updatedInvoice := invoice.ApplyToInvoice(existing)
	err = s.repo.Update(ctx, updatedInvoice)
	if err != nil {
		return err
	}

	// Fetch fully loaded data row from db to get client fields securely
	// hydrated, err := s.repo.GetByID(ctx, s.db, *invoice.ID)
	// if err != nil {
	// 	return err
	// }

	// AUTOMATED TRIGGER: Fires when state flips to paid
	// if hydrated.Status != nil && *hydrated.Status == "paid" && !wasPaid {
	// 	if hydrated.ContactTo != nil && hydrated.ContactTo.Email != "" {

	// 		rsInvoice := hydrated.ToRsInvoice()

	// 		pdfBase64, err := s.compileInvoicePDF(ctx, rsInvoice)
	// 		if err != nil {
	// 			log.Printf("[PDF-WARN] Skipping attachment compilation error trace: %v", err)
	// 		}

	// 		name := rsInvoice.ContactTo.Fname + " " + rsInvoice.ContactTo.Lname
	// 		dbSubject, dbBody, _ := s.repo.GetSavedClinicMailTemplate(ctx, rsInvoice.ClinicID)
	// 		chosenSubject, chosenBody, _ := mail.GetTemplateContext(dbSubject, dbBody)
	// 		subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, rsInvoice.ID.String()[:8])

	// 		go func(to, invNum, sub, html, pdf string) {
	// 			if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
	// 				log.Printf("[MAIL-ERR] Firing automated payment confirmation receipt failed: %v", err)
	// 			}
	// 		}(rsInvoice.ContactTo.Email, rsInvoice.ID.String()[:8], subject, htmlBody, pdfBase64)
	// 	}
	// }

	return nil
}

// func (s *Service) GetClinicTemplate(ctx context.Context, clinicID uuid.UUID) (*RsInvoiceMailTemplate, error) {
// 	dbSubject, dbBody, err := s.repo.GetSavedClinicMailTemplate(ctx, clinicID)
// 	if err != nil {
// 		dbSubject, dbBody = "", ""
// 	}

// 	subject, body, isCustom := mail.GetTemplateContext(dbSubject, dbBody)

// 	return &RsInvoiceMailTemplate{
// 		Subject:  subject,
// 		Body:     body,
// 		IsCustom: isCustom,
// 	}, nil
// }

// func (s *Service) SaveClinicTemplate(ctx context.Context, clinicID uuid.UUID, rq *RqSaveMailTemplate) error {
// 	return s.repo.SaveClinicMailTemplate(ctx, clinicID, rq.Subject, rq.Body)
// }

// func (s *Service) ResendInvoiceEmail(ctx context.Context, id uuid.UUID) error {
// 	hydrated, err := s.repo.GetByID(ctx, s.db, id)
// 	if err != nil {
// 		return err
// 	}

// 	rsInvoice := hydrated.ToRsInvoice()

// 	if rsInvoice.ContactTo == nil || rsInvoice.ContactTo.Email == "" {
// 		return errors.New("cannot resend: missing contact email field")
// 	}

// 	// Generate PDF binary attachment directly from backend cross-service calls
// 	pdfBase64, err := s.compileInvoicePDF(ctx, rsInvoice)
// 	if err != nil {
// 		return fmt.Errorf("failed to generate invoice attachment document: %w", err)
// 	}

// 	dbSubject, dbBody, err := s.repo.GetSavedClinicMailTemplate(ctx, rsInvoice.ClinicID)
// 	if err != nil {
// 		dbSubject, dbBody = "", ""
// 	}

// 	chosenSubject, chosenBody, _ := mail.GetTemplateContext(dbSubject, dbBody)
// 	name := rsInvoice.ContactTo.Fname + " " + rsInvoice.ContactTo.Lname

// 	subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, rsInvoice.ID.String()[:8])

// 	go func(to, invNum, sub, html, pdf string) {
// 		if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
// 			log.Printf("[MAIL-ERR] Running async template mail worker failed processing invoice task context: %v", err)
// 		}
// 	}(rsInvoice.ContactTo.Email, rsInvoice.ID.String()[:8], subject, htmlBody, pdfBase64)

// 	return nil
// }

// Internal Backend Helper: Maps RsInvoice data models directly into template.RqGeneratePDF configurations
// func (s *Service) compileInvoicePDF(ctx context.Context, inv *RsInvoice) (string, error) {
// 	// --- BillTo from contact ---
// 	var billToName, billToEmail, billToPhone, billToABN, billToAddress string
// 	if inv.ContactTo != nil {
// 		billToName = strings.TrimSpace(inv.ContactTo.Fname + " " + inv.ContactTo.Lname)
// 		billToEmail = inv.ContactTo.Email
// 		billToPhone = inv.ContactTo.Phone
// 		billToABN = inv.ContactTo.ABN
// 		if len(inv.ContactTo.Address) > 0 {
// 			addr := inv.ContactTo.Address[0]
// 			for _, a := range inv.ContactTo.Address {
// 				if a.IsPrimary {
// 					addr = a
// 					break
// 				}
// 			}
// 			parts := []string{addr.AddressLine1}
// 			if addr.AddressLine2 != nil && *addr.AddressLine2 != "" {
// 				parts = append(parts, *addr.AddressLine2)
// 			}
// 			parts = append(parts, addr.City, addr.State, addr.PostalCode, addr.Country)
// 			billToAddress = strings.Join(parts, ", ")
// 		}
// 	}

// 	// --- BillFrom + clinic meta from clinicSvc ---
// 	var clinicName, billFromName, billFromABN, billFromEmail, billFromPhone, billFromAddress string
// 	if s.clinicSvc != nil {
// 		clinic, err := s.clinicSvc.GetProfile(ctx, inv.ClinicID)
// 		if err == nil && clinic != nil {
// 			clinicName = clinic.ClinicName
// 			billFromName = clinic.ClinicName
// 			if clinic.ABN != nil {
// 				billFromABN = *clinic.ABN
// 			}
// 			if len(clinic.Addresses) > 0 {
// 				addr := clinic.Addresses[0]
// 				for _, a := range clinic.Addresses {
// 					if a.IsPrimary {
// 						addr = a
// 						break
// 					}
// 				}
// 				parts := []string{}
// 				if addr.Address != "" {
// 					parts = append(parts, addr.Address)
// 				}
// 				if addr.City != "" {
// 					parts = append(parts, addr.City)
// 				}
// 				if addr.State != "" {
// 					parts = append(parts, addr.State)
// 				}
// 				if addr.Postcode != "" {
// 					parts = append(parts, addr.Postcode)
// 				}
// 				billFromAddress = strings.Join(parts, ", ")
// 			}
// 			for _, c := range clinic.Contacts {
// 				switch c.ContactType {
// 				case "PHONE":
// 					if billFromPhone == "" {
// 						billFromPhone = c.Value
// 					}
// 				}
// 			}
// 			// Email comes directly from the clinic record
// 			billFromEmail = clinic.Email
// 		}
// 	}

// 	// --- Line items ---
// 	pdfItems := make([]template.LineItem, 0, len(inv.Entries))
// 	var grandTotal float64

// 	for _, it := range inv.Entries {
// 		var desc string
// 		if it.Description != nil {
// 			desc = *it.Description
// 		}
// 		pdfItems = append(pdfItems, template.LineItem{
// 			Name:        it.Name,
// 			Description: desc,
// 			LineTotal:   it.Amount,
// 		})
// 		grandTotal += it.Amount
// 	}

// 	// --- Template settings: logo, letterhead, footer, terms ---
// 	var showLogo, showLogoImage bool
// 	var logoURL, logoInitial, letterheadHTML, footerHTML, notes string

// 	tplSetting, err := s.tplService.GetSetting(ctx, inv.TemplateID)
// 	if err == nil && tplSetting != nil {
// 		if tplSetting.IsLogo {
// 			showLogo = true
// 			if tplSetting.Logo != nil && tplSetting.Logo.FileKey != "" {
// 				logoURL = strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + tplSetting.Logo.FileKey
// 				showLogoImage = true
// 			} else if clinicName != "" {
// 				// Fall back to initial when no logo image is set
// 				runes := []rune(clinicName)
// 				if len(runes) > 0 {
// 					logoInitial = string(runes[0])
// 				}
// 			}
// 		}
// 		if tplSetting.LetterHead != nil && tplSetting.LetterHead.FileKey != "" {
// 			letterheadHTML = `<img src="` + strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + tplSetting.LetterHead.FileKey + `" style="width:100%;" />`
// 		}
// 		if tplSetting.Footer != nil && tplSetting.Footer.FileKey != "" {
// 			footerHTML = `<img src="` + strings.TrimRight(s.cfg.R2StoragePrefix, "/") + "/" + tplSetting.Footer.FileKey + `" style="width:100%;" />`
// 		}
// 		if tplSetting.TermText != nil {
// 			notes = *tplSetting.TermText
// 		}
// 	}

// 	pdfRq := template.RqGeneratePDF{
// 		ClinicId:   inv.ClinicID,
// 		TemplateId: inv.TemplateID,
// 		Data: template.InvoiceData{
// 			ClinicName:       clinicName,
// 			IssueDateDisplay: inv.IssueDate,
// 			DueDateDisplay:   lo.FromPtrOr(inv.DueDate, ""),
// 			BillingPeriod:    inv.BillingPeriodFrom + " to " + inv.BillingPeriodTo,
// 			InvoiceFrequency: lo.FromPtrOr(inv.InvoiceFrequency, ""),
// 			ShowLogo:         showLogo,
// 			ShowLogoImage:    showLogoImage,
// 			LogoURL:          logoURL,
// 			LogoInitial:      logoInitial,
// 			LetterheadHTML:   letterheadHTML,
// 			FooterHTML:       footerHTML,
// 			Notes:            notes,
// 			BillFrom: template.PartyInfo{
// 				Name:    billFromName,
// 				Address: billFromAddress,
// 				ABN:     billFromABN,
// 				Email:   billFromEmail,
// 				Phone:   billFromPhone,
// 			},
// 			BillTo: template.PartyInfo{
// 				Name:    billToName,
// 				Address: billToAddress,
// 				ABN:     billToABN,
// 				Email:   billToEmail,
// 				Phone:   billToPhone,
// 			},
// 			Items:      pdfItems,
// 			GrandTotal: grandTotal,

// 			// UI Component Display Variables and Table Captions
// 			TotalsAmountsCaption: "All amounts in AUD · Tax inclusive (GST included)",
// 			TotalsGrandLabel:     "Total (AUD)",
// 		},
// 	}

// 	pdfBytes, err := s.tplService.GeneratePDF(ctx, pdfRq)
// 	if err != nil {
// 		return "", err
// 	}

// 	return base64.StdEncoding.EncodeToString(pdfBytes), nil
// }

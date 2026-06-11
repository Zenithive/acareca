package invoice

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/template"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
	"github.com/samber/lo"
)

type IService interface {
	Create(ctx context.Context, invoice *RqInvoice) error
	Update(ctx context.Context, invoice *RqUpdateInvoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error)
	List(ctx context.Context, clinicID uuid.UUID, ft *Filter) (*util.RsList, error)
	GetClinicTemplate(ctx context.Context, clinicID uuid.UUID) (*RsInvoiceMailTemplate, error)
	SaveClinicTemplate(ctx context.Context, clinicID uuid.UUID, rq *RqSaveMailTemplate) error
	ResendInvoiceEmail(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo       IRepository
	cfg        *config.Config
	mailer     *mail.Client
	tplService template.IService
}

func NewService(repo IRepository, cfg *config.Config, tplService template.IService) IService {
	return &Service{
		repo:       repo,
		cfg:        cfg,
		mailer:     mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail),
		tplService: tplService,
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
	invoice, err := s.repo.Get(ctx, id)
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
	existing, err := s.repo.Get(ctx, invoice.ID)
	if err != nil {
		return err
	}

	var wasPaid bool
	if existing.Status != nil {
		wasPaid = (*existing.Status == "paid")
	}

	updatedInvoice := invoice.ApplyToInvoice(existing)
	err = s.repo.Update(ctx, updatedInvoice)
	if err != nil {
		return err
	}

	// Fetch fully loaded data row from db to get client fields securely
	hydrated, err := s.repo.Get(ctx, invoice.ID)
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
			subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, rsInvoice.InvoiceNumber)

			go func(to, invNum, sub, html, pdf string) {
				if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
					log.Printf("[MAIL-ERR] Firing automated payment confirmation receipt failed: %v", err)
				}
			}(rsInvoice.ContactTo.Email, rsInvoice.InvoiceNumber, subject, htmlBody, pdfBase64)
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
	hydrated, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	rsInvoice := hydrated.ToRsInvoice()

	if rsInvoice.ContactTo == nil || rsInvoice.ContactTo.Email == "" {
		return errors.New("cannot resend: missing contact email field")
	}

	// Generate PDF binary attachment directly from backend cross-service calls
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

	subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, rsInvoice.InvoiceNumber)

	go func(to, invNum, sub, html, pdf string) {
		if err := s.mailer.SendInvoicePaidEmail(to, invNum, pdf, sub, html); err != nil {
			log.Printf("[MAIL-ERR] Running async template mail worker failed processing invoice task context: %v", err)
		}
	}(rsInvoice.ContactTo.Email, rsInvoice.InvoiceNumber, subject, htmlBody, pdfBase64)

	return nil
}

// Internal Backend Helper: Maps RsInvoice data models directly into template.RqGeneratePDF configurations
func (s *Service) compileInvoicePDF(ctx context.Context, inv *RsInvoice) (string, error) {
	var billToName, billToEmail string
	if inv.ContactTo != nil {
		billToName = inv.ContactTo.Fname + " " + inv.ContactTo.Lname
		billToEmail = inv.ContactTo.Email
	}

	pdfItems := make([]template.LineItem, 0, len(inv.Items))
	var subtotal, taxTotal, grandTotal float64

	for _, it := range inv.Items {
		pdfItems = append(pdfItems, template.LineItem{
			Name:        it.Name,
			Description: *it.Description,
			UnitPrice:   it.UnitPrice,
			Qty:         it.Quantity,
			TaxPercent:  *it.TaxRate,
			TaxAmount:   *it.TaxAmount,
			LineTotal:   it.TotalAmount,
		})
		taxTotal += *it.TaxAmount
		grandTotal += it.TotalAmount
	}
	subtotal = grandTotal - taxTotal

	var paymentLabel, taxLabel string
	if inv.PaymentMethod != nil {
		paymentLabel = *inv.PaymentMethod
	}
	if inv.TaxMethod != nil {
		taxLabel = *inv.TaxMethod
	}

	// Safely map values into the cross-module Template DTO footprint definitions
	pdfRq := template.RqGeneratePDF{
		ClinicId:   inv.ClinicID,
		TemplateId: inv.TemplateID,
		Data: template.InvoiceData{
			InvoiceNumber:      inv.InvoiceNumber,
			IssueDateDisplay:   inv.IssueDate,
			DueDateDisplay:     lo.FromPtrOr(inv.DueDate, ""),
			Reference:          lo.FromPtrOr(inv.Reference, ""),
			PaymentMethodLabel: paymentLabel,
			TaxMethodLabel:     taxLabel,
			BillTo: template.PartyInfo{
				Name:  billToName,
				Email: billToEmail,
			},
			Items:      pdfItems,
			Subtotal:   subtotal,
			TaxTotal:   taxTotal,
			GrandTotal: grandTotal,
		},
	}

	pdfBytes, err := s.tplService.GeneratePDF(ctx, pdfRq)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pdfBytes), nil
}

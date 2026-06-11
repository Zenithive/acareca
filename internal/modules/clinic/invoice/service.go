package invoice

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/mail"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type IService interface {
	Create(ctx context.Context, invoice *RqInvoice) error
	Update(ctx context.Context, invoice *RqUpdateInvoice) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*RsInvoice, error)
	List(ctx context.Context, clinicID uuid.UUID, ft *Filter) (*util.RsList, error)
	GetClinicTemplate(ctx context.Context, clinicID uuid.UUID) (*RsInvoiceMailTemplate, error)
	SaveClinicTemplate(ctx context.Context, clinicID uuid.UUID, rq *RqSaveMailTemplate) error
	ResendInvoiceEmail(ctx context.Context, id uuid.UUID, rq *RqResendInvoice) error
}

type Service struct {
	repo   IRepository
	cfg    *config.Config
	mailer *mail.Client
}

func NewService(repo IRepository, cfg *config.Config) IService {
	return &Service{
		repo:   repo,
		cfg:    cfg,
		mailer: mail.NewClient(cfg.ResendAPIKey, cfg.SenderEmail)}
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

// Update implements [IService].
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

			name := hydrated.ContactTo.Fname + " " + hydrated.ContactTo.Lname
			dbSubject, dbBody, _ := s.repo.GetSavedClinicMailTemplate(ctx, hydrated.ClinicID)
			chosenSubject, chosenBody, _ := mail.GetTemplateContext(dbSubject, dbBody)
			subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, hydrated.InvoiceNumber)

			go func(to, sub, html, pdf string) {
				if err := s.mailer.SendInvoicePaidEmail(to, hydrated.InvoiceNumber, pdf, sub, html); err != nil {
					log.Printf("[MAIL-ERR] Firing automated payment confirmation receipt failed: %v", err)
				}
			}(hydrated.ContactTo.Email, subject, htmlBody, invoice.AttachmentBase64)
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

func (s *Service) ResendInvoiceEmail(ctx context.Context, id uuid.UUID, rq *RqResendInvoice) error {
	hydrated, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if hydrated.ContactTo == nil || hydrated.ContactTo.Email == "" {
		return errors.New("cannot resend: missing contact email field")
	}

	dbSubject, dbBody, err := s.repo.GetSavedClinicMailTemplate(ctx, hydrated.ClinicID)
	if err != nil {
		dbSubject, dbBody = "", ""
	}

	chosenSubject, chosenBody, _ := mail.GetTemplateContext(dbSubject, dbBody)
	name := hydrated.ContactTo.Fname + " " + hydrated.ContactTo.Lname

	subject, htmlBody := mail.RenderTemplateReplacements(chosenSubject, chosenBody, name, hydrated.InvoiceNumber)

	go func(to, sub, html, pdf string) {
		if err := s.mailer.SendInvoicePaidEmail(to, hydrated.InvoiceNumber, pdf, sub, html); err != nil {
			log.Printf("[MAIL-ERR] Running async template mail worker failed processing invoice task context: %v", err)
		}
	}(hydrated.ContactTo.Email, subject, htmlBody, rq.AttachmentBase64)

	return nil
}

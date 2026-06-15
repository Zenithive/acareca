package invoice

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type InvoiceSectionItem struct {
	InvoiceSection string `json:"invoice_section" validate:"required,oneof=CALCULATION_STATEMENT SFA_INVOICE REMITTANCE_INVOICE"`
	DocumentNumber string `json:"document_number" validate:"required"`
}

type InvoiceSection struct {
	ID             uuid.UUID `db:"id"`
	InvoiceID      uuid.UUID `db:"invoice_id"`
	InvoiceSection string    `db:"invoice_section"`
	DocumentNumber string    `db:"document_number"`
	CreatedAt      string    `db:"created_at"`
	UpdatedAt      string    `db:"updated_at"`
	DeleteAt       string    `db:"delete_at"`
}

type RqInvoice struct {
	ClinicID          uuid.UUID            `json:"clinic_id" validate:"-"`
	ContactID         uuid.UUID            `json:"contact_id" validate:"required"`
	TemplateID        uuid.UUID            `json:"template_id" validate:"required"`
	Name              string               `json:"name" validate:"required"`
	BillingPeriodFrom string               `json:"billing_period_from" validate:"required,datetime=2006-01-02"`
	BillingPeriodTo   string               `json:"billing_period_to" validate:"required,datetime=2006-01-02"`
	InvoiceFrequency  *string              `json:"invoice_frequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         string               `json:"issue_date" validate:"required,datetime=2006-01-02"`
	DueDate           *string              `json:"due_date,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status            *string              `json:"status"`
	InvoiceSections   []InvoiceSectionItem `json:"invoice_sections,omitempty" validate:"omitempty,dive"`
	Items             []*item.RqItem       `json:"items" validate:"required,dive"`
}

func (r *RqInvoice) ToInvoice() *Invoice {
	items := make([]*item.Item, 0, len(r.Items))
	for _, rqItem := range r.Items {
		items = append(items, rqItem.ToItem())
	}

	// Set default status to "draft" if not provided
	status := r.Status
	if status == nil {
		defaultStatus := "draft"
		status = &defaultStatus
	}

	// If no sections provided, create a default CALCULATION_STATEMENT section
	sections := r.InvoiceSections
	if len(sections) == 0 {
		sections = []InvoiceSectionItem{
			{
				InvoiceSection: "CALCULATION_STATEMENT",
				DocumentNumber: uuid.New().String()[:8], // Generate a short doc number
			},
		}
	}

	return &Invoice{
		ClinicID:          r.ClinicID,
		ContactID:         &r.ContactID,
		TemplateID:        r.TemplateID,
		Name:              r.Name,
		BillingPeriodFrom: r.BillingPeriodFrom,
		BillingPeriodTo:   r.BillingPeriodTo,
		InvoiceFrequency:  r.InvoiceFrequency,
		IssueDate:         r.IssueDate,
		Status:            status,
		DueDate:           r.DueDate,
		InvoiceSections:   sections,
		Items:             items,
	}
}

type RqUpdateInvoice struct {
	ID                *uuid.UUID           `json:"id" validate:"-"`
	ContactID         *uuid.UUID           `json:"contact_id,omitempty"`
	TemplateID        *uuid.UUID           `json:"template_id,omitempty"`
	Name              *string              `json:"name,omitempty"`
	BillingPeriodFrom *string              `json:"billing_period_from,omitempty" validate:"omitempty,datetime=2006-01-02"`
	BillingPeriodTo   *string              `json:"billing_period_to,omitempty" validate:"omitempty,datetime=2006-01-02"`
	InvoiceFrequency  *string              `json:"invoice_frequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         *string              `json:"issue_date,omitempty" validate:"omitempty,datetime=2006-01-02"`
	DueDate           *string              `json:"due_date,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status            *string              `json:"status,omitempty"`
	InvoiceSections   []InvoiceSectionItem `json:"invoice_sections,omitempty" validate:"omitempty,dive"`
	Items             []*item.RqUpdateItem `json:"items,omitempty" validate:"omitempty,dive"`
	AttachmentBase64  string               `json:"attachment_base64,omitempty"`
}

func (r *RqUpdateInvoice) ApplyToInvoice(inv *Invoice) *Invoice {
	if r.ID != nil {
		inv.ID = *r.ID
	}
	if r.ContactID != nil {
		inv.ContactID = r.ContactID
	}
	if r.TemplateID != nil {
		inv.TemplateID = *r.TemplateID
	}
	if r.Name != nil {
		inv.Name = *r.Name
	}
	if r.BillingPeriodFrom != nil {
		inv.BillingPeriodFrom = *r.BillingPeriodFrom
	}
	if r.BillingPeriodTo != nil {
		inv.BillingPeriodTo = *r.BillingPeriodTo
	}
	if r.InvoiceFrequency != nil {
		inv.InvoiceFrequency = r.InvoiceFrequency
	}
	if r.IssueDate != nil {
		inv.IssueDate = *r.IssueDate
	}
	if r.DueDate != nil {
		inv.DueDate = r.DueDate
	}
	if r.Items != nil {
		items := make([]*item.Item, 0, len(r.Items))
		for _, rqItem := range r.Items {
			invoiceItem := &item.Item{ID: rqItem.ID}
			items = append(items, rqItem.ApplyToItem(invoiceItem))
		}
		inv.Items = items
	}

	if r.Status != nil {
		inv.Status = r.Status
	}

	if r.InvoiceSections != nil {
		inv.InvoiceSections = r.InvoiceSections
	} else if len(inv.InvoiceSections) == 0 {
		// Ensure at least one section exists
		inv.InvoiceSections = []InvoiceSectionItem{
			{
				InvoiceSection: "CALCULATION_STATEMENT",
				DocumentNumber: inv.ID.String()[:8],
			},
		}
	}

	return inv
}

type Invoice struct {
	ID                uuid.UUID            `db:"id"`
	ClinicID          uuid.UUID            `db:"clinic_id"`
	ContactID         *uuid.UUID           `db:"contact_id"`
	TemplateID        uuid.UUID            `db:"template_id"`
	Name              string               `db:"name"`
	BillingPeriodFrom string               `db:"billing_period_from"`
	BillingPeriodTo   string               `db:"billing_period_to"`
	InvoiceFrequency  *string              `db:"invoice_frequency,omitempty"`
	IssueDate         string               `db:"issue_date"`
	DueDate           *string              `db:"due_date,omitempty"`
	Status            *string              `db:"status"`
	ContactTo         *contact.Contact     `db:"-"`
	InvoiceSections   []InvoiceSectionItem `db:"-"`
	Items             []*item.Item         `db:"-"`
	CreatedAt         string               `db:"created_at"`
	UpdatedAt         string               `db:"updated_at"`
}

func (i *Invoice) ToRsInvoice() *RsInvoice {
	items := make([]*item.RsItem, 0, len(i.Items))
	for _, item := range i.Items {
		items = append(items, item.ToRsItem(i.ID))
	}

	var contactTo *contact.RsContact
	if i.ContactTo != nil {
		rsContact := i.ContactTo.ToRsContact()
		contactTo = &rsContact
	}

	return &RsInvoice{
		ID:                i.ID,
		ClinicID:          i.ClinicID,
		ContactID:         i.ContactID,
		ContactTo:         contactTo,
		SentTo:            contactTo,
		TemplateID:        i.TemplateID,
		Name:              i.Name,
		BillingPeriodFrom: i.BillingPeriodFrom,
		BillingPeriodTo:   i.BillingPeriodTo,
		InvoiceFrequency:  i.InvoiceFrequency,
		IssueDate:         i.IssueDate,
		DueDate:           i.DueDate,
		Status:            i.Status,
		InvoiceSections:   i.InvoiceSections,
		Items:             items,
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
	}
}

type RsInvoice struct {
	ID                uuid.UUID            `json:"id"`
	ClinicID          uuid.UUID            `json:"clinic_id"`
	ContactID         *uuid.UUID           `json:"contact_id,omitempty"`
	ContactTo         *contact.RsContact   `json:"contact_to,omitempty"`
	SentTo            *contact.RsContact   `json:"sent_to,omitempty"`
	TemplateID        uuid.UUID            `json:"template_id"`
	Name              string               `json:"name"`
	BillingPeriodFrom string               `json:"billing_period_from"`
	BillingPeriodTo   string               `json:"billing_period_to"`
	InvoiceFrequency  *string              `json:"invoice_frequency,omitempty"`
	IssueDate         string               `json:"issue_date"`
	DueDate           *string              `json:"due_date,omitempty"`
	Status            *string              `json:"status"`
	InvoiceSections   []InvoiceSectionItem `json:"invoice_sections,omitempty"`
	Items             []*item.RsItem       `json:"items,omitempty"`
	CreatedAt         string               `json:"created_at"`
	UpdatedAt         string               `json:"updated_at"`
}

type Filter struct {
	Name      *string `form:"name,omitempty"`
	Status    *string `form:"status,omitempty"`
	ContactID *string `form:"contact_id,omitempty"`
	IssueDate *string `form:"date_range,omitempty"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	if filter.Name != nil && *filter.Name != "" {
		filters["name"] = "%" + *filter.Name + "%"
		operators["name"] = common.OpLike
	}
	if filter.Status != nil && *filter.Status != "" {
		filters["status"] = *filter.Status
		operators["status"] = common.OpEq
	}
	if filter.ContactID != nil && *filter.ContactID != "" {
		if parsedUUID, err := uuid.Parse(*filter.ContactID); err == nil && parsedUUID != uuid.Nil {
			filters["contact_id"] = parsedUUID
			operators["contact_id"] = common.OpEq
		}
	}

	if filter.IssueDate != nil && *filter.IssueDate != "" {
		now := time.Now().UTC()

		switch *filter.IssueDate {
		case "last_7":
			sevenDaysAgo := now.AddDate(0, 0, -7)
			startTime := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, time.UTC)

			filters["date_range_start"] = startTime.Format("2006-01-02")
			operators["date_range_start"] = common.OpGte

		case "last_30":
			thirtyDaysAgo := now.AddDate(0, 0, -30)
			startTime := time.Date(thirtyDaysAgo.Year(), thirtyDaysAgo.Month(), thirtyDaysAgo.Day(), 0, 0, 0, 0, time.UTC)

			filters["date_range_start"] = startTime.Format("2006-01-02")
			operators["date_range_start"] = common.OpGte

		case "this_month":
			firstDayOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

			filters["date_range_start"] = firstDayOfThisMonth.Format("2006-01-02")
			operators["date_range_start"] = common.OpGte

		case "last_month":
			firstDayOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			firstDayOfLastMonth := firstDayOfThisMonth.AddDate(0, -1, 0)
			lastDayOfLastMonth := firstDayOfThisMonth.AddDate(0, 0, -1)

			filters["date_range_start"] = firstDayOfLastMonth.Format("2006-01-02")
			operators["date_range_start"] = common.OpGte

			filters["date_range_end"] = lastDayOfLastMonth.Format("2006-01-02")
			operators["date_range_end"] = common.OpLte
		}
	}

	return common.NewFilter(filter.Search, filters, operators, filter.Limit, filter.Offset, filter.SortBy, filter.OrderBy)
}

type RqSaveMailTemplate struct {
	Subject string `json:"mail_subject" validate:"required"`
	Body    string `json:"mail_body" validate:"required"`
}

type RsInvoiceMailTemplate struct {
	Subject  string `json:"mail_subject"`
	Body     string `json:"mail_body"`
	IsCustom bool   `json:"is_custom"`
}

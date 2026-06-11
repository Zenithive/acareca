package invoice

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type RqInvoice struct {
	ClinicID      uuid.UUID      `json:"clinic_id" validate:"-"`
	ContactID     uuid.UUID      `json:"contact_id" validate:"required"`
	TemplateID    uuid.UUID      `json:"template_id" validate:"required"`
	Name          string         `json:"name" validate:"required"`
	InvoiceNumber string         `json:"invoice_number" validate:"required"`
	Reference     *string        `json:"reference,omitempty"`
	PaymentMethod *string        `json:"payment_method" validate:"omitempty,oneof=CASH CARD ONLINE"`
	TaxMethod     *string        `json:"tax_method" validate:"omitempty,oneof=EXCLUSIVE INCLUSIVE"`
	IssueDate     string         `json:"issue_date" validate:"required,datetime=2006-01-02"`
	DueDate       *string        `json:"due_date,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status        *string        `json:"status"`
	Items         []*item.RqItem `json:"items" validate:"required,dive"`
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

	return &Invoice{
		ClinicID:      r.ClinicID,
		ContactID:     &r.ContactID,
		TemplateID:    r.TemplateID,
		Name:          r.Name,
		InvoiceNumber: r.InvoiceNumber,
		Reference:     r.Reference,
		PaymentMethod: r.PaymentMethod,
		TaxMethod:     r.TaxMethod,
		IssueDate:     r.IssueDate,
		Status:        status,
		DueDate:       r.DueDate,
		Items:         items,
	}
}

type RqUpdateInvoice struct {
	ID               uuid.UUID            `json:"id" validate:"-"`
	ContactID        *uuid.UUID           `json:"contact_id,omitempty"`
	TemplateID       *uuid.UUID           `json:"template_id,omitempty"`
	Name             *string              `json:"name,omitempty"`
	InvoiceNumber    *string              `json:"invoice_number,omitempty"`
	Reference        *string              `json:"reference,omitempty"`
	PaymentMethod    *string              `json:"payment_method,omitempty" validate:"omitempty,oneof=CASH CARD ONLINE"`
	TaxMethod        *string              `json:"tax_method,omitempty" validate:"omitempty,oneof=EXCLUSIVE INCLUSIVE"`
	IssueDate        *string              `json:"issue_date,omitempty" validate:"omitempty,datetime=2006-01-02"`
	DueDate          *string              `json:"due_date,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status           *string              `json:"status,omitempty"`
	Items            []*item.RqUpdateItem `json:"items,omitempty" validate:"omitempty,dive"`
	AttachmentBase64 string               `json:"attachment_base64,omitempty"`
}

func (r *RqUpdateInvoice) ApplyToInvoice(inv *Invoice) *Invoice {
	inv.ID = r.ID
	if r.ContactID != nil {
		inv.ContactID = r.ContactID
	}
	if r.TemplateID != nil {
		inv.TemplateID = *r.TemplateID
	}
	if r.Name != nil {
		inv.Name = *r.Name
	}
	if r.InvoiceNumber != nil {
		inv.InvoiceNumber = *r.InvoiceNumber
	}
	if r.Reference != nil {
		inv.Reference = r.Reference
	}
	if r.PaymentMethod != nil {
		inv.PaymentMethod = r.PaymentMethod
	}
	if r.TaxMethod != nil {
		inv.TaxMethod = r.TaxMethod
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

	return inv
}

type Invoice struct {
	ID            uuid.UUID        `db:"id"`
	ClinicID      uuid.UUID        `db:"clinic_id"`
	ContactID     *uuid.UUID       `db:"contact_id"`
	TemplateID    uuid.UUID        `db:"template_id"`
	Name          string           `db:"name"`
	InvoiceNumber string           `db:"invoice_number"`
	Reference     *string          `db:"reference,omitempty"`
	PaymentMethod *string          `db:"payment_method,omitempty"`
	TaxMethod     *string          `db:"tax_method,omitempty"`
	IssueDate     string           `db:"issue_date"`
	DueDate       *string          `db:"due_date,omitempty"`
	Status        *string          `db:"status"`
	ContactTo     *contact.Contact `db:"-"`
	Items         []*item.Item     `db:"-"`
	CreatedAt     string           `db:"created_at"`
	UpdatedAt     string           `db:"updated_at"`
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
		ID:            i.ID,
		ClinicID:      i.ClinicID,
		ContactID:     i.ContactID,
		ContactTo:     contactTo,
		SentTo:        contactTo,
		TemplateID:    i.TemplateID,
		Name:          i.Name,
		InvoiceNumber: i.InvoiceNumber,
		Reference:     i.Reference,
		PaymentMethod: i.PaymentMethod,
		TaxMethod:     i.TaxMethod,
		IssueDate:     i.IssueDate,
		DueDate:       i.DueDate,
		Status:        i.Status,
		Items:         items,
		CreatedAt:     i.CreatedAt,
		UpdatedAt:     i.UpdatedAt,
	}
}

type RsInvoice struct {
	ID            uuid.UUID          `json:"id"`
	ClinicID      uuid.UUID          `json:"clinic_id"`
	ContactID     *uuid.UUID         `json:"contact_id,omitempty"`
	ContactTo     *contact.RsContact `json:"contact_to,omitempty"`
	SentTo        *contact.RsContact `json:"sent_to,omitempty"`
	TemplateID    uuid.UUID          `json:"template_id"`
	Name          string             `json:"name"`
	InvoiceNumber string             `json:"invoice_number"`
	Reference     *string            `json:"reference,omitempty"`
	PaymentMethod *string            `json:"payment_method,omitempty"`
	TaxMethod     *string            `json:"tax_method,omitempty"`
	IssueDate     string             `json:"issue_date"`
	DueDate       *string            `json:"due_date,omitempty"`
	Status        *string            `json:"status"`
	Items         []*item.RsItem     `json:"items,omitempty"`
	CreatedAt     string             `json:"created_at"`
	UpdatedAt     string             `json:"updated_at"`
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

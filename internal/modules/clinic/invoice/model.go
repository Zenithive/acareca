package invoice

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/section"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

type RqInvoice struct {
	ClinicID          uuid.UUID           `json:"clinicId" validate:"-"`
	ContactID         uuid.UUID           `json:"contactId" validate:"required"`
	TemplateID        uuid.UUID           `json:"templateId" validate:"required"`
	Name              string              `json:"name" validate:"required"`
	BillingPeriodFrom string              `json:"billingPeriodFrom" validate:"required,datetime=2006-01-02"`
	BillingPeriodTo   string              `json:"billingPeriodTo" validate:"required,datetime=2006-01-02"`
	InvoiceFrequency  *string             `json:"invoiceFrequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         string              `json:"issueDate" validate:"required,datetime=2006-01-02"`
	DueDate           *string             `json:"dueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status            *string             `json:"status"`
	Sections          []section.RqSection `json:"sections,omitempty" validate:"omitempty,dive"`
}

func (r *RqInvoice) ToInvoice() *Invoice {
	status := r.Status
	if status == nil {
		defaultStatus := "draft"
		status = &defaultStatus
	}

	sections := make([]section.Section, 0, len(r.Sections))
	for _, v := range r.Sections {
		sections = append(sections, *v.ToSection())
	}

	return &Invoice{
		ClinicID:          r.ClinicID,
		ContactID:         &r.ContactID,
		TemplateID:        r.TemplateID,
		Name:              r.Name,
		BillingPeriodFrom: &r.BillingPeriodFrom,
		BillingPeriodTo:   &r.BillingPeriodTo,
		InvoiceFrequency:  r.InvoiceFrequency,
		IssueDate:         r.IssueDate,
		Status:            status,
		DueDate:           r.DueDate,
		Sections:          sections,
	}
}

type RqUpdateInvoice struct {
	ID                *uuid.UUID        `json:"id" validate:"-"`
	ContactID         *uuid.UUID        `json:"contactId,omitempty"`
	TemplateID        *uuid.UUID        `json:"templateId,omitempty"`
	Name              *string           `json:"name,omitempty"`
	BillingPeriodFrom *string           `json:"billingPeriodFrom,omitempty" validate:"omitempty,datetime=2006-01-02"`
	BillingPeriodTo   *string           `json:"billingPeriodTo,omitempty" validate:"omitempty,datetime=2006-01-02"`
	InvoiceFrequency  *string           `json:"invoiceFrequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         *string           `json:"issueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	DueDate           *string           `json:"dueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status            *string           `json:"status,omitempty"`
	Sections          []section.Section `json:"sections,omitempty" validate:"omitempty,dive"`
	AttachmentBase64  string            `json:"attachmentBase64,omitempty"`
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
		inv.BillingPeriodFrom = r.BillingPeriodFrom
	}
	if r.BillingPeriodTo != nil {
		inv.BillingPeriodTo = r.BillingPeriodTo
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

	if r.Status != nil {
		inv.Status = r.Status
	}

	if r.Sections != nil {
		inv.Sections = r.Sections
	}

	return inv
}

type Invoice struct {
	ID                uuid.UUID         `db:"id"`
	ClinicID          uuid.UUID         `db:"clinic_id"`
	ContactID         *uuid.UUID        `db:"contact_id"`
	TemplateID        uuid.UUID         `db:"template_id"`
	Name              string            `db:"name"`
	BillingPeriodFrom *string           `db:"billing_period_from"`
	BillingPeriodTo   *string           `db:"billing_period_to"`
	InvoiceFrequency  *string           `db:"invoice_frequency,omitempty"`
	IssueDate         string            `db:"issue_date"`
	DueDate           *string           `db:"due_date,omitempty"`
	Status            *string           `db:"status"`
	ContactTo         *contact.Contact  `db:"-"`
	Sections          []section.Section `db:"-"`
	CreatedAt         string            `db:"created_at"`
	UpdatedAt         string            `db:"updated_at"`
}

func (i *Invoice) ToRsInvoice() *RsInvoice {
	var contactTo *contact.RsContact
	if i.ContactTo != nil {
		rsContact := i.ContactTo.ToRsContact()
		contactTo = &rsContact
	}

	billingPeriodFrom := ""
	if i.BillingPeriodFrom != nil {
		billingPeriodFrom = *i.BillingPeriodFrom
	}
	billingPeriodTo := ""
	if i.BillingPeriodTo != nil {
		billingPeriodTo = *i.BillingPeriodTo
	}

	return &RsInvoice{
		ID:                i.ID,
		ClinicID:          i.ClinicID,
		ContactID:         i.ContactID,
		ContactTo:         contactTo,
		TemplateID:        i.TemplateID,
		Name:              i.Name,
		BillingPeriodFrom: billingPeriodFrom,
		BillingPeriodTo:   billingPeriodTo,
		InvoiceFrequency:  i.InvoiceFrequency,
		IssueDate:         i.IssueDate,
		DueDate:           i.DueDate,
		Status:            i.Status,
		Sections:          i.Sections,
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
	}
}

type RsInvoice struct {
	ID                uuid.UUID          `json:"id"`
	ClinicID          uuid.UUID          `json:"clinicId"`
	ContactID         *uuid.UUID         `json:"contactId,omitempty"`
	ContactTo         *contact.RsContact `json:"contactTo,omitempty"`
	TemplateID        uuid.UUID          `json:"templateId"`
	Name              string             `json:"name"`
	BillingPeriodFrom string             `json:"billingPeriodFrom"`
	BillingPeriodTo   string             `json:"billingPeriodTo"`
	InvoiceFrequency  *string            `json:"invoiceFrequency,omitempty"`
	IssueDate         string             `json:"issueDate"`
	DueDate           *string            `json:"dueDate,omitempty"`
	Status            *string            `json:"status"`
	Sections          []section.Section  `json:"sections,omitempty"`
	CreatedAt         string             `json:"createdAt"`
	UpdatedAt         string             `json:"updatedAt"`
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
			endTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

			filters["date_range_start"] = startTime.Format("2006-01-02")
			operators["date_range_start"] = common.OpGte

			filters["date_range_end"] = endTime.Format("2006-01-02")
			operators["date_range_end"] = common.OpLte

		case "last_30":
			thirtyDaysAgo := now.AddDate(0, 0, -30)
			startTime := time.Date(thirtyDaysAgo.Year(), thirtyDaysAgo.Month(), thirtyDaysAgo.Day(), 0, 0, 0, 0, time.UTC)
			endTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

			filters["date_range_start"] = startTime.Format("2006-01-02")
			operators["date_range_start"] = common.OpGte

			filters["date_range_end"] = endTime.Format("2006-01-02")
			operators["date_range_end"] = common.OpLte

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

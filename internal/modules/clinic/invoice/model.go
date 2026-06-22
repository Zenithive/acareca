package invoice

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/section"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/samber/lo"
)

type RqInvoice struct {
	ClinicID          uuid.UUID           `json:"clinicId" validate:"-"`
	ContactID         uuid.UUID           `json:"contactId" validate:"required"`
	TemplateID        uuid.UUID           `json:"templateId" validate:"required"`
	Name              string              `json:"name" validate:"required"`
	BillingPeriodFrom string              `json:"billingPeriodFrom" validate:"required"`
	BillingPeriodTo   string              `json:"billingPeriodTo" validate:"required"`
	InvoiceFrequency  *string             `json:"invoiceFrequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         string              `json:"issueDate" validate:"required"`
	DueDate           *string             `json:"dueDate,omitempty" validate:"omitempty"`
	Status            *string             `json:"status"`
	Sections          []section.RqSection `json:"sections,omitempty" validate:"omitempty,dive"`
	Settings          *RqInvoiceSetting   `json:"settings,omitempty"`
}

type RqInvoiceSetting struct {
	PrimaryColor     *string `json:"primaryColor,omitempty"`
	AccentColor      *string `json:"accentColor,omitempty"`
	BodyFontFamily   *string `json:"bodyFontFamily,omitempty"`
	HeaderFontFamily *string `json:"headerFontFamily,omitempty"`
	IsLogo           *bool   `json:"isLogo,omitempty"`
	LogoID           *string `json:"logoId,omitempty"`
	LetterheadID     *string `json:"letterheadId,omitempty"`
	FooterID         *string `json:"footerId,omitempty"`
	TermsText        *string `json:"termsText,omitempty"`
	IsWatermark      *bool   `json:"isWatermark,omitempty"`
	WatermarkText    *string `json:"watermarkText,omitempty"`
	IsTax            *bool   `json:"isTax,omitempty"`
	TableStyle       *string `json:"tableStyle,omitempty"`
}

func (r *RqInvoice) ToInvoice() *Invoice {
	status := r.Status
	if status == nil {
		defaultStatus := "draft"
		status = &defaultStatus
	}

	invoiceID := uuid.New()

	issueDate, _ := time.Parse("2006-01-02", r.IssueDate)

	var dueDate *time.Time
	if r.DueDate != nil {
		parsed, err := time.Parse("2006-01-02", *r.DueDate)
		if err == nil {
			dueDate = &parsed
		}
	}

	sections := make([]section.Section, 0, len(r.Sections))
	for _, v := range r.Sections {
		sec := v.ToSection()
		if sec.InvoiceID == nil {
			sec.InvoiceID = &invoiceID
		}
		sections = append(sections, *sec)
	}

	return &Invoice{
		ID:                invoiceID,
		ClinicID:          r.ClinicID,
		ContactID:         &r.ContactID,
		TemplateID:        r.TemplateID,
		Name:              r.Name,
		BillingPeriodFrom: &r.BillingPeriodFrom,
		BillingPeriodTo:   &r.BillingPeriodTo,
		InvoiceFrequency:  r.InvoiceFrequency,
		IssueDate:         issueDate,
		Status:            status,
		DueDate:           dueDate,
		Sections:          sections,
		Settings:          r.Settings,
	}
}

type RqUpdateInvoice struct {
	ID                *uuid.UUID                `json:"id" validate:"-"`
	ClinicID          uuid.UUID                 `json:"clinicId"`
	ContactID         *uuid.UUID                `json:"contactId,omitempty"`
	TemplateID        *uuid.UUID                `json:"templateId,omitempty"`
	Name              *string                   `json:"name,omitempty"`
	BillingPeriodFrom *string                   `json:"billingPeriodFrom,omitempty" validate:"omitempty,datetime=2006-01-02"`
	BillingPeriodTo   *string                   `json:"billingPeriodTo,omitempty" validate:"omitempty,datetime=2006-01-02"`
	InvoiceFrequency  *string                   `json:"invoiceFrequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         *string                   `json:"issueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	DueDate           *string                   `json:"dueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	Status            *string                   `json:"status,omitempty"`
	Sections          []section.RqUpdateSection `json:"sections,omitempty" validate:"omitempty,dive"`
	DeleteSections    []uuid.UUID               `json:"deleteSections,omitempty"`
	AttachmentBase64  string                    `json:"attachmentBase64,omitempty"`
	Settings          *RqInvoiceSetting         `json:"settings,omitempty"`
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
		if t, err := time.Parse("2006-01-02", *r.IssueDate); err == nil {
			inv.IssueDate = t
		}
	}
	if r.DueDate != nil {
		if t, err := time.Parse("2006-01-02", *r.DueDate); err == nil {
			inv.DueDate = &t
		}
	}
	if r.Status != nil {
		inv.Status = r.Status
	}

	if r.Sections != nil {
		sections := make([]section.Section, 0, len(r.Sections))
		for _, rqSec := range r.Sections {
			sections = append(sections, *rqSec.ToSection())
		}
		inv.Sections = sections
	}

	if r.Settings != nil {
		inv.Settings = r.Settings
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
	IssueDate         time.Time         `db:"issue_date"`
	DueDate           *time.Time        `db:"due_date,omitempty"`
	Status            *string           `db:"status"`
	ContactTo         *contact.Contact  `db:"-"`
	Sections          []section.Section `db:"-"`
	Settings          *RqInvoiceSetting `db:"-"`
	CreatedAt         time.Time         `db:"created_at"`
	UpdatedAt         *time.Time        `db:"updated_at"`
}

func (i *Invoice) ToRsInvoiceSummary() *RsInvoiceSummary {
	var contactTo *contact.RsContact
	if i.ContactTo != nil {
		rsContact := i.ContactTo.ToRsContact()
		contactTo = &rsContact
	}

	return &RsInvoiceSummary{
		ID:                i.ID,
		ClinicID:          i.ClinicID,
		ContactID:         i.ContactID,
		ContactTo:         contactTo,
		TemplateID:        i.TemplateID,
		Name:              i.Name,
		BillingPeriodFrom: lo.FromPtrOr(i.BillingPeriodFrom, ""),
		BillingPeriodTo:   lo.FromPtrOr(i.BillingPeriodTo, ""),
		InvoiceFrequency:  i.InvoiceFrequency,
		IssueDate:         i.IssueDate,
		DueDate:           i.DueDate,
		Status:            i.Status,
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
	}
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

	rsSection := make([]section.RsSection, 0, len(i.Sections))
	for _, v := range i.Sections {
		rsSection = append(rsSection, *v.ToRsSection())
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
		Sections:          rsSection,
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
	}
}

type RsInvoiceSummary struct {
	ID                uuid.UUID          `json:"id"`
	ClinicID          uuid.UUID          `json:"clinicId"`
	ContactID         *uuid.UUID         `json:"contactId,omitempty"`
	ContactTo         *contact.RsContact `json:"contactTo,omitempty"`
	TemplateID        uuid.UUID          `json:"templateId"`
	Name              string             `json:"name"`
	BillingPeriodFrom string             `json:"billingPeriodFrom"`
	BillingPeriodTo   string             `json:"billingPeriodTo"`
	InvoiceFrequency  *string            `json:"invoiceFrequency,omitempty"`
	IssueDate         time.Time          `json:"issueDate"`
	DueDate           *time.Time         `json:"dueDate,omitempty"`
	Status            *string            `json:"status"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         *time.Time         `json:"updatedAt"`
}

type RsInvoice struct {
	ID                uuid.UUID           `json:"id"`
	ClinicID          uuid.UUID           `json:"clinicId"`
	ContactID         *uuid.UUID          `json:"contactId,omitempty"`
	ContactTo         *contact.RsContact  `json:"contactTo,omitempty"`
	TemplateID        uuid.UUID           `json:"templateId"`
	Name              string              `json:"name"`
	BillingPeriodFrom string              `json:"billingPeriodFrom"`
	BillingPeriodTo   string              `json:"billingPeriodTo"`
	InvoiceFrequency  *string             `json:"invoiceFrequency,omitempty"`
	IssueDate         time.Time           `json:"issueDate"`
	DueDate           *time.Time          `json:"dueDate,omitempty"`
	Status            *string             `json:"status"`
	Sections          []section.RsSection `json:"sections,omitempty"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         *time.Time          `json:"updatedAt"`
}

type Filter struct {
	Name      *string    `form:"name,omitempty"`
	Status    *string    `form:"status,omitempty"`
	ContactID *string    `form:"contact_id,omitempty"`
	IssueDate *string    `form:"date_range,omitempty"`
	ClinicId  *uuid.UUID `form:"-"`
	common.Filter
}

func (filter *Filter) MapToFilter() common.Filter {
	filters := map[string]interface{}{}
	operators := map[string]common.Operator{}

	if filter.Name != nil {
		filters["name"] = "%" + *filter.Name + "%"
		operators["name"] = common.OpLike
	}
	if filter.Status != nil {
		filters["status"] = *filter.Status
		operators["status"] = common.OpEq
	}
	if filter.ContactID != nil {
		if parsedUUID, err := uuid.Parse(*filter.ContactID); err == nil && parsedUUID != uuid.Nil {
			filters["contact_id"] = parsedUUID
			operators["contact_id"] = common.OpEq
		}
	}

	if filter.ClinicId != nil {
		filters["clinic_id"] = *filter.ClinicId
		operators["clinic_id"] = common.OpEq
	}

	if filter.IssueDate != nil {
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

package invoice

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/contact"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/section"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/samber/lo"
)

type RqInvoice struct {
	ClinicID          uuid.UUID           `json:"clinicId" validate:"-"`
	ContactID         uuid.UUID           `json:"contactId" validate:"required"`
	Name              string              `json:"name" validate:"required"`
	BillingPeriodFrom string              `json:"billingPeriodFrom" validate:"required"`
	BillingPeriodTo   string              `json:"billingPeriodTo" validate:"required"`
	InvoiceFrequency  *string             `json:"invoiceFrequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         string              `json:"issueDate" validate:"required"`
	DueDate           *string             `json:"dueDate,omitempty" validate:"omitempty"`
	InvoiceMethod     *util.InvoiceType   `json:"invoiceMethod,omitempty" validate:"omitempty,oneof=SFA_CLINIC_COLLECTS SFA_DENTIST_COLLECTS INDEPENDENT_CONTRACTOR"`
	Status            *string             `json:"status"`
	Sections          []section.RqSection `json:"sections,omitempty" validate:"omitempty,dive"`
	Settings          *RqInvoiceSetting   `json:"settings,omitempty"`
}

type RqInvoiceSetting struct {
	PrimaryColor     *string    `json:"primaryColor,omitempty"`
	AccentColor      *string    `json:"accentColor,omitempty"`
	BodyFontFamily   *string    `json:"bodyFontFamily,omitempty"`
	HeaderFontFamily *string    `json:"headerFontFamily,omitempty"`
	IsLogo           *bool      `json:"isLogo,omitempty"`
	LogoID           *uuid.UUID `json:"logoId,omitempty"`
	LetterheadID     *uuid.UUID `json:"letterheadId,omitempty"`
	FooterID         *uuid.UUID `json:"footerId,omitempty"`
	TermsText        *string    `json:"termsText,omitempty"`
	PaymentTerms     *string    `json:"paymentTerms,omitempty"`
	IsWatermark      *bool      `json:"isWatermark,omitempty"`
	WatermarkText    *string    `json:"watermarkText,omitempty"`
	IsTax            *bool      `json:"isTax,omitempty"`
	TableStyle       *string    `json:"tableStyle,omitempty"`
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
		Name:              r.Name,
		BillingPeriodFrom: &r.BillingPeriodFrom,
		BillingPeriodTo:   &r.BillingPeriodTo,
		InvoiceFrequency:  r.InvoiceFrequency,
		IssueDate:         issueDate,
		Status:            status,
		DueDate:           dueDate,
		Sections:          sections,
		InvoiceMethod:     r.InvoiceMethod,
		Settings:          r.Settings,
	}
}

type RqUpdateInvoice struct {
	ID                *uuid.UUID                `json:"id" validate:"-"`
	ClinicID          uuid.UUID                 `json:"clinicId"`
	ContactID         *uuid.UUID                `json:"contactId,omitempty"`
	Name              *string                   `json:"name,omitempty"`
	BillingPeriodFrom *string                   `json:"billingPeriodFrom,omitempty" validate:"omitempty,datetime=2006-01-02"`
	BillingPeriodTo   *string                   `json:"billingPeriodTo,omitempty" validate:"omitempty,datetime=2006-01-02"`
	InvoiceFrequency  *string                   `json:"invoiceFrequency,omitempty" validate:"omitempty,oneof=DAILY WEEKLY MONTHLY YEARLY"`
	IssueDate         *string                   `json:"issueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	DueDate           *string                   `json:"dueDate,omitempty" validate:"omitempty,datetime=2006-01-02"`
	InvoiceMethod     *util.InvoiceType         `json:"invoiceMethod,omitempty" validate:"omitempty,oneof=SFA_CLINIC_COLLECTS SFA_DENTIST_COLLECTS INDEPENDENT_CONTRACTOR"`
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
		existingSectionMap := make(map[uuid.UUID]*section.Section)
		for i := range inv.Sections {
			existingSectionMap[inv.Sections[i].ID] = &inv.Sections[i]
		}

		requestSectionIDs := make(map[uuid.UUID]bool)
		sections := make([]section.Section, 0, len(r.Sections))
		for _, rqSec := range r.Sections {
			sec := rqSec.ToSection()

			if existingSec, ok := existingSectionMap[sec.ID]; ok {
				if sec.InvoiceSection == "" {
					sec.InvoiceSection = existingSec.InvoiceSection
				}
				if sec.InvoiceID == nil {
					sec.InvoiceID = existingSec.InvoiceID
				}
				if len(rqSec.Entries) == 0 {
					sec.Entries = existingSec.Entries
				} else {
					existingEntryMap := make(map[uuid.UUID]*item.Item)
					for _, e := range existingSec.Entries {
						existingEntryMap[e.ID] = e
					}

					for idx, entry := range sec.Entries {
						if existingEntry, ok := existingEntryMap[entry.ID]; ok && entry.ID != uuid.Nil {
							rqEntry := rqSec.Entries[idx]
							if entry.Name == "" {
								entry.Name = existingEntry.Name
							}
							if entry.Description == nil && existingEntry.Description != nil {
								entry.Description = existingEntry.Description
							}
							if entry.EntryType == nil && existingEntry.EntryType != nil {
								entry.EntryType = existingEntry.EntryType
							}
							if entry.BASCode == nil && existingEntry.BASCode != nil {
								entry.BASCode = existingEntry.BASCode
							}
							if entry.FieldKey == nil && existingEntry.FieldKey != nil {
								entry.FieldKey = existingEntry.FieldKey
							}
							if entry.Amount == 0 && existingEntry.Amount != 0 {
								entry.Amount = existingEntry.Amount
							}
							if entry.SortOrder == 0 && existingEntry.SortOrder != 0 {
								entry.SortOrder = existingEntry.SortOrder
							}
							if entry.Expression == nil && existingEntry.Expression != nil {
								entry.Expression = existingEntry.Expression
							}
							if rqEntry.IsFinal == nil && existingEntry.IsFinal {
								entry.IsFinal = existingEntry.IsFinal
							}
							if entry.InvoiceSectionID == nil && existingEntry.InvoiceSectionID != nil {
								entry.InvoiceSectionID = existingEntry.InvoiceSectionID
							}
						}
					}
				}
			}

			requestSectionIDs[sec.ID] = true
			sections = append(sections, *sec)
		}

		for _, existingSec := range inv.Sections {
			if !requestSectionIDs[existingSec.ID] {
				sections = append(sections, existingSec)
			}
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
	Name              string            `db:"name"`
	BillingPeriodFrom *string           `db:"billing_period_from"`
	BillingPeriodTo   *string           `db:"billing_period_to"`
	InvoiceFrequency  *string           `db:"invoice_frequency,omitempty"`
	InvoiceMethod     *util.InvoiceType `db:"invoice_method,omitempty"`
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
		Name:              i.Name,
		BillingPeriodFrom: lo.FromPtrOr(i.BillingPeriodFrom, ""),
		BillingPeriodTo:   lo.FromPtrOr(i.BillingPeriodTo, ""),
		InvoiceFrequency:  i.InvoiceFrequency,
		InvoiceMethod:     i.InvoiceMethod,
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
	Name              string             `json:"name"`
	InvoiceMethod     *util.InvoiceType  `json:"invoiceMethod,omitempty"`
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
	Name              string              `json:"name"`
	BillingPeriodFrom string              `json:"billingPeriodFrom"`
	BillingPeriodTo   string              `json:"billingPeriodTo"`
	InvoiceFrequency  *string             `json:"invoiceFrequency,omitempty"`
	IssueDate         time.Time           `json:"issueDate"`
	DueDate           *time.Time          `json:"dueDate,omitempty"`
	Status            *string             `json:"status"`
	InvoiceMethod     *util.InvoiceType   `json:"invoiceMethod,omitempty"`
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

var allowedSectionTypesByInvoiceMethod = map[util.InvoiceType]map[section.SectionType]bool{
	util.InvoiceTypeSFAClinicCollects: {
		section.CALCULATIONSTATEMENT: true,
		section.TAXINVOICE:           true,
		section.REMITTANCEINVOICE:    true,
	},
	util.InvoiceTypeSFADentistCollects: {
		section.CALCULATIONSTATEMENT: true,
		section.TAXINVOICE:           true,
	},
	util.InvoiceTypeIndependentContractor: {
		section.CALCULATIONSTATEMENT: true,
		section.RCTI:                 true,
		section.REMITTANCEINVOICE:    true,
	},
}

func (rq *RqInvoice) Validate() error {
	if rq.ClinicID == uuid.Nil {
		return errors.New("clinic ID is required")
	}

	if rq.InvoiceMethod == nil {
		return errors.New("invoice method is required")
	}

	allowed, ok := allowedSectionTypesByInvoiceMethod[*rq.InvoiceMethod]
	if !ok {
		return fmt.Errorf("invalid invoice method: %v", *rq.InvoiceMethod)
	}

	for _, sec := range rq.Sections {
		if !allowed[sec.SectionType] {
			return fmt.Errorf("invalid section type %v for invoice method %v", sec.SectionType, *rq.InvoiceMethod)
		}
	}

	return nil
}

func (rq *RqUpdateInvoice) Validate() error {
	if rq.ClinicID == uuid.Nil {
		return errors.New("clinic ID is required")
	}

	if rq.InvoiceMethod == nil {
		return errors.New("invoice method is required")
	}

	allowed, ok := allowedSectionTypesByInvoiceMethod[*rq.InvoiceMethod]
	if !ok {
		return fmt.Errorf("invalid invoice method: %v", *rq.InvoiceMethod)
	}

	for _, sec := range rq.Sections {
		if sec.SectionType == nil {
			return errors.New("section type is required")
		}
		if !allowed[*sec.SectionType] {
			return fmt.Errorf("invalid section type %v for invoice method %v", sec.SectionType, *rq.InvoiceMethod)
		}
	}

	return nil
}

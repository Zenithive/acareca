package form

import (
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/engine/formula"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
	StatusArchived  = "ARCHIVED"
)

// RqFieldsSync groups field create/update/delete in a nested object.
type RqFieldsSync struct {
	Create []field.RqCreateField     `json:"create" validate:"omitempty,dive"`
	Update []field.RqUpdateFormField `json:"update" validate:"omitempty,dive"`
	Delete []uuid.UUID               `json:"delete" validate:"omitempty,dive"`
}

type RqBulkSyncFields struct {
	FormID   uuid.UUID                 `json:"form_id" validate:"omitempty,uuid"`
	ClinicID uuid.UUID                 `json:"clinic_id" validate:"required,uuid"`
	Create   []field.RqCreateField     `json:"create" validate:"omitempty,dive"`
	Update   []field.RqUpdateFormField `json:"update" validate:"omitempty,dive"`
	Delete   []uuid.UUID               `json:"delete" validate:"omitempty,dive"`
}

type RsBulkSyncFields struct {
	ClinicID uuid.UUID           `json:"clinic_id"`
	Created  []field.RsFormField `json:"created"`
	Updated  []field.RsFormField `json:"updated"`
	Deleted  []uuid.UUID         `json:"deleted"`
}

type RsFormWithFieldsSyncResult struct {
	ClinicID     uuid.UUID   `json:"clinic_id"`
	CreatedCount int         `json:"created_count"`
	UpdatedCount int         `json:"updated_count"`
	DeletedCount int         `json:"deleted_count"`
	DeletedIDs   []uuid.UUID `json:"deleted_ids,omitempty"`
}

type RqCreateFormWithFields struct {
	Name           string                `json:"name" validate:"required"`
	Description    *string               `json:"description" validate:"omitempty"`
	Status         string                `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED ARCHIVED"`
	Method         string                `json:"method" validate:"required,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE EXPENSE_ENTRY"`
	OwnerShare     int                   `json:"owner_share" validate:"required,min=0,max=100"`
	ClinicShare    int                   `json:"clinic_share" validate:"required,min=0,max=100"`
	SuperComponent *float64              `json:"super_component" validate:"omitempty,min=0,max=100"`
	ClinicID       uuid.UUID             `json:"clinic_id" validate:"required,uuid"`
	Fields         []field.RqCreateField `json:"fields" validate:"omitempty,dive"`
	Formulas       []formula.RqFormula   `json:"formulas" validate:"omitempty,dive"`
}

func (r *RqCreateFormWithFields) ValidateShares() error {
	if r.OwnerShare+r.ClinicShare != 100 {
		return errors.New("owner_share + clinic_share must equal 100")
	}
	return nil
}

type RqUpdateFormWithFields struct {
	ID             *uuid.UUID                `json:"form_id" validate:"omitempty,uuid"`
	ClinicID       uuid.UUID                 `json:"clinic_id" validate:"required,uuid"`
	Name           *string                   `json:"name" validate:"omitempty"`
	Description    *string                   `json:"description" validate:"omitempty"`
	Status         *string                   `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED ARCHIVED"`
	Method         *string                   `json:"method" validate:"omitempty,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE EXPENSE_ENTRY"`
	OwnerShare     *int                      `json:"owner_share" validate:"omitempty,min=0,max=100"`
	ClinicShare    *int                      `json:"clinic_share" validate:"omitempty,min=0,max=100"`
	SuperComponent *float64                  `json:"super_component" validate:"omitempty"`
	Fields         RqFieldsSync              `json:"fields"`
	Create         []field.RqCreateField     `json:"create" validate:"omitempty,dive"`
	Update         []field.RqUpdateFormField `json:"update" validate:"omitempty,dive"`
	Delete         []uuid.UUID               `json:"delete" validate:"omitempty,dive"`
	ForceDelete    *bool                     `json:"force_delete"` // If true, delete fields even if they have submitted entries
	Formulas       []formula.RqFormula       `json:"formulas" validate:"omitempty,dive"`
}

func (r *RqUpdateFormWithFields) ValidateShares() error {
	if r.OwnerShare != nil && r.ClinicShare != nil {
		if *r.OwnerShare+*r.ClinicShare != 100 {
			return errors.New("owner_share + clinic_share must equal 100")
		}
	}
	return nil
}

func (r *RqUpdateFormWithFields) Normalize() {
	if len(r.Create) > 0 || len(r.Update) > 0 || len(r.Delete) > 0 {
		if len(r.Fields.Create) == 0 && len(r.Fields.Update) == 0 && len(r.Fields.Delete) == 0 {
			r.Fields.Create = r.Create
			r.Fields.Update = r.Update
			r.Fields.Delete = r.Delete
		}
	}
}

type RsFormWithFields struct {
	Form            detail.RsFormDetail `json:"form"`
	ActiveVersionID uuid.UUID           `json:"active_version_id"`
	Fields          []field.RsFormField `json:"fields"`
	Formulas        []formula.RsFormula `json:"formulas"`
}

type Filter struct {
	PractitionerID *string `form:"practitioner_id"`
	ClinicIDs      *string `form:"clinic_ids"`
	FormName       *string `form:"form_name"`
	Method         *string `form:"method"`
	Status         *string `form:"status"`
	common.Filter
}

type RqExpense struct {
	Name  string         `json:"name" validate:"required"`
	Date  string         `json:"date" validate:"required"`
	Items []*ExpenseItem `json:"items" validate:"required,min=1,dive"`
}

type ExpenseItem struct {
	Name        string    `json:"name" validate:"required"`
	CoaID       uuid.UUID `json:"coa_id" validate:"required,uuid"`
	BusinessUse float64   `json:"business_use" validate:"required,min=0,max=100"`
	Amount      float64   `json:"amount" validate:"required,gt=0"`
	Description *string   `json:"description,omitempty"`
	TaxType     *string   `json:"tax_type" validate:"omitempty"`
}

type RqUpdateExpense struct {
	Name   string               `json:"name" validate:"required"`
	Date   string               `json:"date" validate:"required"`
	Create []*ExpenseItem       `json:"create,omitempty" validate:"omitempty,dive"`
	Update []*ExpenseItemUpdate `json:"update,omitempty" validate:"omitempty,dive"`
	Delete []uuid.UUID          `json:"delete,omitempty" validate:"omitempty,dive,uuid"`
}

type ExpenseItemUpdate struct {
	ID          uuid.UUID  `json:"id" validate:"required,uuid"`
	Name        *string    `json:"name,omitempty"`
	CoaID       *uuid.UUID `json:"coa_id,omitempty"`
	BusinessUse *float64   `json:"business_use,omitempty" validate:"omitempty,min=0,max=100"`
	Amount      *float64   `json:"amount,omitempty" validate:"omitempty,gt=0"`
	Description *string    `json:"description,omitempty"`
	TaxType     *string    `json:"tax_type,omitempty" validate:"omitempty"`
}

type RsExpense struct {
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Date       string          `json:"date"`
	AmountType string          `json:"amount_type"`
	Items      []RsExpenseItem `json:"items"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  *string         `json:"updated_at,omitempty"`
}

type RsExpenseItem struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	CoaID       uuid.UUID `json:"coa_id"`
	CoaName     string    `json:"coa_name"`
	BusinessUse float64   `json:"business_use"`
	Amount      float64   `json:"amount"`
	NetAmount   float64   `json:"net_amount"`
	GstAmount   float64   `json:"gst_amount"`
	GrossAmount float64   `json:"gross_amount"`
	Description *string   `json:"description,omitempty"`
}

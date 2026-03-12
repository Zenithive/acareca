package form

import (
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
)

type RqBulkSyncFields struct {
	FormID   uuid.UUID                 `json:"form_id" validate:"omitempty,uuid"`
	ClinicID uuid.UUID                 `json:"clinic_id" validate:"required,uuid"`
	Create   []field.RqFormField       `json:"create" validate:"omitempty,dive"`
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
	Name        string              `json:"name" validate:"required"`
	Description *string             `json:"description" validate:"omitempty"`
	Status      string              `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED ARCHIVED"`
	Method      string              `json:"method" validate:"required,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE"`
	OwnerShare  int                 `json:"owner_share" validate:"required,min=0,max=100"`
	ClinicShare int                 `json:"clinic_share" validate:"required,min=0,max=100"`
	ClinicID    uuid.UUID           `json:"clinic_id" validate:"required,uuid"`
	Fields      []field.RqFormField `json:"fields" validate:"omitempty,dive"`
}

type RqUpdateFormWithFields struct {
	ID          uuid.UUID                 `json:"id" validate:"required"`
	Name        *string                   `json:"name" validate:"omitempty"`
	Description *string                   `json:"description" validate:"omitempty"`
	Status      *string                   `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED ARCHIVED"`
	Method      *string                   `json:"method" validate:"omitempty,oneof=INDEPENDENT_CONTRACTOR SERVICE_FEE"`
	OwnerShare  *int                      `json:"owner_share" validate:"omitempty,min=0,max=100"`
	ClinicShare *int                      `json:"clinic_share" validate:"omitempty,min=0,max=100"`
	ClinicID    uuid.UUID                 `json:"clinic_id" validate:"required,uuid"`
	Fields      []field.RqUpdateFormField `json:"fields" validate:"omitempty,dive"`
}

type RsFormWithFields struct {
	Form            detail.RsFormDetail `json:"form"`
	ActiveVersionID uuid.UUID           `json:"active_version_id"`
	Fields          []field.RsFormField `json:"fields"`
}

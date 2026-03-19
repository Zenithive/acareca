package form

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/builder/detail"
	"github.com/iamarpitzala/acareca/internal/modules/builder/entry"
	"github.com/iamarpitzala/acareca/internal/modules/builder/field"
	"github.com/iamarpitzala/acareca/internal/modules/builder/version"
	"github.com/iamarpitzala/acareca/internal/modules/business/coa"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type IService interface {
	GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error)
	CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	UpdateWithFields(ctx context.Context, d *RqUpdateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error)
	GetFormWithFields(ctx context.Context, formID uuid.UUID) (*RsFormWithFields, error)
	List(ctx context.Context, filter Filter, practitionerID uuid.UUID) (*util.RsList, error)
	Delete(ctx context.Context, formID uuid.UUID) error
}

type service struct {
	db         *sqlx.DB
	detailSvc  detail.IService
	versionSvc version.IService
	fieldSvc   field.IService
	entryRepo  entry.IRepository
	coaSvc     coa.Service
	clinicSvc  interface{} // Will be clinic.Service but avoiding circular import
}

func NewService(db *sqlx.DB, detailSvc detail.IService, versionSvc version.IService, fieldSvc field.IService, entryRepo entry.IRepository, coaSvc coa.Service) IService {
	return &service{db: db, detailSvc: detailSvc, versionSvc: versionSvc, fieldSvc: fieldSvc, entryRepo: entryRepo, coaSvc: coaSvc}
}

func (s *service) CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	formReq := &detail.RqFormDetail{
		Name:        d.Name,
		Description: d.Description,
		Status:      d.Status,
		Method:      d.Method,
		OwnerShare:  d.OwnerShare,
		ClinicShare: d.ClinicShare,
	}
	if formReq.Status == "" {
		formReq.Status = StatusDraft
	}

	// Create form via detail service (handles its own transaction)
	created, err := s.detailSvc.Create(ctx, formReq, d.ClinicID, practitionerID)
	if err != nil {
		return nil, nil, err
	}

	syncResult := &RsFormWithFieldsSyncResult{ClinicID: created.ClinicID}

	if len(d.Fields) == 0 {
		return created, syncResult, nil
	}

	// Get active version
	versions, err := s.versionSvc.List(ctx, created.ID, d.ClinicID)
	if err != nil {
		return nil, nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}
	if activeVersionID == uuid.Nil {
		return created, syncResult, nil
	}

	// Create fields within transaction (atomic operation)
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		for _, f := range d.Fields {
			_, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, d.ClinicID, practitionerID, &field.RqFormField{
				Label:                 f.Label,
				SectionType:           f.SectionType,
				PaymentResponsibility: f.PaymentResponsibility,
				TaxType:               f.TaxType,
				CoaID:                 f.CoaID,
			})
			if err != nil {
				return err
			}
			syncResult.CreatedCount++
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return created, syncResult, nil
}

func (s *service) UpdateWithFields(ctx context.Context, req *RqUpdateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	existing, err := s.detailSvc.GetByID(ctx, *req.ID)
	if err != nil {
		return nil, nil, err
	}

	updateReq := &detail.RqUpdateFormDetail{
		ID:          *req.ID,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Method:      req.Method,
		OwnerShare:  req.OwnerShare,
		ClinicShare: req.ClinicShare,
	}

	// Update form metadata via detail service
	updated, err := s.detailSvc.Update(ctx, updateReq, practitionerID)
	if err != nil {
		return nil, nil, err
	}

	syncResult := &RsFormWithFieldsSyncResult{ClinicID: updated.ClinicID}

	hasChanges := len(req.Update) > 0 || len(req.Create) > 0 || len(req.Delete) > 0
	if !hasChanges {
		return updated, syncResult, nil
	}

	// Get active version
	versions, err := s.versionSvc.List(ctx, existing.ID, req.ClinicID)
	if err != nil {
		return nil, nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}
	if activeVersionID == uuid.Nil {
		return updated, syncResult, nil
	}

	// Apply all field changes atomically within a transaction
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Delete fields
		for _, idStr := range req.Delete {
			id, err := uuid.Parse(idStr)
			if err != nil {
				return err
			}
			if s.entryRepo != nil {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
				if err != nil {
					return err
				}
				if has {
					return errors.New("field has submitted entries")
				}
			}
			if err := s.fieldSvc.DeleteTx(ctx, tx, id); err != nil {
				return err
			}
			syncResult.DeletedCount++
		}

		// Update fields
		for _, item := range req.Update {
			_, err = s.fieldSvc.UpdateTx(ctx, tx, item.ID, req.ClinicID, practitionerID, &field.RqUpdateFormField{
				ID:                    item.ID,
				CoaID:                 item.CoaID,
				Label:                 item.Label,
				SectionType:           item.SectionType,
				PaymentResponsibility: item.PaymentResponsibility,
				TaxType:               item.TaxType,
			})
			if err != nil {
				return err
			}
			syncResult.UpdatedCount++
		}

		// Create fields
		for _, item := range req.Create {
			_, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, req.ClinicID, practitionerID, &field.RqFormField{
				CoaID:                 item.CoaID,
				Label:                 item.Label,
				SectionType:           item.SectionType,
				PaymentResponsibility: item.PaymentResponsibility,
				TaxType:               item.TaxType,
			})
			if err != nil {
				return err
			}
			syncResult.CreatedCount++
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return updated, syncResult, nil
}

func (s *service) BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error) {
	// Get form details
	form, err := s.detailSvc.GetByID(ctx, req.FormID)
	if err != nil {
		return nil, err
	}

	result := &RsBulkSyncFields{
		ClinicID: req.ClinicID,
		Created:  []field.RsFormField{},
		Updated:  []field.RsFormField{},
		Deleted:  []uuid.UUID{},
	}

	// Get versions to find active version
	versions, err := s.versionSvc.List(ctx, form.ID, req.ClinicID)
	if err != nil {
		return nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}

	// Apply all field changes atomically within a transaction
	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Delete fields
		for _, fieldID := range req.Delete {
			if s.entryRepo != nil {
				has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, fieldID)
				if err != nil {
					return err
				}
				if has {
					return errors.New("field has submitted entries")
				}
			}
			if err := s.fieldSvc.DeleteTx(ctx, tx, fieldID); err != nil {
				return err
			}
			result.Deleted = append(result.Deleted, fieldID)
		}

		// Update fields
		for _, updateItem := range req.Update {
			updated, err := s.fieldSvc.UpdateTx(ctx, tx, updateItem.ID, req.ClinicID, practitionerID, &updateItem)
			if err != nil {
				return err
			}
			result.Updated = append(result.Updated, *updated)
		}

		// Create fields
		for _, createItem := range req.Create {
			created, err := s.fieldSvc.CreateTx(ctx, tx, activeVersionID, req.ClinicID, practitionerID, &createItem)
			if err != nil {
				return err
			}
			result.Created = append(result.Created, *created)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) GetFormWithFields(ctx context.Context, formID uuid.UUID) (*RsFormWithFields, error) {
	formDetail, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}
	out := &RsFormWithFields{
		Form:   *formDetail,
		Fields: []field.RsFormField{},
	}
	versions, err := s.versionSvc.List(ctx, formDetail.ID, formDetail.ClinicID)
	if err != nil {
		return nil, err
	}
	var activeVersionID uuid.UUID
	for _, v := range versions {
		if v.IsActive {
			activeVersionID = v.Id
			break
		}
	}
	if activeVersionID != uuid.Nil {
		out.ActiveVersionID = activeVersionID
		fields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			out.Fields = append(out.Fields, *f)
		}
	}
	return out, nil
}

func (s *service) List(ctx context.Context, filter Filter, practitionerID uuid.UUID) (*util.RsList, error) {
	// Pass the request to the detail service and return the consolidated result
	return s.detailSvc.List(ctx, detail.Filter{
		ClinicID:  filter.ClinicID,
		FormName:  filter.FormName,
		Status:    filter.Status,
		Method:    filter.Method,
		SortBy:    filter.SortBy,
		SortOrder: filter.SortOrder,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}, practitionerID)
}

func (s *service) Delete(ctx context.Context, formID uuid.UUID) error {
	formDetail, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return err
	}
	return s.detailSvc.Delete(ctx, formDetail.ID)
}

// GetByID implements [IService].
func (s *service) GetFormByID(ctx context.Context, formId uuid.UUID) (*detail.RsFormDetail, error) {

	detail, err := s.detailSvc.GetByID(ctx, formId)
	if err != nil {
		return detail, err
	}

	return detail, err
}

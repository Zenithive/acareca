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
)

type IService interface {
	BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error)
	CreateWithFields(ctx context.Context, d *RqCreateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	UpdateWithFields(ctx context.Context, d *RqUpdateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error)
	GetFormWithFields(ctx context.Context, formID, clinicID uuid.UUID) (*RsFormWithFields, error)
	List(ctx context.Context, filter detail.Filter) ([]*detail.RsFormDetail, error)
	Delete(ctx context.Context, formID, clinicID uuid.UUID) error
}

type service struct {
	detailSvc  detail.IService
	versionSvc version.IService
	fieldSvc   field.IService
	entryRepo  entry.IRepository
	coaSvc     coa.Service
}

func NewService(detailSvc detail.IService, versionSvc version.IService, fieldSvc field.IService, entryRepo entry.IRepository, coaSvc coa.Service) IService {
	return &service{detailSvc: detailSvc, versionSvc: versionSvc, fieldSvc: fieldSvc, entryRepo: entryRepo, coaSvc: coaSvc}
}

func (s *service) BulkSyncFields(ctx context.Context, practitionerID uuid.UUID, req *RqBulkSyncFields) (*RsBulkSyncFields, error) {
	versions, err := s.versionSvc.List(ctx, req.ClinicID, req.ClinicID)
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
	if activeVersionID == uuid.Nil {
		return nil, errors.New("no active version found")
	}
	if s.detailSvc != nil && s.versionSvc != nil {
		status, err := s.versionSvc.GetByID(ctx, activeVersionID)
		if err != nil {
			return nil, err
		}
		if status.IsActive {
			return nil, errors.New("form is not draft for fields")
		}
	}

	out := &RsBulkSyncFields{
		Created: make([]field.RsFormField, 0, len(req.Create)),
		Updated: make([]field.RsFormField, 0, len(req.Update)),
		Deleted: make([]uuid.UUID, 0, len(req.Delete)),
	}

	for _, id := range req.Delete {
		existing, err := s.fieldSvc.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if existing.FormVersionID != activeVersionID {
			return nil, errors.New("field is not in the correct version")
		}
		if s.entryRepo != nil {
			has, err := s.entryRepo.HasSubmittedEntryValuesForField(ctx, id)
			if err != nil {
				return nil, err
			}
			if has {
				return nil, errors.New("field has submitted entries")
			}
		}
		if err := s.fieldSvc.Delete(ctx, id); err != nil {
			return nil, err
		}
		out.Deleted = append(out.Deleted, id)
	}
	for i := range req.Update {
		item := &req.Update[i]
		existing, err := s.fieldSvc.GetByID(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if existing.FormVersionID != activeVersionID {
			return nil, errors.New("field is not in the correct version")
		}
		if item.CoaID != nil {
			coaID, err := uuid.Parse(*item.CoaID)
			if err != nil {
				return nil, err
			}
			if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
				if errors.Is(err, coa.ErrNotFound) {
					return nil, errors.New("coa not found")
				}
				return nil, err
			}
			existing.CoaID = coaID
		}
		if item.Label != nil {
			existing.Label = *item.Label
		}
		if item.SectionType != nil {
			existing.SectionType = *item.SectionType
		}
		if item.PaymentResponsibility != nil {
			existing.PaymentResponsibility = *item.PaymentResponsibility
		}
		if item.TaxType != nil {
			existing.TaxType = *item.TaxType
		}
		if item.SortOrder != nil {
			existing.SortOrder = *item.SortOrder
		}
		updated, err := s.fieldSvc.Update(ctx, existing.ID, req.ClinicID, practitionerID, &field.RqUpdateFormField{
			CoaID:                 item.CoaID,
			Label:                 item.Label,
			SectionType:           item.SectionType,
			PaymentResponsibility: item.PaymentResponsibility,
			TaxType:               item.TaxType,
			SortOrder:             item.SortOrder,
		})
		if err != nil {
			return nil, err
		}
		out.Updated = append(out.Updated, *updated)
	}
	for i := range req.Create {
		item := &req.Create[i]
		coaID, err := uuid.Parse(item.CoaID)
		if err != nil {
			return nil, err
		}
		if _, err := s.coaSvc.GetChartOfAccount(ctx, coaID, practitionerID); err != nil {
			if errors.Is(err, coa.ErrNotFound) {
				return nil, errors.New("coa not found")
			}
			return nil, err
		}
		created, err := s.fieldSvc.Create(ctx, activeVersionID, req.ClinicID, practitionerID, &field.RqFormField{
			CoaID:                 item.CoaID,
			Label:                 item.Label,
			SectionType:           item.SectionType,
			PaymentResponsibility: item.PaymentResponsibility,
			TaxType:               item.TaxType,
			SortOrder:             item.SortOrder,
		})
		if err != nil {
			return nil, err
		}
		out.Created = append(out.Created, *created)
	}
	return out, nil
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
		formReq.Status = detail.StatusDraft
	}
	created, err := s.detailSvc.Create(ctx, formReq, d.ClinicID, practitionerID)
	if err != nil {
		return nil, nil, err
	}
	syncResult := &RsFormWithFieldsSyncResult{}
	if len(d.Fields) == 0 {
		return created, syncResult, nil
	}
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
	createList := make([]field.RqFormField, 0, len(d.Fields))
	for i := range d.Fields {
		f := &d.Fields[i]
		r := field.RqFormField{
			Label:                 f.Label,
			SectionType:           f.SectionType,
			PaymentResponsibility: f.PaymentResponsibility,
			TaxType:               f.TaxType,
			CoaID:                 f.CoaID,
		}
		if f.SortOrder != nil {
			r.SortOrder = f.SortOrder
		}
		createList = append(createList, r)
	}
	bulk, err := s.BulkSyncFields(ctx, practitionerID, &RqBulkSyncFields{
		ClinicID: d.ClinicID,
		Create:   createList,
		Update:   nil,
		Delete:   nil,
	})
	if err != nil {
		return nil, nil, err
	}
	syncResult.CreatedCount = len(bulk.Created)
	syncResult.UpdatedCount = len(bulk.Updated)
	syncResult.DeletedCount = len(bulk.Deleted)
	syncResult.DeletedIDs = bulk.Deleted
	return created, syncResult, nil
}

func (s *service) UpdateWithFields(ctx context.Context, req *RqUpdateFormWithFields, practitionerID uuid.UUID) (*detail.RsFormDetail, *RsFormWithFieldsSyncResult, error) {
	existing, err := s.detailSvc.GetByID(ctx, req.ID)
	if err != nil {
		return nil, nil, err
	}
	syncResult := &RsFormWithFieldsSyncResult{}
	updateReq := &detail.RqUpdateFormDetail{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		Method:      req.Method,
		OwnerShare:  req.OwnerShare,
		ClinicShare: req.ClinicShare,
	}
	updated, err := s.detailSvc.UpdateMetadata(ctx, updateReq)
	if err != nil {
		return nil, nil, err
	}
	if existing.Status != detail.StatusDraft && len(req.Fields) > 0 {
		return nil, nil, errors.New("form is not draft for fields")
	}
	if len(req.Fields) == 0 {
		return updated, syncResult, nil
	}
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
	currentFields, err := s.fieldSvc.ListByFormVersionID(ctx, activeVersionID)
	if err != nil {
		return nil, nil, err
	}
	currentIDs := make(map[uuid.UUID]struct{})
	for _, f := range currentFields {
		currentIDs[f.ID] = struct{}{}
	}
	keepIDs := make(map[uuid.UUID]struct{})
	var createList []field.RqFormField
	var updateList []field.RqUpdateFormField
	for i := range req.Fields {
		f := &req.Fields[i]
		if f.ID != uuid.Nil {
			keepIDs[f.ID] = struct{}{}
			item := field.RqUpdateFormField{
				ID:                    f.ID,
				Label:                 f.Label,
				SectionType:           f.SectionType,
				PaymentResponsibility: f.PaymentResponsibility,
				TaxType:               f.TaxType,
				CoaID:                 f.CoaID,
			}
			if f.SortOrder != nil {
				item.SortOrder = f.SortOrder
			}
			updateList = append(updateList, item)
		} else {
			r := field.RqFormField{
				Label:                 *f.Label,
				SectionType:           *f.SectionType,
				PaymentResponsibility: *f.PaymentResponsibility,
				TaxType:               *f.TaxType,
				CoaID:                 *f.CoaID,
			}
			if f.SortOrder != nil {
				r.SortOrder = f.SortOrder
			}
			createList = append(createList, r)
		}
	}
	var deleteList []uuid.UUID
	for id := range currentIDs {
		if _, keep := keepIDs[id]; !keep {
			deleteList = append(deleteList, id)
		}
	}
	bulk, err := s.BulkSyncFields(ctx, practitionerID, &RqBulkSyncFields{
		ClinicID: req.ClinicID,
		Create:   createList,
		Update:   updateList,
		Delete:   deleteList,
	})
	if err != nil {
		return nil, nil, err
	}
	syncResult.CreatedCount = len(bulk.Created)
	syncResult.UpdatedCount = len(bulk.Updated)
	syncResult.DeletedCount = len(bulk.Deleted)
	syncResult.DeletedIDs = bulk.Deleted
	return updated, syncResult, nil
}

func (s *service) GetFormWithFields(ctx context.Context, formID, clinicID uuid.UUID) (*RsFormWithFields, error) {
	formDetail, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return nil, err
	}
	if formDetail.ClinicID != clinicID {
		return nil, detail.ErrNotFound
	}
	out := &RsFormWithFields{
		Form:   *formDetail,
		Fields: []field.RsFormField{},
	}
	versions, err := s.versionSvc.List(ctx, formDetail.ID, clinicID)
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

func (s *service) List(ctx context.Context, filter detail.Filter) ([]*detail.RsFormDetail, error) {
	return s.detailSvc.ListForm(ctx, filter)
}

func (s *service) Delete(ctx context.Context, formID, clinicID uuid.UUID) error {
	formDetail, err := s.detailSvc.GetByID(ctx, formID)
	if err != nil {
		return err
	}
	if formDetail.ClinicID != clinicID {
		return detail.ErrNotFound
	}
	return s.detailSvc.Delete(ctx, formID)
}

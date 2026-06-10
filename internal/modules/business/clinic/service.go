package clinic

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/admin"
	"github.com/iamarpitzala/acareca/internal/modules/business/invitation"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/modules/notification"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	sharednotification "github.com/iamarpitzala/acareca/internal/shared/notification"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type Service interface {
	CreateClinic(ctx context.Context, practitionerID uuid.UUID, role string, req *RqCreateClinic) (*RsClinic, error)
	ListClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (*util.RsList, error)
	CountClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (int, error)
	GetClinicByID(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) (*RsClinic, error)
	UpdateClinic(ctx context.Context, practitionerID uuid.UUID, role string, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error)
	BulkUpdateClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkUpdateClinic) ([]RsClinic, error)
	DeleteClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) error
	BulkDeleteClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkDeleteClinic) error

	// Internal methods for service-to-service calls (no user validation)
	GetClinicByIDInternal(ctx context.Context, id uuid.UUID) (*RsClinic, error)
	ListClinicsForAccountant(ctx context.Context, accountantID uuid.UUID, filter Filter) (*util.RsList, error)
}

type service struct {
	db              *sqlx.DB
	repo            Repository
	accountantRepo  accountant.Repository
	authRepo        auth.Repository
	fileRepo        file.Repository
	auditSvc        audit.Service
	limitsSvc       limits.Service
	notificationPub *sharednotification.Publisher
	authSvc         auth.Service
	invitationRepo  invitation.Repository
	invitationSvc   invitation.Service
	adminRepo       admin.Repository
}

func NewService(db *sqlx.DB, repo Repository, accRepo accountant.Repository, authRepo auth.Repository, fileRepo file.Repository, auditSvc audit.Service, notificationSvc notification.Service, authSvc auth.Service, invitationRepo invitation.Repository, invitationSvc invitation.Service, adminRepo admin.Repository) Service {
	return &service{
		db:              db,
		repo:            repo,
		accountantRepo:  accRepo,
		authRepo:        authRepo,
		fileRepo:        fileRepo,
		auditSvc:        auditSvc,
		limitsSvc:       limits.NewService(db),
		notificationPub: sharednotification.NewPublisher(notification.NewServiceAdapter(notificationSvc), adminRepo),
		authSvc:         authSvc,
		invitationRepo:  invitationRepo,
		invitationSvc:   invitationSvc,
		adminRepo:       adminRepo,
	}
}

func (s *service) CreateClinic(ctx context.Context, actorID uuid.UUID, role string, req *RqCreateClinic) (*RsClinic, error) {

	if role == util.RoleAccountant {
		actorID = req.PractitionerID
	}

	var result *RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		activeFinancialYearID, err := s.repo.GetActiveFinancialYear(ctx, tx)
		if err != nil {
			return fmt.Errorf("get active financial year: %w", err)
		}

		finalEntityID := actorID
		if req.EntityID != uuid.Nil {
			finalEntityID = req.EntityID
		}

		// Resolve document if provided
		var doc *file.Document
		if req.DocumentId != nil && *req.DocumentId != "" {
			docID, parseErr := uuid.Parse(*req.DocumentId)
			if parseErr == nil {
				doc, _ = s.fileRepo.FindByID(ctx, docID)
			}
		}

		clinic := &Clinic{
			PractitionerID: actorID,
			EntityID:       finalEntityID,
			ProfilePicture: req.ProfilePicture,
			ImageURL:       req.ImageURL,
			Name:           req.Name,
			ABN:            req.ABN,
			Description:    req.Description,
			IsActive:       true,
			Document:       doc,
		}
		if req.IsActive != nil {
			clinic.IsActive = *req.IsActive
		}

		created, err := s.repo.CreateClinic(ctx, tx, clinic)
		if err != nil {
			return fmt.Errorf("create clinic: %w", err)
		}

		// Create financial settings
		financialSettings := &FinancialSettings{
			ClinicID:        created.ID,
			PractitionerID:  actorID,
			FinancialYearID: *activeFinancialYearID,
			LockDate:        nil,
		}

		createdFS, err := s.repo.CreateFinancialSettings(ctx, tx, financialSettings)
		if err != nil {
			return fmt.Errorf("create financial settings: %w", err)
		}

		// Create Address
		var rsAddress *RsClinicAddress
		if req.Address != nil {
			isPrimary := true // Default to true for singular address
			if req.Address.IsPrimary != nil {
				isPrimary = *req.Address.IsPrimary
			}

			clinicAddr := &ClinicAddress{
				ID:        uuid.New(),
				ClinicID:  created.ID,
				Address:   req.Address.Address,
				City:      req.Address.City,
				State:     req.Address.State,
				Postcode:  req.Address.Postcode,
				IsPrimary: isPrimary,
			}

			createdAddr, err := s.repo.CreateClinicAddress(ctx, tx, clinicAddr)
			if err != nil {
				return fmt.Errorf("create address: %w", err)
			}

			rsAddress = &RsClinicAddress{
				ID:        createdAddr.ID,
				Address:   createdAddr.Address,
				City:      createdAddr.City,
				State:     createdAddr.State,
				Postcode:  createdAddr.Postcode,
				IsPrimary: createdAddr.IsPrimary,
			}
		}

		// Create Contacts
		var contacts []RsClinicContact
		for _, cont := range req.Contacts {
			isPrimary := false
			if cont.IsPrimary != nil {
				isPrimary = *cont.IsPrimary
			}

			clinicContact := &ClinicContact{
				ClinicID:    created.ID,
				ContactType: cont.ContactType,
				Value:       cont.Value,
				Label:       cont.Label,
				IsPrimary:   isPrimary,
			}

			createdContact, err := s.repo.CreateClinicContact(ctx, tx, clinicContact)
			if err != nil {
				return fmt.Errorf("create contact: %w", err)
			}

			contacts = append(contacts, RsClinicContact{
				ID:          createdContact.ID,
				ContactType: createdContact.ContactType,
				Value:       createdContact.Value,
				Label:       createdContact.Label,
				IsPrimary:   createdContact.IsPrimary,
			})
		}

		// Map to result struct for use in event/audit
		result = &RsClinic{
			ID:             created.ID,
			PractitionerID: actorID,
			EntityID:       created.EntityID,
			ProfilePicture: created.ProfilePicture,
			ImageURL:       created.ImageURL,
			Name:           created.Name,
			ABN:            created.ABN,
			Description:    created.Description,
			IsActive:       created.IsActive,
			Address:        rsAddress,
			Contacts:       contacts,
			Document:       ToRsDocument(created.Document),
			FinancialSettings: &RsFinancialSettings{
				ID:              createdFS.ID,
				FinancialYearID: createdFS.FinancialYearID,
				LockDate:        createdFS.LockDate,
			},
			CreatedAt: created.CreatedAt,
			UpdatedAt: created.UpdatedAt,
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("create clinic transaction failed: %w", err)
	}

	idStr := result.ID.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:     auditctx.ActionClinicCreated,
		Module:     auditctx.ModuleClinic,
		EntityType: lo.ToPtr(auditctx.EntityClinic),
		EntityID:   &idStr,
		AfterState: result,
	})

	if err := s.notifyClinic(ctx, actorID, util.ActorType(role), util.EventClinicUpdated, "New clinic created"); err != nil {
		log.Printf("[WARN] failed to send clinic creation notification: %v", err)
	}

	return result, nil
}

func (s *service) ListClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (*util.RsList, error) {
	f := filter.MapToFilter()

	var rsList *util.RsList

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		clinics, err := s.repo.ListClinicByPractitioner(ctx, practitionerID, f)
		if err != nil {
			return err
		}

		result := make([]RsClinic, 0, len(clinics))
		for _, clinic := range clinics {
			addresses, addrErr := s.repo.GetClinicAddresses(ctx, tx, clinic.ID)
			if addrErr != nil {
				return addrErr
			}
			var rsAddress *RsClinicAddress
			if len(addresses) > 0 {
				addr := addresses[0]
				for _, a := range addresses {
					if a.IsPrimary {
						addr = a
						break
					}
				}
				rsAddress = &RsClinicAddress{
					ID:        addr.ID,
					Address:   addr.Address,
					City:      addr.City,
					State:     addr.State,
					Postcode:  addr.Postcode,
					IsPrimary: addr.IsPrimary,
				}
			}

			contacts, contErr := s.repo.GetClinicContacts(ctx, tx, clinic.ID)
			if contErr != nil {
				return contErr
			}
			rsContacts := make([]RsClinicContact, len(contacts))
			for i, cont := range contacts {
				rsContacts[i] = RsClinicContact{
					ID:          cont.ID,
					ContactType: cont.ContactType,
					Value:       cont.Value,
					Label:       cont.Label,
					IsPrimary:   cont.IsPrimary,
				}
			}

			financialSettings, fsErr := s.repo.GetFinancialSettings(ctx, tx, clinic.ID)
			if fsErr != nil {
				return fsErr
			}
			var rsFinancialSettings *RsFinancialSettings
			if financialSettings != nil {
				rsFinancialSettings = &RsFinancialSettings{
					ID:              financialSettings.ID,
					FinancialYearID: financialSettings.FinancialYearID,
					LockDate:        financialSettings.LockDate,
				}
			}

			doc, docErr := s.repo.GetDocumentByClinicID(ctx, tx, clinic.ID)
			if docErr != nil {
				return docErr
			}
			clinic.Document = doc

			result = append(result, RsClinic{
				ID:                clinic.ID,
				EntityID:          clinic.EntityID,
				PractitionerID:    clinic.PractitionerID,
				ProfilePicture:    clinic.ProfilePicture,
				ImageURL:          clinic.ImageURL,
				Name:              clinic.Name,
				ABN:               clinic.ABN,
				Description:       clinic.Description,
				IsActive:          clinic.IsActive,
				Address:           rsAddress,
				Contacts:          rsContacts,
				FinancialSettings: rsFinancialSettings,
				Document:          ToRsDocument(clinic.Document),
				CreatedAt:         clinic.CreatedAt,
				UpdatedAt:         clinic.UpdatedAt,
			})
		}

		total, err := s.repo.CountClinicByPractitioner(ctx, practitionerID, f)
		if err != nil {
			return err
		}

		rsList = &util.RsList{}
		rsList.MapToList(result, total, *f.Offset, *f.Limit)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return rsList, nil
}
func (s *service) CountClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (int, error) {
	f := filter.MapToFilter()
	return s.repo.CountClinicByPractitioner(ctx, practitionerID, f)
}

func (s *service) GetClinicByID(ctx context.Context, actorID uuid.UUID, id uuid.UUID) (*RsClinic, error) {

	var result *RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		clinic, err := s.repo.GetClinicByID(ctx, tx, id)
		if err != nil {
			return err
		}

		addresses, err := s.repo.GetClinicAddresses(ctx, tx, id)
		if err != nil {
			return err
		}

		contacts, err := s.repo.GetClinicContacts(ctx, tx, id)
		if err != nil {
			return err
		}

		financialSettings, err := s.repo.GetFinancialSettings(ctx, tx, id)
		if err != nil {
			return err
		}

		doc, err := s.repo.GetDocumentByClinicID(ctx, tx, id)
		if err != nil {
			return err
		}
		clinic.Document = doc

		var rsAddress *RsClinicAddress
		if len(addresses) > 0 {
			addr := addresses[0]
			for _, a := range addresses {
				if a.IsPrimary {
					addr = a
					break
				}
			}
			rsAddress = &RsClinicAddress{
				ID:        addr.ID,
				Address:   addr.Address,
				City:      addr.City,
				State:     addr.State,
				Postcode:  addr.Postcode,
				IsPrimary: addr.IsPrimary,
			}
		}

		rsContacts := make([]RsClinicContact, 0, len(contacts))
		for _, cont := range contacts {
			rsContacts = append(rsContacts, RsClinicContact{
				ID:          cont.ID,
				ContactType: cont.ContactType,
				Value:       cont.Value,
				Label:       cont.Label,
				IsPrimary:   cont.IsPrimary,
			})
		}

		var rsFinancialSettings *RsFinancialSettings
		if financialSettings != nil {
			rsFinancialSettings = &RsFinancialSettings{
				ID:              financialSettings.ID,
				FinancialYearID: financialSettings.FinancialYearID,
				LockDate:        financialSettings.LockDate,
			}
		}

		result = &RsClinic{
			ID:                clinic.ID,
			PractitionerID:    clinic.PractitionerID,
			ProfilePicture:    clinic.ProfilePicture,
			ImageURL:          clinic.ImageURL,
			Name:              clinic.Name,
			ABN:               clinic.ABN,
			Description:       clinic.Description,
			IsActive:          clinic.IsActive,
			Address:           rsAddress,
			Contacts:          rsContacts,
			FinancialSettings: rsFinancialSettings,
			Document:          ToRsDocument(clinic.Document),
			CreatedAt:         clinic.CreatedAt,
			UpdatedAt:         clinic.UpdatedAt,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) DeleteClinic(ctx context.Context, actorID uuid.UUID, id uuid.UUID) error {
	return util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		existing, err := s.repo.GetClinicByID(ctx, tx, id)
		if err != nil {
			return err
		}

		// Cascade Soft-Delete Clinic Addresses
		if err := s.repo.DeleteClinicAddress(ctx, tx, id); err != nil {
			return fmt.Errorf("delete clinic addresses: %w", err)
		}

		// Cascade Soft-Delete Clinic Contacts
		if err := s.repo.DeleteClinicContacts(ctx, tx, id); err != nil {
			return fmt.Errorf("delete clinic contacts: %w", err)
		}

		// Cascade Soft-Delete All Forms, Fields, Versions, Entries & Values
		if err := s.repo.DeleteFormsByClinicID(ctx, tx, id); err != nil {
			return fmt.Errorf("delete clinic custom forms tree: %w", err)
		}

		// Finally, Delete the Core Clinic Entry
		if err := s.repo.DeleteClinic(ctx, tx, id); err != nil {
			return fmt.Errorf("delete clinic: %w", err)
		}

		idStr := id.String()
		s.auditSvc.LogAsync(ctx, &audit.LogEntry{
			Action:      auditctx.ActionClinicDeleted,
			Module:      auditctx.ModuleClinic,
			EntityType:  lo.ToPtr(auditctx.EntityClinic),
			EntityID:    &idStr,
			BeforeState: existing,
		})

		return nil
	})
}

func (s *service) UpdateClinic(ctx context.Context, actorID uuid.UUID, role string, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error) {

	var result *RsClinic
	var beforeState *RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		_, err := s.getClinicByIDInternal(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("get before state: %w", err)
		}

		clinic, err := s.repo.GetClinicByID(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("get clinic: %w", err)
		}

		if req.Name != nil {
			clinic.Name = *req.Name
		}
		if req.ProfilePicture != nil {
			clinic.ProfilePicture = req.ProfilePicture
		}
		if req.ImageURL != nil {
			clinic.ImageURL = req.ImageURL
		}
		if req.ABN != nil {
			clinic.ABN = req.ABN
		}
		if req.Description != nil {
			clinic.Description = req.Description
		}
		if req.IsActive != nil {
			clinic.IsActive = *req.IsActive
		}

		if role == "PRACTITIONER" {
			clinic.PractitionerID = actorID
		}

		if req.EntityID != uuid.Nil {
			clinic.EntityID = req.EntityID
		}

		// Resolve document if provided
		if req.DocumentId != nil {
			if *req.DocumentId == "" {
				clinic.Document = nil
			} else {
				docID, parseErr := uuid.Parse(*req.DocumentId)
				if parseErr == nil {
					doc, _ := s.fileRepo.FindByID(ctx, docID)
					clinic.Document = doc
				}
			}
		}

		_, err = s.repo.UpdateClinic(ctx, tx, clinic)
		if err != nil {
			return fmt.Errorf("update clinic: %w", err)
		}

		if req.Address != nil {
			if req.Address.ID != nil && *req.Address.ID != uuid.Nil {
				existingAddr, err := s.repo.GetAddressByID(ctx, tx, *req.Address.ID)
				if err == nil && existingAddr.ClinicID == clinic.ID {
					if req.Address.Address != nil {
						existingAddr.Address = req.Address.Address
					}
					if req.Address.City != nil {
						existingAddr.City = req.Address.City
					}
					if req.Address.State != nil {
						existingAddr.State = req.Address.State
					}
					if req.Address.Postcode != nil {
						existingAddr.Postcode = req.Address.Postcode
					}
					existingAddr.IsPrimary = true // Usually single address is primary

					err = s.repo.UpdateClinicAddress(ctx, tx, existingAddr)
					if err != nil {
						return fmt.Errorf("update address: %w", err)
					}
				}
			} else {
				newAddr := &ClinicAddress{
					ID:        uuid.New(),
					ClinicID:  clinic.ID,
					Address:   req.Address.Address,
					City:      req.Address.City,
					State:     req.Address.State,
					Postcode:  req.Address.Postcode,
					IsPrimary: true,
				}
				_, err = s.repo.CreateClinicAddress(ctx, tx, newAddr)
				if err != nil {
					return fmt.Errorf("create address: %w", err)
				}
			}
		} else {
			err = s.repo.DeleteClinicAddress(ctx, tx, clinic.ID)
			if err != nil {
				return fmt.Errorf("delete address: %w", err)
			}
		}

		// Update contacts
		existingContacts, _ := s.repo.GetClinicContacts(ctx, tx, clinic.ID)
		incomingIDs := make(map[uuid.UUID]bool)

		for _, cont := range req.Contacts {
			if cont.ID != nil && *cont.ID != uuid.Nil {
				incomingIDs[*cont.ID] = true
				existing, err := s.repo.GetContactByID(ctx, tx, *cont.ID)
				if err == nil && existing.ClinicID == clinic.ID {
					if cont.ContactType != nil {
						existing.ContactType = *cont.ContactType
					}
					if cont.Value != nil {
						existing.Value = *cont.Value
					}
					if cont.IsPrimary != nil {
						existing.IsPrimary = *cont.IsPrimary
					}
					err = s.repo.UpdateClinicContact(ctx, tx, existing)
					if err != nil {
						return fmt.Errorf("update contact: %w", err)
					}
				}
			} else {
				newCont := &ClinicContact{
					ID:          uuid.New(),
					ClinicID:    clinic.ID,
					ContactType: *cont.ContactType,
					Value:       *cont.Value,
					IsPrimary:   cont.IsPrimary != nil && *cont.IsPrimary,
				}
				_, err = s.repo.CreateClinicContact(ctx, tx, newCont)
				if err != nil {
					return fmt.Errorf("create contact: %w", err)
				}
			}
		}

		for _, ex := range existingContacts {
			if !incomingIDs[ex.ID] {
				err = s.repo.DeleteClinicContact(ctx, tx, ex.ID, clinic.ID)
				if err != nil {
					return fmt.Errorf("delete contact: %w", err)
				}
			}
		}

		updatedClinic, err := s.getClinicByIDInternal(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("get updated clinic: %w", err)
		}
		result = updatedClinic

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("update clinic transaction failed: %w", err)
	}

	// Audit log: clinic updated (Async - for both Practitioner and Accountant)

	idStr := id.String()
	s.auditSvc.LogAsync(ctx, &audit.LogEntry{
		Action:      auditctx.ActionClinicUpdated,
		Module:      auditctx.ModuleClinic,
		EntityType:  lo.ToPtr(auditctx.EntityClinic),
		EntityID:    &idStr,
		BeforeState: beforeState,
		AfterState:  result,
	})

	if err := s.notifyClinic(ctx, actorID, util.ActorType(role), util.EventClinicUpdated, "Clinic updated"); err != nil {
		log.Printf("[WARN] failed to send clinic update notification: %v", err)
	}

	return result, nil
}

// GetClinicByIDInternal is for internal service-to-service calls without user validation
func (s *service) GetClinicByIDInternal(ctx context.Context, id uuid.UUID) (*RsClinic, error) {
	var result *RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		clinic, err := s.repo.GetClinicByID(ctx, tx, id)
		if err != nil {
			return err
		}

		addresses, err := s.repo.GetClinicAddresses(ctx, tx, id)
		if err != nil {
			return err
		}

		contacts, err := s.repo.GetClinicContacts(ctx, tx, id)
		if err != nil {
			return err
		}

		financialSettings, err := s.repo.GetFinancialSettings(ctx, tx, id)
		if err != nil {
			return err
		}

		doc, err := s.repo.GetDocumentByClinicID(ctx, tx, id)
		if err != nil {
			return err
		}
		clinic.Document = doc

		var rsAddress *RsClinicAddress
		if len(addresses) > 0 {
			addr := addresses[0]
			for _, a := range addresses {
				if a.IsPrimary {
					addr = a
					break
				}
			}
			rsAddress = &RsClinicAddress{
				ID:        addr.ID,
				Address:   addr.Address,
				City:      addr.City,
				State:     addr.State,
				Postcode:  addr.Postcode,
				IsPrimary: addr.IsPrimary,
			}
		}

		rsContacts := make([]RsClinicContact, 0, len(contacts))
		for _, cont := range contacts {
			rsContacts = append(rsContacts, RsClinicContact{
				ID:          cont.ID,
				ContactType: cont.ContactType,
				Value:       cont.Value,
				Label:       cont.Label,
				IsPrimary:   cont.IsPrimary,
			})
		}

		var rsFinancialSettings *RsFinancialSettings
		if financialSettings != nil {
			rsFinancialSettings = &RsFinancialSettings{
				ID:              financialSettings.ID,
				FinancialYearID: financialSettings.FinancialYearID,
				LockDate:        financialSettings.LockDate,
			}
		}

		result = &RsClinic{
			ID:                clinic.ID,
			PractitionerID:    clinic.PractitionerID,
			ProfilePicture:    clinic.ProfilePicture,
			ImageURL:          clinic.ImageURL,
			Name:              clinic.Name,
			ABN:               clinic.ABN,
			Description:       clinic.Description,
			IsActive:          clinic.IsActive,
			Address:           rsAddress,
			Contacts:          rsContacts,
			FinancialSettings: rsFinancialSettings,
			Document:          ToRsDocument(clinic.Document),
			CreatedAt:         clinic.CreatedAt,
			UpdatedAt:         clinic.UpdatedAt,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *service) BulkUpdateClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkUpdateClinic) ([]RsClinic, error) {
	var results []RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		for _, clinicReq := range req.Clinics {
			if clinicReq.ID == nil {
				return fmt.Errorf("clinic ID is required for bulk update")
			}

			// Perform update within the same transaction
			updatedClinic, err := s.updateClinicIn(ctx, tx, practitionerID, *clinicReq.ID, &clinicReq)
			if err != nil {
				return fmt.Errorf("failed to update clinic %s: %w", clinicReq.ID.String(), err)
			}

			results = append(results, *updatedClinic)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("bulk update clinics transaction failed: %w", err)
	}

	return results, nil
}

func (s *service) BulkDeleteClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkDeleteClinic) error {
	return util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Verify all clinics exist before deletion
		for _, clinicID := range req.ClinicIDs {
			_, err := s.repo.GetClinicByID(ctx, tx, clinicID)
			if err != nil {
				return fmt.Errorf("clinic %s not found: %w", clinicID.String(), err)
			}
		}

		if err := s.repo.BulkDeleteClinics(ctx, req.ClinicIDs); err != nil {
			return fmt.Errorf("failed during structural bulk delete sweep: %w", err)
		}
		return nil
	})
}

// Helper method to get clinic details within a transaction
func (s *service) getClinicByIDInternal(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*RsClinic, error) {
	clinic, err := s.repo.GetClinicByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	addresses, err := s.repo.GetClinicAddresses(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	contacts, err := s.repo.GetClinicContacts(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	financialSettings, err := s.repo.GetFinancialSettings(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	doc, err := s.repo.GetDocumentByClinicID(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	clinic.Document = doc

	var rsAddress *RsClinicAddress
	if len(addresses) > 0 {
		// Default to first, but try to find the marked primary
		addr := addresses[0]
		for _, a := range addresses {
			if a.IsPrimary {
				addr = a
				break
			}
		}
		rsAddress = &RsClinicAddress{
			ID:        addr.ID,
			Address:   addr.Address,
			City:      addr.City,
			State:     addr.State,
			Postcode:  addr.Postcode,
			IsPrimary: addr.IsPrimary,
		}
	}

	rsContacts := make([]RsClinicContact, 0, len(contacts))
	for _, cont := range contacts {
		rsContacts = append(rsContacts, RsClinicContact{
			ID:          cont.ID,
			ContactType: cont.ContactType,
			Value:       cont.Value,
			Label:       cont.Label,
			IsPrimary:   cont.IsPrimary,
		})
	}

	var rsFinancialSettings *RsFinancialSettings
	if financialSettings != nil {
		rsFinancialSettings = &RsFinancialSettings{
			ID:              financialSettings.ID,
			FinancialYearID: financialSettings.FinancialYearID,
			LockDate:        financialSettings.LockDate,
		}
	}

	return &RsClinic{
		ID:                clinic.ID,
		EntityID:          clinic.EntityID,
		PractitionerID:    clinic.PractitionerID,
		ProfilePicture:    clinic.ProfilePicture,
		ImageURL:          clinic.ImageURL,
		Name:              clinic.Name,
		ABN:               clinic.ABN,
		Description:       clinic.Description,
		IsActive:          clinic.IsActive,
		Address:           rsAddress,
		Contacts:          rsContacts,
		FinancialSettings: rsFinancialSettings,
		Document:          ToRsDocument(clinic.Document),
		CreatedAt:         clinic.CreatedAt,
		UpdatedAt:         clinic.UpdatedAt,
	}, nil
}

// Helper method to update clinic within a transaction (used by bulk update)
func (s *service) updateClinicIn(ctx context.Context, tx *sqlx.Tx, actorID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error) {
	clinic, err := s.repo.GetClinicByID(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("get clinic: %w", err)
	}

	// Update clinic fields if provided
	if req.Name != nil {
		clinic.Name = *req.Name
	}
	if req.ProfilePicture != nil {
		clinic.ProfilePicture = req.ProfilePicture
	}
	if req.ImageURL != nil {
		clinic.ImageURL = req.ImageURL
	}
	if req.ABN != nil {
		clinic.ABN = req.ABN
	}
	if req.Description != nil {
		clinic.Description = req.Description
	}
	if req.IsActive != nil {
		clinic.IsActive = *req.IsActive
	}

	// Resolve document if provided
	if req.DocumentId != nil {
		if *req.DocumentId == "" {
			clinic.Document = nil
		} else {
			docID, parseErr := uuid.Parse(*req.DocumentId)
			if parseErr == nil {
				doc, _ := s.fileRepo.FindByID(ctx, docID)
				clinic.Document = doc
			}
		}
	}

	_, err = s.repo.UpdateClinic(ctx, tx, clinic)
	if err != nil {
		return nil, fmt.Errorf("update clinic: %w", err)
	}

	// Update address
	if req.Address != nil {
		if req.Address.ID != nil && *req.Address.ID != uuid.Nil {
			existingAddr, err := s.repo.GetAddressByID(ctx, tx, *req.Address.ID)
			if err == nil && existingAddr.ClinicID == clinic.ID {
				if req.Address.Address != nil {
					existingAddr.Address = req.Address.Address
				}
				if req.Address.City != nil {
					existingAddr.City = req.Address.City
				}
				if req.Address.State != nil {
					existingAddr.State = req.Address.State
				}
				if req.Address.Postcode != nil {
					existingAddr.Postcode = req.Address.Postcode
				}
				existingAddr.IsPrimary = true
				_ = s.repo.UpdateClinicAddress(ctx, tx, existingAddr)
			}
		} else {
			newAddr := &ClinicAddress{
				ID:        uuid.New(),
				ClinicID:  clinic.ID,
				Address:   req.Address.Address,
				City:      req.Address.City,
				State:     req.Address.State,
				Postcode:  req.Address.Postcode,
				IsPrimary: true,
			}
			_, _ = s.repo.CreateClinicAddress(ctx, tx, newAddr)
		}
	} else {
		_ = s.repo.DeleteClinicAddress(ctx, tx, clinic.ID)
	}

	// Update contacts
	existingContacts, _ := s.repo.GetClinicContacts(ctx, tx, clinic.ID)
	incomingIDs := make(map[uuid.UUID]bool)

	for _, cont := range req.Contacts {
		if cont.ID != nil && *cont.ID != uuid.Nil {
			incomingIDs[*cont.ID] = true
			existing, err := s.repo.GetContactByID(ctx, tx, *cont.ID)
			if err == nil && existing.ClinicID == clinic.ID {
				if cont.ContactType != nil {
					existing.ContactType = *cont.ContactType
				}
				if cont.Value != nil {
					existing.Value = *cont.Value
				}
				if cont.Label != nil {
					existing.Label = cont.Label
				}
				if cont.IsPrimary != nil {
					existing.IsPrimary = *cont.IsPrimary
					if *cont.IsPrimary {
						_ = s.repo.UnsetPrimaryContact(ctx, tx, clinic.ID, *cont.ID)
					}
				}
				_ = s.repo.UpdateClinicContact(ctx, tx, existing)
			}
		} else {
			// CREATE
			newCont := &ClinicContact{
				ID:          uuid.New(),
				ClinicID:    clinic.ID,
				ContactType: *cont.ContactType,
				Value:       *cont.Value,
				Label:       cont.Label,
				IsPrimary:   cont.IsPrimary != nil && *cont.IsPrimary,
			}
			if newCont.IsPrimary {
				_ = s.repo.UnsetPrimaryContact(ctx, tx, clinic.ID, uuid.Nil)
			}
			_, _ = s.repo.CreateClinicContact(ctx, tx, newCont)
		}
	}

	// DELETE contacts not present in request
	for _, ex := range existingContacts {
		if !incomingIDs[ex.ID] {
			_ = s.repo.DeleteClinicContact(ctx, tx, ex.ID, clinic.ID)
		}
	}

	// Update financial settings if provided
	if req.FinancialYearID != nil || req.LockDate != nil {
		financialSettings, err := s.repo.GetFinancialSettings(ctx, tx, clinic.ID)
		if err != nil {
			return nil, fmt.Errorf("get financial settings: %w", err)
		}

		if financialSettings != nil {
			if req.FinancialYearID != nil {
				financialSettings.FinancialYearID = *req.FinancialYearID
			}
			if req.LockDate != nil {
				financialSettings.LockDate = req.LockDate
			}

			if err := s.repo.UpdateFinancialSettings(ctx, tx, financialSettings); err != nil {
				return nil, fmt.Errorf("update financial settings: %w", err)
			}
		}
	}

	// Get the updated clinic with all related data
	return s.getClinicByIDInternal(ctx, tx, id)
}

func (s *service) ListClinicsForAccountant(ctx context.Context, accountantID uuid.UUID, filter Filter) (*util.RsList, error) {
	f := filter.MapToFilter()
	f.PractitionerID = filter.PractitionerID

	var rsList *util.RsList

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		clinics, err := s.repo.ListClinicByAccountant(ctx, accountantID, f)
		if err != nil {
			return err
		}

		result := make([]RsClinic, 0, len(clinics))
		for _, clinic := range clinics {
			addresses, addrErr := s.repo.GetClinicAddresses(ctx, tx, clinic.ID)
			if addrErr != nil {
				return addrErr
			}
			var rsAddress *RsClinicAddress
			if len(addresses) > 0 {
				addr := addresses[0]
				for _, a := range addresses {
					if a.IsPrimary {
						addr = a
						break
					}
				}
				rsAddress = &RsClinicAddress{
					ID:        addr.ID,
					Address:   addr.Address,
					City:      addr.City,
					State:     addr.State,
					Postcode:  addr.Postcode,
					IsPrimary: addr.IsPrimary,
				}
			}

			contacts, contErr := s.repo.GetClinicContacts(ctx, tx, clinic.ID)
			if contErr != nil {
				return contErr
			}
			rsContacts := make([]RsClinicContact, len(contacts))
			for i, cont := range contacts {
				rsContacts[i] = RsClinicContact{
					ID:          cont.ID,
					ContactType: cont.ContactType,
					Value:       cont.Value,
					Label:       cont.Label,
					IsPrimary:   cont.IsPrimary,
				}
			}

			financialSettings, fsErr := s.repo.GetFinancialSettings(ctx, tx, clinic.ID)
			if fsErr != nil {
				return fsErr
			}
			var rsFinancialSettings *RsFinancialSettings
			if financialSettings != nil {
				rsFinancialSettings = &RsFinancialSettings{
					ID:              financialSettings.ID,
					FinancialYearID: financialSettings.FinancialYearID,
					LockDate:        financialSettings.LockDate,
				}
			}

			doc, docErr := s.repo.GetDocumentByClinicID(ctx, tx, clinic.ID)
			if docErr != nil {
				return docErr
			}
			clinic.Document = doc

			result = append(result, RsClinic{
				ID:                clinic.ID,
				EntityID:          clinic.EntityID,
				PractitionerID:    clinic.PractitionerID,
				ProfilePicture:    clinic.ProfilePicture,
				ImageURL:          clinic.ImageURL,
				Name:              clinic.Name,
				ABN:               clinic.ABN,
				Description:       clinic.Description,
				IsActive:          clinic.IsActive,
				Address:           rsAddress,
				Contacts:          rsContacts,
				FinancialSettings: rsFinancialSettings,
				Document:          ToRsDocument(clinic.Document),
				CreatedAt:         clinic.CreatedAt,
				UpdatedAt:         clinic.UpdatedAt,
			})
		}

		total, err := s.repo.CountClinicByAccountant(ctx, accountantID, f)
		if err != nil {
			return err
		}

		rsList = &util.RsList{}
		rsList.MapToList(result, total, *f.Offset, *f.Limit)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return rsList, nil
}

func (s *service) notifyClinic(ctx context.Context, entityID uuid.UUID, recipientType util.ActorType, eventType util.EventType, title string) error {
	if s.notificationPub == nil {
		return fmt.Errorf("notification publisher is nil")
	}

	// Get sender information - the user who triggered the action
	user, err := s.authSvc.GetUserByID(ctx, entityID, recipientType)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	senderName := user.FirstName + " " + user.LastName
	senderType := recipientType

	// Build recipients list
	recipients := []sharednotification.RecipientWithPreferences{}

	switch recipientType {
	case util.ActorPractitioner:
		// If the sender is a practitioner, notify all their linked accountants
		accountants, err := s.invitationRepo.GetAccountantsLinkedToPractitioner(ctx, entityID)
		if err != nil {
			log.Printf("[WARN] failed to get linked accountants for practitioner %s: %v", entityID, err)
			return nil // Don't fail the operation if notification fails
		}

		for _, acc := range accountants {
			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   acc.AccountantID,
				RecipientType: util.ActorAccountant,
				UserID:        acc.UserID,
			})
		}

	case util.ActorAccountant:
		// If the sender is an accountant, notify all linked practitioners
		practitionerIDs, err := s.invitationRepo.GetPractitionersLinkedToAccountant(ctx, entityID)
		if err != nil {
			log.Printf("[WARN] failed to get practitioners for accountant %s: %v", entityID, err)
			return nil // Don't fail the operation if notification fails
		}

		// Notify each linked practitioner
		for _, practitionerID := range practitionerIDs {
			practitionerUserID, err := s.invitationRepo.GetPractitionerUserIDByID(ctx, practitionerID)
			if err != nil {
				log.Printf("[WARN] failed to get user ID for practitioner %s: %v", practitionerID, err)
				continue
			}

			recipients = append(recipients, sharednotification.RecipientWithPreferences{
				RecipientID:   practitionerID,
				RecipientType: util.ActorPractitioner,
				UserID:        practitionerUserID,
			})
		}

	default:
		return fmt.Errorf("unsupported recipient type: %s", recipientType)
	}

	// If no recipients, don't send notification
	if len(recipients) == 0 {
		log.Printf("[INFO] no recipients found for clinic notification")
		return nil
	}

	// Send notifications with preferences using the new publisher
	return s.notificationPub.Publish(ctx, sharednotification.PublishRequest{
		Recipients: recipients,
		SenderID:   entityID,
		SenderType: senderType,
		SenderName: senderName,
		EventType:  eventType,
		EntityType: util.EntityClinic,
		EntityID:   entityID,
		EntityKey:  "clinic_id",
		Title:      title,
		Body:       fmt.Sprintf("%s by %s", title, senderName),
	})
}

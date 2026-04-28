package clinic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/admin/audit"
	"github.com/iamarpitzala/acareca/internal/modules/auth"
	"github.com/iamarpitzala/acareca/internal/modules/business/accountant"
	"github.com/iamarpitzala/acareca/internal/modules/business/shared/events"
	auditctx "github.com/iamarpitzala/acareca/internal/shared/audit"
	"github.com/iamarpitzala/acareca/internal/shared/limits"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateClinic(ctx context.Context, practitionerID uuid.UUID, req *RqCreateClinic) (*RsClinic, error)
	ListClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (*util.RsList, error)
	CountClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (int, error)
	GetClinicByID(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) (*RsClinic, error)
	UpdateClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error)
	BulkUpdateClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkUpdateClinic) ([]RsClinic, error)
	DeleteClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) error
	BulkDeleteClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkDeleteClinic) error

	// Internal methods for service-to-service calls (no user validation)
	GetClinicByIDInternal(ctx context.Context, id uuid.UUID) (*RsClinic, error)
	ListClinicsForAccountant(ctx context.Context, accountantID uuid.UUID, filter Filter) (*util.RsList, error)
}

type service struct {
	db             *sqlx.DB
	repo           Repository
	accountantRepo accountant.Repository
	authRepo       auth.Repository
	auditSvc       audit.Service
	limitsSvc      limits.Service
	eventsSvc      events.Service
}

func NewService(db *sqlx.DB, repo Repository, accRepo accountant.Repository, authRepo auth.Repository, auditSvc audit.Service, eventsSvc events.Service) Service {
	return &service{db: db, repo: repo, accountantRepo: accRepo, authRepo: authRepo, auditSvc: auditSvc, limitsSvc: limits.NewService(db), eventsSvc: eventsSvc}
}

func (s *service) CreateClinic(ctx context.Context, practitionerID uuid.UUID, req *RqCreateClinic) (*RsClinic, error) {
	meta := auditctx.GetMetadata(ctx)

	// Permission checks are handled by middleware - no need to check here

	limitCheckID := practitionerID

	// 2. Perform the limit check
	if err := s.limitsSvc.Check(ctx, limitCheckID, limits.KeyClinicCreate); err != nil {
		return nil, err
	}

	var result *RsClinic

	// 3. Start Transaction
	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Get active financial year
		activeFinancialYearID, err := s.repo.GetActiveFinancialYearTx(ctx, tx)
		if err != nil {
			return fmt.Errorf("get active financial year: %w", err)
		}

		// --- NEW LOGIC: Resolve EntityID ---
		// If an Accountant is creating this, they should pass the EntityID from
		// the tbl_invite_permissions. If it's a Practitioner, it defaults to their ID.
		finalEntityID := practitionerID
		if req.EntityID != uuid.Nil {
			finalEntityID = req.EntityID
		}

		clinic := &Clinic{
			PractitionerID: practitionerID,
			EntityID:       finalEntityID,
			ProfilePicture: req.ProfilePicture,
			Name:           req.Name,
			ABN:            req.ABN,
			Description:    req.Description,
			IsActive:       true,
		}
		if req.IsActive != nil {
			clinic.IsActive = *req.IsActive
		}

		created, err := s.repo.CreateClinicTx(ctx, tx, clinic)
		if err != nil {
			return fmt.Errorf("create clinic: %w", err)
		}

		// Create financial settings
		financialSettings := &FinancialSettings{
			ClinicID:        created.ID,
			PractitionerID:  practitionerID,
			FinancialYearID: *activeFinancialYearID,
			LockDate:        nil,
		}

		createdFS, err := s.repo.CreateFinancialSettingsTx(ctx, tx, financialSettings)
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

			createdAddr, err := s.repo.CreateClinicAddressTx(ctx, tx, clinicAddr)
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

			createdContact, err := s.repo.CreateClinicContactTx(ctx, tx, clinicContact)
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
			PractitionerID: practitionerID,
			EntityID:       created.EntityID,
			ProfilePicture: created.ProfilePicture,
			Name:           created.Name,
			ABN:            created.ABN,
			Description:    created.Description,
			IsActive:       created.IsActive,
			Address:        rsAddress,
			Contacts:       contacts,
			FinancialSettings: &RsFinancialSettings{
				ID:              createdFS.ID,
				FinancialYearID: createdFS.FinancialYearID,
				LockDate:        createdFS.LockDate,
			},
			CreatedAt: created.CreatedAt,
			UpdatedAt: created.UpdatedAt,
		}

		// --- TRIGGER SHARED EVENT RECORD (ACCOUNTANTS ONLY) ---
		if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) && meta.UserID != nil {
			fmt.Println(">>> DEBUG: Accountant detected. Recording Shared Event...")

			actorUserID, err := uuid.Parse(*meta.UserID)
			if err != nil {
				fmt.Printf(">>> DEBUG ERROR: Failed to parse Actor UUID: %v\n", err)
			} else {

				var finalAccountantID uuid.UUID
				accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
				if err == nil {
					finalAccountantID = accProfile.ID
				} else {

					finalAccountantID = actorUserID
				}

				user, err := s.authRepo.FindByID(ctx, actorUserID)
				if err == nil {
					fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)

					// 3. Record the Event
					err = s.eventsSvc.Record(ctx, events.SharedEvent{
						ID:             uuid.New(),
						PractitionerID: practitionerID,
						AccountantID:   finalAccountantID,
						ActorID:        actorUserID,
						ActorName:      &fullName,
						ActorType:      "ACCOUNTANT",
						EventType:      "clinic.created",
						EntityType:     "CLINIC",
						EntityID:       result.ID,
						Description:    fmt.Sprintf("Accountant %s created a new clinic: %s", fullName, result.Name),
						Metadata:       events.JSONBMap{"clinic_name": result.Name},
						CreatedAt:      time.Now(),
					})

					if err != nil {
						fmt.Printf(">>> DEBUG [!] Event Record Failed: %v\n", err)
					} else {
						fmt.Println(">>> DEBUG [!] Shared Event successfully recorded.")
					}
				}
			}
		}

		return nil
	})

	// 4. Handle Transaction Error
	if err != nil {
		return nil, fmt.Errorf("create clinic transaction failed: %w", err)
	}

	// 5. Audit Logging (Async - for both Practitioner and Accountant)
	idStr := result.ID.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionClinicCreated,
		Module:     auditctx.ModuleClinic,
		EntityType: strPtr(auditctx.EntityClinic),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

func (s *service) ListClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (*util.RsList, error) {
	f := filter.MapToFilter()

	clinics, err := s.repo.ListClinicByPractitioner(ctx, practitionerID, f)
	if err != nil {
		return nil, err
	}

	result := make([]RsClinic, 0, len(clinics))
	for _, clinic := range clinics {
		addresses, addrErr := s.repo.GetClinicAddresses(ctx, clinic.ID)
		if addrErr != nil {
			return nil, addrErr
		}
		var rsAddress *RsClinicAddress
		if len(addresses) > 0 {
			// Find the primary address, or default to the first one
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

		contacts, contErr := s.repo.GetClinicContacts(ctx, clinic.ID)
		if contErr != nil {
			return nil, contErr
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

		financialSettings, fsErr := s.repo.GetFinancialSettings(ctx, clinic.ID)
		if fsErr != nil {
			return nil, fsErr
		}
		var rsFinancialSettings *RsFinancialSettings
		if financialSettings != nil {
			rsFinancialSettings = &RsFinancialSettings{
				ID:              financialSettings.ID,
				FinancialYearID: financialSettings.FinancialYearID,
				LockDate:        financialSettings.LockDate,
			}
		}

		result = append(result, RsClinic{
			ID:                clinic.ID,
			EntityID:          clinic.EntityID,
			PractitionerID:    clinic.PractitionerID,
			ProfilePicture:    clinic.ProfilePicture,
			Name:              clinic.Name,
			ABN:               clinic.ABN,
			Description:       clinic.Description,
			IsActive:          clinic.IsActive,
			Address:           rsAddress,
			Contacts:          rsContacts,
			FinancialSettings: rsFinancialSettings,
			CreatedAt:         clinic.CreatedAt,
			UpdatedAt:         clinic.UpdatedAt,
		})
	}

	total, err := s.CountClinic(ctx, practitionerID, filter)
	if err != nil {
		return nil, err
	}

	rsList := &util.RsList{}
	rsList.MapToList(result, total, *f.Offset, *f.Limit)

	return rsList, nil
}

func (s *service) CountClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) (int, error) {
	f := filter.MapToFilter()
	return s.repo.CountClinicByPractitioner(ctx, practitionerID, f)
}

func (s *service) GetClinicByID(ctx context.Context, actorID uuid.UUID, id uuid.UUID) (*RsClinic, error) {
	meta := auditctx.GetMetadata(ctx)

	var ownerID uuid.UUID
	var err error

	// 1. ROLE-BASED IDENTITY RESOLUTION
	if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) {
		// --- ACCOUNTANT FLOW ---
		// actorID is the Accountant Profile ID from your Handler

		// Find which Practitioner owns this clinic context
		ownerID, err = s.CheckPermission(ctx, actorID, id)
		if err != nil {
			return nil, err
		}
	} else {
		// --- PRACTITIONER FLOW ---
		// actorID is already the PractitionerID
		ownerID = actorID
	}

	// 2. FETCH CLINIC
	// Now ownerID is guaranteed to be the Practitioner's ID for both roles
	clinic, err := s.repo.GetClinicByIDAndPractitioner(ctx, id, ownerID)
	if err != nil {
		// If it fails here for a Practitioner, it means the actorID
		// in the token doesn't match the practitioner_id in tbl_clinic
		return nil, err
	}

	addresses, err := s.repo.GetClinicAddresses(ctx, id)
	if err != nil {
		return nil, err
	}

	contacts, err := s.repo.GetClinicContacts(ctx, id)
	if err != nil {
		return nil, err
	}

	financialSettings, err := s.repo.GetFinancialSettings(ctx, id)
	if err != nil {
		return nil, err
	}

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
		ID: clinic.ID,
		//EntityID:          clinic.EntityID,
		PractitionerID:    clinic.PractitionerID,
		ProfilePicture:    clinic.ProfilePicture,
		Name:              clinic.Name,
		ABN:               clinic.ABN,
		Description:       clinic.Description,
		IsActive:          clinic.IsActive,
		Address:           rsAddress,
		Contacts:          rsContacts,
		FinancialSettings: rsFinancialSettings,
		CreatedAt:         clinic.CreatedAt,
		UpdatedAt:         clinic.UpdatedAt,
	}, nil
}

func (s *service) DeleteClinic(ctx context.Context, actorID uuid.UUID, id uuid.UUID) error {

	meta := auditctx.GetMetadata(ctx)
	ownerID, err := s.CheckPermission(ctx, actorID, id)
	if err != nil {
		return err
	}

	return util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {

		existing, err := s.repo.GetClinicByIDAndPractitionerTx(ctx, tx, id, ownerID)
		if err != nil {
			fmt.Printf(">>> DEBUG ERROR: Clinic not found for deletion: %v\n", err)
			return err
		}

		// 2. Perform the actual deletion
		if err := s.repo.DeleteClinicTx(ctx, tx, id); err != nil {
			return fmt.Errorf("delete clinic: %w", err)
		}

		if err := s.repo.DeletePermissionsByEntity(ctx, id, "CLINIC"); err != nil {
			// Log the error but don't fail the request since the clinic is already deleted
			fmt.Printf("Alert: Clinic %s deleted but permissions cleanup failed: %v\n", id, err)
			s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemWarning, "clinic.delete_permissions",
				err, "", id.String(), auditctx.EntityClinic, auditctx.ModuleClinic,
			)
		}
		// --- TRIGGER SHARED EVENT RECORD (ACCOUNTANTS ONLY) ---

		if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) && meta.UserID != nil {
			fmt.Println(">>> DEBUG: Accountant detected. Recording Shared Event for deletion...")

			actorUserID, err := uuid.Parse(*meta.UserID)
			if err == nil {

				var finalAccountantID uuid.UUID
				accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
				if err == nil {
					finalAccountantID = accProfile.ID
					fmt.Printf(">>> DEBUG: Resolved Accountant Profile ID: %s\n", finalAccountantID)
				} else {
					finalAccountantID = actorUserID
				}

				user, err := s.authRepo.FindByID(ctx, actorUserID)
				if err == nil {
					fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)

					// 3. Record the Event
					err = s.eventsSvc.Record(ctx, events.SharedEvent{
						ID:             uuid.New(),
						PractitionerID: ownerID,
						AccountantID:   finalAccountantID,
						ActorID:        actorUserID,
						ActorName:      &fullName,
						ActorType:      "ACCOUNTANT",
						EventType:      "clinic.deleted",
						EntityType:     "CLINIC",
						EntityID:       id,
						Description:    fmt.Sprintf("Accountant %s deleted clinic: %s", fullName, existing.Name),
						Metadata:       events.JSONBMap{"clinic_name": existing.Name},
						CreatedAt:      time.Now(),
					})

					if err != nil {
						fmt.Printf(">>> DEBUG ERROR: Shared Event Record failed: %v\n", err)
					} else {
						fmt.Println(">>> DEBUG SUCCESS: Shared Event recorded for deletion.")
					}
				}
			}
		} else {
			fmt.Println(">>> DEBUG: Action by Practitioner. Skipping Shared Event record.")
		}

		// 3. Original Audit Log (Async - captures all users)
		idStr := id.String()
		s.auditSvc.LogAsync(&audit.LogEntry{
			PracticeID:  meta.PracticeID,
			UserID:      meta.UserID,
			Action:      auditctx.ActionClinicDeleted,
			Module:      auditctx.ModuleClinic,
			EntityType:  strPtr(auditctx.EntityClinic),
			EntityID:    &idStr,
			BeforeState: existing,
			IPAddress:   meta.IPAddress,
			UserAgent:   meta.UserAgent,
		})

		return nil
	})
}

func (s *service) UpdateClinic(ctx context.Context, actorID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error) {

	ownerID, err := s.CheckPermission(ctx, actorID, id)
	if err != nil {
		return nil, err
	}

	// Permission checks are handled by middleware
	meta := auditctx.GetMetadata(ctx)

	var result *RsClinic

	err = util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		clinic, err := s.repo.GetClinicByIDAndPractitionerTx(ctx, tx, id, ownerID)
		if err != nil {
			return fmt.Errorf("get clinic: %w", err)
		}

		// Update clinic fields if provided
		if req.Name != nil {
			clinic.Name = *req.Name
		}
		if req.ProfilePicture != nil {
			clinic.ProfilePicture = req.ProfilePicture
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

		clinic.PractitionerID = ownerID

		// Update EntityID only if a new one is provided in the request
		if req.EntityID != uuid.Nil {
			clinic.EntityID = req.EntityID
		}

		_, err = s.repo.UpdateClinicTx(ctx, tx, clinic)
		if err != nil {
			return fmt.Errorf("update clinic: %w", err)
		}

		// Update address
		if req.Address != nil {
			if req.Address.ID != nil && *req.Address.ID != uuid.Nil {
				// UPDATE EXISTING
				existingAddr, err := s.repo.GetAddressByIDTx(ctx, tx, *req.Address.ID)
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

					_ = s.repo.UpdateClinicAddressTx(ctx, tx, existingAddr)
				}
			} else {
				// CREATE NEW
				newAddr := &ClinicAddress{
					ID:        uuid.New(),
					ClinicID:  clinic.ID,
					Address:   req.Address.Address,
					City:      req.Address.City,
					State:     req.Address.State,
					Postcode:  req.Address.Postcode,
					IsPrimary: true,
				}
				_, _ = s.repo.CreateClinicAddressTx(ctx, tx, newAddr)
			}
		} else {
			// ABSENCE = DELETE
			_ = s.repo.DeleteClinicAddressTx(ctx, tx, clinic.ID)
		}

		// Update contacts
		existingContacts, _ := s.repo.GetClinicContactsTx(ctx, tx, clinic.ID)
		incomingIDs := make(map[uuid.UUID]bool)

		for _, cont := range req.Contacts {
			if cont.ID != nil && *cont.ID != uuid.Nil {
				// UPDATE existing
				incomingIDs[*cont.ID] = true
				existing, err := s.repo.GetContactByIDTx(ctx, tx, *cont.ID)
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
					_ = s.repo.UpdateClinicContactTx(ctx, tx, existing)
				}
			} else {
				// CREATE new
				newCont := &ClinicContact{
					ID:          uuid.New(),
					ClinicID:    clinic.ID,
					ContactType: *cont.ContactType,
					Value:       *cont.Value,
					IsPrimary:   cont.IsPrimary != nil && *cont.IsPrimary,
				}
				_, _ = s.repo.CreateClinicContactTx(ctx, tx, newCont)
			}
		}

		// DELETE contacts that weren't in the request
		for _, ex := range existingContacts {
			if !incomingIDs[ex.ID] {
				_ = s.repo.DeleteClinicContactTx(ctx, tx, ex.ID, clinic.ID)
			}
		}

		// Get the updated clinic with all related data
		updatedClinic, err := s.getClinicByIDInternalTx(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("get updated clinic: %w", err)
		}
		result = updatedClinic

		// --- TRIGGER SHARED EVENT RECORD (ACCOUNTANTS ONLY) ---
		//meta := auditctx.GetMetadata(ctx)
		if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) && meta.UserID != nil {
			actorUserID, err := uuid.Parse(*meta.UserID)
			if err == nil {

				var finalAccountantID uuid.UUID
				accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorUserID.String())
				if err == nil {
					finalAccountantID = accProfile.ID
				} else {
					finalAccountantID = actorUserID
				}

				user, err := s.authRepo.FindByID(ctx, actorUserID)
				if err == nil {
					fullName := fmt.Sprintf("%s %s", user.FirstName, user.LastName)

					_ = s.eventsSvc.Record(ctx, events.SharedEvent{
						ID:             uuid.New(),
						PractitionerID: ownerID,
						AccountantID:   finalAccountantID,
						ActorID:        actorUserID,
						ActorName:      &fullName,
						ActorType:      "ACCOUNTANT",
						EventType:      "clinic.updated",
						EntityType:     "CLINIC",
						EntityID:       result.ID,
						Description:    fmt.Sprintf("Accountant %s updated clinic: %s", fullName, result.Name),
						Metadata:       events.JSONBMap{"clinic_name": result.Name},
						CreatedAt:      time.Now(),
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("update clinic transaction failed: %w", err)
	}

	// Audit log: clinic updated (Async - for both Practitioner and Accountant)

	idStr := id.String()
	s.auditSvc.LogAsync(&audit.LogEntry{
		PracticeID: meta.PracticeID,
		UserID:     meta.UserID,
		Action:     auditctx.ActionClinicUpdated,
		Module:     auditctx.ModuleClinic,
		EntityType: strPtr(auditctx.EntityClinic),
		EntityID:   &idStr,
		AfterState: result,
		IPAddress:  meta.IPAddress,
		UserAgent:  meta.UserAgent,
	})

	return result, nil
}

// GetClinicByIDInternal is for internal service-to-service calls without user validation
func (s *service) GetClinicByIDInternal(ctx context.Context, id uuid.UUID) (*RsClinic, error) {
	clinic, err := s.repo.GetClinicByID(ctx, id)
	if err != nil {
		return nil, err
	}

	addresses, err := s.repo.GetClinicAddresses(ctx, id)
	if err != nil {
		return nil, err
	}

	contacts, err := s.repo.GetClinicContacts(ctx, id)
	if err != nil {
		return nil, err
	}

	financialSettings, err := s.repo.GetFinancialSettings(ctx, id)
	if err != nil {
		return nil, err
	}

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
		PractitionerID:    clinic.PractitionerID,
		ProfilePicture:    clinic.ProfilePicture,
		Name:              clinic.Name,
		ABN:               clinic.ABN,
		Description:       clinic.Description,
		IsActive:          clinic.IsActive,
		Address:           rsAddress,
		Contacts:          rsContacts,
		FinancialSettings: rsFinancialSettings,
		CreatedAt:         clinic.CreatedAt,
		UpdatedAt:         clinic.UpdatedAt,
	}, nil
}

func (s *service) BulkUpdateClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkUpdateClinic) ([]RsClinic, error) {
	var results []RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		for _, clinicReq := range req.Clinics {
			if clinicReq.ID == nil {
				return fmt.Errorf("clinic ID is required for bulk update")
			}

			// Perform update within the same transaction
			updatedClinic, err := s.updateClinicInTx(ctx, tx, practitionerID, *clinicReq.ID, &clinicReq)
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
	for _, clinicID := range req.ClinicIDs {
		_, err := s.repo.GetClinicByIDAndPractitioner(ctx, clinicID, practitionerID)
		if err != nil {
			return fmt.Errorf("clinic %s not found or access denied: %w", clinicID.String(), err)
		}
	}

	return s.repo.BulkDeleteClinics(ctx, req.ClinicIDs)
}

// Helper method to get clinic details within a transaction
func (s *service) getClinicByIDInternalTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (*RsClinic, error) {
	clinic, err := s.repo.GetClinicByIDTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	addresses, err := s.repo.GetClinicAddressesTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	contacts, err := s.repo.GetClinicContactsTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	financialSettings, err := s.repo.GetFinancialSettingsTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}

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
		Name:              clinic.Name,
		ABN:               clinic.ABN,
		Description:       clinic.Description,
		IsActive:          clinic.IsActive,
		Address:           rsAddress,
		Contacts:          rsContacts,
		FinancialSettings: rsFinancialSettings,
		CreatedAt:         clinic.CreatedAt,
		UpdatedAt:         clinic.UpdatedAt,
	}, nil
}

// Helper method to update clinic within a transaction (used by bulk update)
func (s *service) updateClinicInTx(ctx context.Context, tx *sqlx.Tx, actorID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error) {
	ownerID, err := s.verifyAccess(ctx, actorID, id)
	if err != nil {
		return nil, fmt.Errorf("access denied: %w", err)
	}

	clinic, err := s.repo.GetClinicByIDAndPractitionerTx(ctx, tx, id, ownerID)
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
	if req.ABN != nil {
		clinic.ABN = req.ABN
	}
	if req.Description != nil {
		clinic.Description = req.Description
	}
	if req.IsActive != nil {
		clinic.IsActive = *req.IsActive
	}

	_, err = s.repo.UpdateClinicTx(ctx, tx, clinic)
	if err != nil {
		return nil, fmt.Errorf("update clinic: %w", err)
	}

	// Update address
	if req.Address != nil {
		if req.Address.ID != nil && *req.Address.ID != uuid.Nil {
			// UPDATE
			existingAddr, err := s.repo.GetAddressByIDTx(ctx, tx, *req.Address.ID)
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
				_ = s.repo.UpdateClinicAddressTx(ctx, tx, existingAddr)
			}
		} else {
			// CREATE
			newAddr := &ClinicAddress{
				ID:        uuid.New(),
				ClinicID:  clinic.ID,
				Address:   req.Address.Address,
				City:      req.Address.City,
				State:     req.Address.State,
				Postcode:  req.Address.Postcode,
				IsPrimary: true,
			}
			_, _ = s.repo.CreateClinicAddressTx(ctx, tx, newAddr)
		}
	} else {
		// ABSENCE = DELETE
		_ = s.repo.DeleteClinicAddressTx(ctx, tx, clinic.ID)
	}

	// Update contacts
	existingContacts, _ := s.repo.GetClinicContactsTx(ctx, tx, clinic.ID)
	incomingIDs := make(map[uuid.UUID]bool)

	for _, cont := range req.Contacts {
		if cont.ID != nil && *cont.ID != uuid.Nil {
			// UPDATE
			incomingIDs[*cont.ID] = true
			existing, err := s.repo.GetContactByIDTx(ctx, tx, *cont.ID)
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
						_ = s.repo.UnsetPrimaryContactTx(ctx, tx, clinic.ID, *cont.ID)
					}
				}
				_ = s.repo.UpdateClinicContactTx(ctx, tx, existing)
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
				_ = s.repo.UnsetPrimaryContactTx(ctx, tx, clinic.ID, uuid.Nil)
			}
			_, _ = s.repo.CreateClinicContactTx(ctx, tx, newCont)
		}
	}

	// DELETE contacts not present in request
	for _, ex := range existingContacts {
		if !incomingIDs[ex.ID] {
			_ = s.repo.DeleteClinicContactTx(ctx, tx, ex.ID, clinic.ID)
		}
	}

	// Update financial settings if provided
	if req.FinancialYearID != nil || req.LockDate != nil {
		financialSettings, err := s.repo.GetFinancialSettingsTx(ctx, tx, clinic.ID)
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

			if err := s.repo.UpdateFinancialSettingsTx(ctx, tx, financialSettings); err != nil {
				return nil, fmt.Errorf("update financial settings: %w", err)
			}
		}
	}

	// Get the updated clinic with all related data
	return s.getClinicByIDInternalTx(ctx, tx, id)
}

func strPtr(s string) *string { return &s }

func (s *service) verifyAccess(ctx context.Context, actorID uuid.UUID, clinicID uuid.UUID) (uuid.UUID, error) {
	meta := auditctx.GetMetadata(ctx)

	// If Accountant, check permission bridge
	if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) {
		// This repo method should check tbl_invite_permissions
		permission, err := s.CheckPermission(ctx, actorID, clinicID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("access denied for accountant: %w", err)
		}
		return permission, nil
	}

	// If Practitioner, they are the owner
	return actorID, nil
}

func (s *service) CheckPermission(ctx context.Context, actorID uuid.UUID, clinicID uuid.UUID) (uuid.UUID, error) {
	meta := auditctx.GetMetadata(ctx)

	// 1. Accountant Flow
	if meta.UserType != nil && strings.EqualFold(*meta.UserType, util.RoleAccountant) {

		accProfile, err := s.accountantRepo.GetAccountantByUserID(ctx, actorID.String())
		var finalAccountantID uuid.UUID

		if err != nil {

			finalAccountantID = actorID
		} else {

			finalAccountantID = accProfile.ID
		}
		// Query tbl_invite_permissions
		permission, err := s.repo.GetAccountantPermission(ctx, finalAccountantID, clinicID)
		if err != nil {
			s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "clinic.check_permission",
				err, finalAccountantID.String(), clinicID.String(), auditctx.EntityClinic, auditctx.ModuleClinic,
			)
			// Return a clear error for the handler to map to 403 Forbidden
			return uuid.Nil, fmt.Errorf("accountant access denied for clinic %s: %w", clinicID, err)
		}

		// Return the Practitioner who owns the data
		return permission.PractitionerID, nil
	}

	// 2. Practitioner Flow
	pracIDPtr, err := s.repo.GetPractitionerIDByUserID(ctx, actorID.String())
	if err != nil {
		return uuid.Nil, fmt.Errorf("practitioner profile not found: %w", err)
	}

	ownerID := *pracIDPtr

	// 2. Practitioner Flow - check if they own the clinic
	exists, err := s.repo.IsClinicOwner(ctx, ownerID, clinicID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("database error checking ownership: %w", err)
	}
	if !exists {
		s.auditSvc.LogSystemIssue(ctx, auditctx.ActionSystemError, "clinic.check_permission",
			fmt.Errorf("practitioner %s attempted to access clinic %s they do not own", actorID, clinicID),
			actorID.String(), clinicID.String(), auditctx.EntityClinic, auditctx.ModuleClinic,
		)
		return uuid.Nil, fmt.Errorf("practitioner %s does not own clinic %s", actorID, clinicID)
	}

	return ownerID, nil
}

func (s *service) ListClinicsForAccountant(ctx context.Context, accountantID uuid.UUID, filter Filter) (*util.RsList, error) {
	f := filter.MapToFilter()

	f.PractitionerID = filter.PractitionerID

	// 1. Call a specific repository method for accountants
	// This repo method should only return clinics the accountant is invited to
	clinics, err := s.repo.ListClinicByAccountant(ctx, accountantID, f)
	if err != nil {
		return nil, err
	}

	result := make([]RsClinic, 0, len(clinics))
	for _, clinic := range clinics {
		// --- The following logic remains identical to ListClinic ---

		// Fetch Addresses
		addresses, addrErr := s.repo.GetClinicAddresses(ctx, clinic.ID)
		if addrErr != nil {
			return nil, addrErr
		}
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

		// Fetch Contacts
		contacts, contErr := s.repo.GetClinicContacts(ctx, clinic.ID)
		if contErr != nil {
			return nil, contErr
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

		// Fetch Financial Settings
		financialSettings, fsErr := s.repo.GetFinancialSettings(ctx, clinic.ID)
		if fsErr != nil {
			return nil, fsErr
		}
		var rsFinancialSettings *RsFinancialSettings
		if financialSettings != nil {
			rsFinancialSettings = &RsFinancialSettings{
				ID:              financialSettings.ID,
				FinancialYearID: financialSettings.FinancialYearID,
				LockDate:        financialSettings.LockDate,
			}
		}

		// Append to result
		result = append(result, RsClinic{
			ID:                clinic.ID,
			EntityID:          clinic.EntityID,
			PractitionerID:    clinic.PractitionerID,
			ProfilePicture:    clinic.ProfilePicture,
			Name:              clinic.Name,
			ABN:               clinic.ABN,
			Description:       clinic.Description,
			IsActive:          clinic.IsActive,
			Address:           rsAddress,
			Contacts:          rsContacts,
			FinancialSettings: rsFinancialSettings,
			CreatedAt:         clinic.CreatedAt,
			UpdatedAt:         clinic.UpdatedAt,
		})
	}

	// 2. Count using the accountant-specific count method
	total, err := s.repo.CountClinicByAccountant(ctx, accountantID, f)
	if err != nil {
		return nil, err
	}

	rsList := &util.RsList{}
	rsList.MapToList(result, total, *f.Offset, *f.Limit)

	return rsList, nil
}

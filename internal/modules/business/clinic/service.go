package clinic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateClinic(ctx context.Context, practitionerID uuid.UUID, req *RqCreateClinic) (*RsClinic, error)
	ListClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) ([]RsClinic, error)
	GetClinicByID(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) (*RsClinic, error)
	UpdateClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error)
	BulkUpdateClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkUpdateClinic) ([]RsClinic, error)
	DeleteClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) error
	BulkDeleteClinics(ctx context.Context, practitionerID uuid.UUID, req *RqBulkDeleteClinic) error

	// Internal methods for service-to-service calls (no user validation)
	GetClinicByIDInternal(ctx context.Context, id uuid.UUID) (*RsClinic, error)
}

type service struct {
	repo Repository
	db   *sqlx.DB
}

func NewService(db *sqlx.DB, repo Repository) Service {
	return &service{db: db, repo: repo}
}

func (s *service) CreateClinic(ctx context.Context, practitionerID uuid.UUID, req *RqCreateClinic) (*RsClinic, error) {
	var result *RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		// Get active financial year
		activeFinancialYearID, err := s.repo.GetActiveFinancialYearTx(ctx, tx)
		if err != nil {
			return fmt.Errorf("get active financial year: %w", err)
		}

		clinic := &Clinic{
			PractitionerID: practitionerID,
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

		// Create financial settings with active financial year
		financialSettings := &FinancialSettings{
			ClinicID:        created.ID,
			FinancialYearID: *activeFinancialYearID,
			LockDate:        nil, // Keep empty as requested
		}

		createdFS, err := s.repo.CreateFinancialSettingsTx(ctx, tx, financialSettings)
		if err != nil {
			return fmt.Errorf("create financial settings: %w", err)
		}

		var addresses []RsClinicAddress
		for _, addr := range req.Addresses {
			isPrimary := false
			if addr.IsPrimary != nil {
				isPrimary = *addr.IsPrimary
			}

			clinicAddr := &ClinicAddress{
				ClinicID:  created.ID,
				Address:   addr.Address,
				City:      addr.City,
				State:     addr.State,
				Postcode:  addr.Postcode,
				IsPrimary: isPrimary,
			}

			createdAddr, err := s.repo.CreateClinicAddressTx(ctx, tx, clinicAddr)
			if err != nil {
				return fmt.Errorf("create address: %w", err)
			}

			addresses = append(addresses, RsClinicAddress{
				ID:        createdAddr.ID,
				Address:   createdAddr.Address,
				City:      createdAddr.City,
				State:     createdAddr.State,
				Postcode:  createdAddr.Postcode,
				IsPrimary: createdAddr.IsPrimary,
			})
		}

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

		result = &RsClinic{
			ID:             created.ID,
			PractitionerID: created.PractitionerID,
			ProfilePicture: created.ProfilePicture,
			Name:           created.Name,
			ABN:            created.ABN,
			Description:    created.Description,
			IsActive:       created.IsActive,
			Addresses:      addresses,
			Contacts:       contacts,
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

	return result, nil
}

func (s *service) ListClinic(ctx context.Context, practitionerID uuid.UUID, filter Filter) ([]RsClinic, error) {
	f := filter.MapToFilter()

	clinics, err := s.repo.ListClinicByPractitioner(ctx, practitionerID, f)
	if err != nil {
		return nil, err
	}

	result := make([]RsClinic, 0, len(clinics))
	for _, clinic := range clinics {
		// Get addresses for this clinic
		addresses, err := s.repo.GetClinicAddresses(ctx, clinic.ID)
		if err != nil {
			return nil, err
		}

		// Get contacts for this clinic
		contacts, err := s.repo.GetClinicContacts(ctx, clinic.ID)
		if err != nil {
			return nil, err
		}

		// Get financial settings for this clinic
		financialSettings, err := s.repo.GetFinancialSettings(ctx, clinic.ID)
		if err != nil {
			return nil, err
		}

		// Convert addresses to response format
		rsAddresses := make([]RsClinicAddress, 0, len(addresses))
		for _, addr := range addresses {
			rsAddresses = append(rsAddresses, RsClinicAddress{
				ID:        addr.ID,
				Address:   addr.Address,
				City:      addr.City,
				State:     addr.State,
				Postcode:  addr.Postcode,
				IsPrimary: addr.IsPrimary,
			})
		}

		// Convert contacts to response format
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

		// Convert financial settings to response format
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
			PractitionerID:    clinic.PractitionerID,
			ProfilePicture:    clinic.ProfilePicture,
			Name:              clinic.Name,
			ABN:               clinic.ABN,
			Description:       clinic.Description,
			IsActive:          clinic.IsActive,
			Addresses:         rsAddresses,
			Contacts:          rsContacts,
			FinancialSettings: rsFinancialSettings,
			CreatedAt:         clinic.CreatedAt,
			UpdatedAt:         clinic.UpdatedAt,
		})
	}

	return result, nil
}

func (s *service) GetClinicByID(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) (*RsClinic, error) {

	clinic, err := s.repo.GetClinicByIDAndPractitioner(ctx, id, practitionerID)
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

	rsAddresses := make([]RsClinicAddress, 0, len(addresses))
	for _, addr := range addresses {
		rsAddresses = append(rsAddresses, RsClinicAddress{
			ID:        addr.ID,
			Address:   addr.Address,
			City:      addr.City,
			State:     addr.State,
			Postcode:  addr.Postcode,
			IsPrimary: addr.IsPrimary,
		})
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
		Addresses:         rsAddresses,
		Contacts:          rsContacts,
		FinancialSettings: rsFinancialSettings,
		CreatedAt:         clinic.CreatedAt,
		UpdatedAt:         clinic.UpdatedAt,
	}, nil
}

func (s *service) DeleteClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) error {
	_, err := s.repo.GetClinicByIDAndPractitioner(ctx, id, practitionerID)
	if err != nil {
		return err
	}

	return s.repo.DeleteClinic(ctx, id)
}
func (s *service) UpdateClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error) {
	var result *RsClinic

	err := util.RunInTransaction(ctx, s.db, func(ctx context.Context, tx *sqlx.Tx) error {
		clinic, err := s.repo.GetClinicByIDAndPractitionerTx(ctx, tx, id, practitionerID)
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

		_, err = s.repo.UpdateClinicTx(ctx, tx, clinic)
		if err != nil {
			return fmt.Errorf("update clinic: %w", err)
		}

		// Update addresses
		for _, addr := range req.Addresses {
			if addr.ID != nil {
				// Get existing address to update only provided fields
				existingAddr, err := s.repo.GetAddressByIDTx(ctx, tx, *addr.ID)
				if err != nil {
					return fmt.Errorf("get address by id: %w", err)
				}

				// Validate that the address belongs to this clinic
				if existingAddr.ClinicID != clinic.ID {
					return fmt.Errorf("address %s does not belong to clinic %s", addr.ID.String(), clinic.ID.String())
				}

				// Update only provided fields
				if addr.Address != nil {
					existingAddr.Address = addr.Address
				}
				if addr.City != nil {
					existingAddr.City = addr.City
				}
				if addr.State != nil {
					existingAddr.State = addr.State
				}
				if addr.Postcode != nil {
					existingAddr.Postcode = addr.Postcode
				}
				if addr.IsPrimary != nil {
					existingAddr.IsPrimary = *addr.IsPrimary
					// If setting as primary, unset other primary addresses for this clinic
					if *addr.IsPrimary {
						if err := s.repo.UnsetPrimaryAddressTx(ctx, tx, clinic.ID, *addr.ID); err != nil {
							return fmt.Errorf("unset primary address: %w", err)
						}
					}
				}

				if err := s.repo.UpdateClinicAddressTx(ctx, tx, existingAddr); err != nil {
					return fmt.Errorf("update address: %w", err)
				}
			}
		}

		// Update contacts
		for _, cont := range req.Contacts {
			if cont.ID != nil {
				// Get existing contact to update only provided fields
				existingContact, err := s.repo.GetContactByIDTx(ctx, tx, *cont.ID)
				if err != nil {
					return fmt.Errorf("get contact by id: %w", err)
				}

				// Validate that the contact belongs to this clinic
				if existingContact.ClinicID != clinic.ID {
					return fmt.Errorf("contact %s does not belong to clinic %s", cont.ID.String(), clinic.ID.String())
				}

				// Update only provided fields
				if cont.ContactType != nil {
					existingContact.ContactType = *cont.ContactType
				}
				if cont.Value != nil {
					existingContact.Value = *cont.Value
				}
				if cont.Label != nil {
					existingContact.Label = cont.Label
				}
				if cont.IsPrimary != nil {
					existingContact.IsPrimary = *cont.IsPrimary
					// If setting as primary, unset other primary contacts for this clinic
					if *cont.IsPrimary {
						if err := s.repo.UnsetPrimaryContactTx(ctx, tx, clinic.ID, *cont.ID); err != nil {
							return fmt.Errorf("unset primary contact: %w", err)
						}
					}
				}

				if err := s.repo.UpdateClinicContactTx(ctx, tx, existingContact); err != nil {
					return fmt.Errorf("update contact: %w", err)
				}
			}
		}

		// Update financial settings if provided
		if req.FinancialYearID != nil || req.LockDate != nil {
			financialSettings, err := s.repo.GetFinancialSettingsTx(ctx, tx, clinic.ID)
			if err != nil {
				return fmt.Errorf("get financial settings: %w", err)
			}

			if financialSettings != nil {
				if req.FinancialYearID != nil {
					financialSettings.FinancialYearID = *req.FinancialYearID
				}
				if req.LockDate != nil {
					financialSettings.LockDate = req.LockDate
				}

				if err := s.repo.UpdateFinancialSettingsTx(ctx, tx, financialSettings); err != nil {
					return fmt.Errorf("update financial settings: %w", err)
				}
			}
		}

		// Get the updated clinic with all related data
		updatedClinic, err := s.getClinicByIDInternalTx(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("get updated clinic: %w", err)
		}

		result = updatedClinic
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("update clinic transaction failed: %w", err)
	}

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

	rsAddresses := make([]RsClinicAddress, 0, len(addresses))
	for _, addr := range addresses {
		rsAddresses = append(rsAddresses, RsClinicAddress{
			ID:        addr.ID,
			Address:   addr.Address,
			City:      addr.City,
			State:     addr.State,
			Postcode:  addr.Postcode,
			IsPrimary: addr.IsPrimary,
		})
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
		Addresses:         rsAddresses,
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

	rsAddresses := make([]RsClinicAddress, 0, len(addresses))
	for _, addr := range addresses {
		rsAddresses = append(rsAddresses, RsClinicAddress{
			ID:        addr.ID,
			Address:   addr.Address,
			City:      addr.City,
			State:     addr.State,
			Postcode:  addr.Postcode,
			IsPrimary: addr.IsPrimary,
		})
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
		Addresses:         rsAddresses,
		Contacts:          rsContacts,
		FinancialSettings: rsFinancialSettings,
		CreatedAt:         clinic.CreatedAt,
		UpdatedAt:         clinic.UpdatedAt,
	}, nil
}

// Helper method to update clinic within a transaction (used by bulk update)
func (s *service) updateClinicInTx(ctx context.Context, tx *sqlx.Tx, practitionerID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error) {
	clinic, err := s.repo.GetClinicByIDAndPractitionerTx(ctx, tx, id, practitionerID)
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

	// Update addresses
	for _, addr := range req.Addresses {
		if addr.ID != nil {
			// Get existing address to update only provided fields
			existingAddr, err := s.repo.GetAddressByIDTx(ctx, tx, *addr.ID)
			if err != nil {
				return nil, fmt.Errorf("get address by id: %w", err)
			}

			// Validate that the address belongs to this clinic
			if existingAddr.ClinicID != clinic.ID {
				return nil, fmt.Errorf("address %s does not belong to clinic %s", addr.ID.String(), clinic.ID.String())
			}

			// Update only provided fields
			if addr.Address != nil {
				existingAddr.Address = addr.Address
			}
			if addr.City != nil {
				existingAddr.City = addr.City
			}
			if addr.State != nil {
				existingAddr.State = addr.State
			}
			if addr.Postcode != nil {
				existingAddr.Postcode = addr.Postcode
			}
			if addr.IsPrimary != nil {
				existingAddr.IsPrimary = *addr.IsPrimary
				// If setting as primary, unset other primary addresses for this clinic
				if *addr.IsPrimary {
					if err := s.repo.UnsetPrimaryAddressTx(ctx, tx, clinic.ID, *addr.ID); err != nil {
						return nil, fmt.Errorf("unset primary address: %w", err)
					}
				}
			}

			if err := s.repo.UpdateClinicAddressTx(ctx, tx, existingAddr); err != nil {
				return nil, fmt.Errorf("update address: %w", err)
			}
		}
	}

	// Update contacts
	for _, cont := range req.Contacts {
		if cont.ID != nil {
			// Get existing contact to update only provided fields
			existingContact, err := s.repo.GetContactByIDTx(ctx, tx, *cont.ID)
			if err != nil {
				return nil, fmt.Errorf("get contact by id: %w", err)
			}

			// Validate that the contact belongs to this clinic
			if existingContact.ClinicID != clinic.ID {
				return nil, fmt.Errorf("contact %s does not belong to clinic %s", cont.ID.String(), clinic.ID.String())
			}

			// Update only provided fields
			if cont.ContactType != nil {
				existingContact.ContactType = *cont.ContactType
			}
			if cont.Value != nil {
				existingContact.Value = *cont.Value
			}
			if cont.Label != nil {
				existingContact.Label = cont.Label
			}
			if cont.IsPrimary != nil {
				existingContact.IsPrimary = *cont.IsPrimary
				// If setting as primary, unset other primary contacts for this clinic
				if *cont.IsPrimary {
					if err := s.repo.UnsetPrimaryContactTx(ctx, tx, clinic.ID, *cont.ID); err != nil {
						return nil, fmt.Errorf("unset primary contact: %w", err)
					}
				}
			}

			if err := s.repo.UpdateClinicContactTx(ctx, tx, existingContact); err != nil {
				return nil, fmt.Errorf("update contact: %w", err)
			}
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

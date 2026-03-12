package clinic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service interface {
	CreateClinic(ctx context.Context, practitionerID uuid.UUID, req *RqCreateClinic) (*RsClinic, error)
	GetClinics(ctx context.Context, practitionerID uuid.UUID) ([]RsClinic, error)
	GetClinicByID(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) (*RsClinic, error)
	UpdateClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID, req *RqUpdateClinic) (*RsClinic, error)
	DeleteClinic(ctx context.Context, practitionerID uuid.UUID, id uuid.UUID) error

	// Internal methods for service-to-service calls (no user validation)
	GetClinicByIDInternal(ctx context.Context, id uuid.UUID) (*RsClinic, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateClinic(ctx context.Context, practitionerID uuid.UUID, req *RqCreateClinic) (*RsClinic, error) {

	// Get active financial year
	activeFinancialYearID, err := s.repo.GetActiveFinancialYear(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active financial year: %w", err)
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

	created, err := s.repo.CreateClinic(ctx, clinic)
	if err != nil {
		return nil, err
	}

	// Create financial settings with active financial year
	financialSettings := &FinancialSettings{
		ClinicID:        created.ID,
		FinancialYearID: *activeFinancialYearID,
		LockDate:        nil, // Keep empty as requested
	}

	createdFS, err := s.repo.CreateFinancialSettings(ctx, financialSettings)
	if err != nil {
		return nil, fmt.Errorf("create financial settings: %w", err)
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

		createdAddr, err := s.repo.CreateClinicAddress(ctx, clinicAddr)
		if err != nil {
			return nil, fmt.Errorf("create address: %w", err)
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

		createdContact, err := s.repo.CreateClinicContact(ctx, clinicContact)
		if err != nil {
			return nil, fmt.Errorf("create contact: %w", err)
		}

		contacts = append(contacts, RsClinicContact{
			ID:          createdContact.ID,
			ContactType: createdContact.ContactType,
			Value:       createdContact.Value,
			Label:       createdContact.Label,
			IsPrimary:   createdContact.IsPrimary,
		})
	}

	return &RsClinic{
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
	}, nil
}

func (s *service) GetClinics(ctx context.Context, practitionerID uuid.UUID) ([]RsClinic, error) {

	clinics, err := s.repo.GetClinicsByPractitioner(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	result := make([]RsClinic, 0, len(clinics))
	for _, clinic := range clinics {
		result = append(result, RsClinic{
			ID:             clinic.ID,
			PractitionerID: clinic.PractitionerID,
			ProfilePicture: clinic.ProfilePicture,
			Name:           clinic.Name,
			ABN:            clinic.ABN,
			Description:    clinic.Description,
			IsActive:       clinic.IsActive,
			CreatedAt:      clinic.CreatedAt,
			UpdatedAt:      clinic.UpdatedAt,
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

	clinic, err := s.repo.GetClinicByIDAndPractitioner(ctx, id, practitionerID)
	if err != nil {
		fmt.Printf("UpdateClinic - Failed to get clinic by ID and practitioner: %v\n", err)
		return nil, err
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

	_, err = s.repo.UpdateClinic(ctx, clinic)
	if err != nil {
		fmt.Printf("UpdateClinic - Failed to update clinic: %v\n", err)
		return nil, err
	}

	fmt.Printf("UpdateClinic - Successfully updated clinic\n")
	return s.GetClinicByID(ctx, practitionerID, id)
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

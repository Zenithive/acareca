package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/jmoiron/sqlx"
)

// =========================================================================
// INTERFACE STRUCTURAL CONTRACT TEST & MOCK SETUP
// =========================================================================

type MockRepository struct {
	clinics     map[uuid.UUID]*Clinic
	addresses   map[uuid.UUID]*ClinicAddress
	contacts    map[uuid.UUID]*ClinicContact
	sessions    map[string]*Session
	tokens      map[uuid.UUID]*VerificationToken
	resetTokens map[string]mockResetToken
	documents   map[uuid.UUID]*file.Document
}

type mockResetToken struct {
	clinicID  string
	tokenHash string
	expiresAt time.Time
	status    string
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		clinics:     make(map[uuid.UUID]*Clinic),
		addresses:   make(map[uuid.UUID]*ClinicAddress),
		contacts:    make(map[uuid.UUID]*ClinicContact),
		sessions:    make(map[string]*Session),
		tokens:      make(map[uuid.UUID]*VerificationToken),
		resetTokens: make(map[string]mockResetToken),
		documents:   make(map[uuid.UUID]*file.Document),
	}
}

var _ Repository = (*MockRepository)(nil)

func (m *MockRepository) CreateClinic(ctx context.Context, clinic *Clinic, tx *sqlx.Tx) (*Clinic, error) {
	if clinic == nil {
		return nil, errors.New("clinic payload cannot be nil")
	}
	clinic.ID = uuid.New()
	now := time.Now().Format(time.RFC3339)
	clinic.CreatedAt = now
	clinic.UpdatedAt = &now
	m.clinics[clinic.ID] = clinic
	return clinic, nil
}

func (m *MockRepository) FindByEmail(ctx context.Context, email string) (*Clinic, error) {
	for _, c := range m.clinics {
		if c.Email == email && c.DeletedAt == nil {
			return c, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *MockRepository) FindByID(ctx context.Context, id uuid.UUID) (*Clinic, error) {
	c, ok := m.clinics[id]
	if !ok || c.DeletedAt != nil {
		return nil, errors.New("user not found")
	}
	return c, nil
}

func (m *MockRepository) UpdateClinic(ctx context.Context, clinic *Clinic, tx *sqlx.Tx) (*Clinic, error) {
	if _, ok := m.clinics[clinic.ID]; !ok {
		return nil, errors.New("user not found")
	}
	now := time.Now().Format(time.RFC3339)
	clinic.UpdatedAt = &now
	m.clinics[clinic.ID] = clinic
	return clinic, nil
}

func (m *MockRepository) DeleteClinic(ctx context.Context, id uuid.UUID) error {
	c, ok := m.clinics[id]
	if !ok || c.DeletedAt != nil {
		return errors.New("user not found")
	}
	now := time.Now().Format(time.RFC3339)
	c.DeletedAt = &now
	return nil
}

func (m *MockRepository) UpdatePassword(ctx context.Context, clinicID uuid.UUID, hashedPassword string) error {
	c, ok := m.clinics[clinicID]
	if !ok || c.DeletedAt != nil {
		return errors.New("user not found")
	}
	c.Password = &hashedPassword
	return nil
}

func (m *MockRepository) CreateAddress(ctx context.Context, addr *ClinicAddress, tx *sqlx.Tx) (*ClinicAddress, error) {
	addr.ID = uuid.New()
	m.addresses[addr.ID] = addr
	return addr, nil
}

func (m *MockRepository) ListAddressesByClinicID(ctx context.Context, clinicID uuid.UUID) ([]ClinicAddress, error) {
	var result []ClinicAddress
	for _, a := range m.addresses {
		if a.ClinicID == clinicID && a.DeletedAt == nil {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (m *MockRepository) UpdateAddress(ctx context.Context, addr *ClinicAddress, tx *sqlx.Tx) (*ClinicAddress, error) {
	m.addresses[addr.ID] = addr
	return addr, nil
}

func (m *MockRepository) DeleteAddressByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error {
	if a, ok := m.addresses[id]; ok {
		now := time.Now().Format(time.RFC3339)
		a.DeletedAt = &now
	}
	return nil
}

func (m *MockRepository) CountActiveAddresses(ctx context.Context, clinicID uuid.UUID, tx *sqlx.Tx) (int, error) {
	count := 0
	for _, a := range m.addresses {
		if a.ClinicID == clinicID && a.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (m *MockRepository) CreateContact(ctx context.Context, contact *ClinicContact, tx *sqlx.Tx) (*ClinicContact, error) {
	contact.ID = uuid.New()
	m.contacts[contact.ID] = contact
	return contact, nil
}

func (m *MockRepository) ListContactsByClinicID(ctx context.Context, clinicID uuid.UUID) ([]ClinicContact, error) {
	var result []ClinicContact
	for _, c := range m.contacts {
		if c.ClinicID == clinicID && c.DeletedAt == nil {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *MockRepository) UpdateContact(ctx context.Context, contact *ClinicContact, tx *sqlx.Tx) (*ClinicContact, error) {
	m.contacts[contact.ID] = contact
	return contact, nil
}

func (m *MockRepository) DeleteContactByID(ctx context.Context, id uuid.UUID, tx *sqlx.Tx) error {
	if c, ok := m.contacts[id]; ok {
		now := time.Now().Format(time.RFC3339)
		c.DeletedAt = &now
	}
	return nil
}

func (m *MockRepository) CountActiveContacts(ctx context.Context, clinicID uuid.UUID, tx *sqlx.Tx) (int, error) {
	count := 0
	for _, c := range m.contacts {
		if c.ClinicID == clinicID && c.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (m *MockRepository) CreateSession(ctx context.Context, s *Session) (*Session, error) {
	m.sessions[s.RefreshToken] = s
	return s, nil
}

func (m *MockRepository) FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	s, ok := m.sessions[refreshToken]
	if !ok || s.DeletedAt != nil {
		return nil, errors.New("user not found")
	}
	return s, nil
}

func (m *MockRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	for _, s := range m.sessions {
		if s.ID == id {
			now := time.Now()
			s.DeletedAt = &now
		}
	}
	return nil
}

func (m *MockRepository) CreateVerificationToken(ctx context.Context, token *VerificationToken, tx *sqlx.Tx) error {
	m.tokens[token.ID] = token
	return nil
}

func (m *MockRepository) DeactivateOldTokens(ctx context.Context, clinicID uuid.UUID) error {
	for _, t := range m.tokens {
		if t.ClinicID == clinicID && t.Status == TokenStatusPending {
			t.Status = TokenStatusResent
		}
	}
	return nil
}

func (m *MockRepository) GetToken(ctx context.Context, tokenID uuid.UUID) (*VerificationToken, error) {
	t, ok := m.tokens[tokenID]
	if !ok {
		return nil, errors.New("token not found")
	}
	return t, nil
}

func (m *MockRepository) MarkUserVerified(ctx context.Context, token *VerificationToken) error {
	c, ok := m.clinics[token.ClinicID]
	if !ok {
		return errors.New("clinic not found")
	}
	c.Verified = true
	if t, ok := m.tokens[token.ID]; ok {
		t.Status = TokenStatusUsed
	}
	return nil
}

func (m *MockRepository) SaveResetToken(ctx context.Context, clinicID string, tokenHash string, expiresAt time.Time) error {
	m.resetTokens[tokenHash] = mockResetToken{
		clinicID:  clinicID,
		tokenHash: tokenHash,
		expiresAt: expiresAt,
		status:    TokenStatusPending,
	}
	return nil
}

func (m *MockRepository) CompletePasswordReset(ctx context.Context, tokenHash string, newPasswordHash string) error {
	t, ok := m.resetTokens[tokenHash]
	if !ok || t.status != TokenStatusPending {
		return errors.New("invalid or expired reset link")
	}
	uid, _ := uuid.Parse(t.clinicID)
	if c, ok := m.clinics[uid]; ok {
		c.Password = &newPasswordHash
		t.status = TokenStatusUsed
		m.resetTokens[tokenHash] = t
		return nil
	}
	return errors.New("invalid or expired reset link")
}

func (m *MockRepository) GetDocumentByID(ctx context.Context, documentID string) (*file.Document, error) {
	uid, _ := uuid.Parse(documentID)
	if doc, ok := m.documents[uid]; ok {
		return doc, nil
	}
	return nil, nil
}

// =========================================================================
// INDIVIDUAL CLINIC CRUD FUNCTIONS
// =========================================================================

func TestRepository_CreateClinic(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	pass := "hashed_secret"

	clinic := &Clinic{
		ClinicName: "Aura Dental Care",
		Email:      "hello@auradental.com",
		Password:   &pass,
	}

	created, err := repo.CreateClinic(ctx, clinic, nil)
	if err != nil {
		t.Fatalf("CreateClinic failed: %v", err)
	}

	if created.ID == uuid.Nil {
		t.Fatal("Expected unique UUID generated for clinic")
	}
	if created.CreatedAt == "" {
		t.Fatal("Expected CreatedAt timestamp assignment")
	}
}

func TestRepository_FindClinic(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	pass := "secret"

	seeded, _ := repo.CreateClinic(ctx, &Clinic{
		ClinicName: "Metro General",
		Email:      "metro@general.com",
		Password:   &pass,
	}, nil)

	t.Run("FindByID", func(t *testing.T) {
		fetched, err := repo.FindByID(ctx, seeded.ID)
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		if fetched.ClinicName != "Metro General" {
			t.Errorf("Expected 'Metro General', got '%s'", fetched.ClinicName)
		}
	})

	t.Run("FindByEmail", func(t *testing.T) {
		fetched, err := repo.FindByEmail(ctx, "metro@general.com")
		if err != nil {
			t.Fatalf("FindByEmail failed: %v", err)
		}
		if fetched.ID != seeded.ID {
			t.Error("Identity mismatch on email lookups")
		}
	})
}

func TestRepository_UpdateClinic(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	pass := "secret"

	seeded, _ := repo.CreateClinic(ctx, &Clinic{
		ClinicName: "City Health",
		Email:      "city@health.com",
		Password:   &pass,
	}, nil)

	seeded.ClinicName = "City Health Group"
	updated, err := repo.UpdateClinic(ctx, seeded, nil)
	if err != nil {
		t.Fatalf("UpdateClinic operational failure: %v", err)
	}

	if updated.ClinicName != "City Health Group" {
		t.Errorf("Expected updated mutation profile layout name change to sync")
	}
}

func TestRepository_UpdatePassword(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	pass := "old_pass"

	seeded, _ := repo.CreateClinic(ctx, &Clinic{
		Email:    "pass@change.com",
		Password: &pass,
	}, nil)

	err := repo.UpdatePassword(ctx, seeded.ID, "brand_new_hash")
	if err != nil {
		t.Fatalf("UpdatePassword operational failure: %v", err)
	}

	if *repo.clinics[seeded.ID].Password != "brand_new_hash" {
		t.Fatal("Expected clinic mapping value to accurately pivot values")
	}
}

func TestRepository_DeleteClinic(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	pass := "secret"

	seeded, _ := repo.CreateClinic(ctx, &Clinic{
		ClinicName: "Temp Clinic",
		Email:      "temp@clinic.com",
		Password:   &pass,
	}, nil)

	err := repo.DeleteClinic(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("DeleteClinic execution failure: %v", err)
	}

	_, err = repo.FindByID(ctx, seeded.ID)
	if err == nil {
		t.Fatal("Expected 'user not found' error sequence checking soft-deleted datasets")
	}
}

// =========================================================================
// INDIVIDUAL ADDRESS & CONTACT ACTIONS
// =========================================================================

func TestRepository_AddressLifecycle(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	addr := &ClinicAddress{
		ClinicID:  clinicID,
		Address:   "100 Collins St",
		City:      "Melbourne",
		IsPrimary: true,
	}

	// Create
	created, err := repo.CreateAddress(ctx, addr, nil)
	if err != nil {
		t.Fatalf("CreateAddress failed: %v", err)
	}

	// Count & List
	count, _ := repo.CountActiveAddresses(ctx, clinicID, nil)
	list, _ := repo.ListAddressesByClinicID(ctx, clinicID)
	if count != 1 || len(list) != 1 {
		t.Fatal("Address listing or count mechanics unbalanced")
	}

	// Update
	created.Address = "200 Bourke St"
	_, err = repo.UpdateAddress(ctx, created, nil)
	if err != nil || repo.addresses[created.ID].Address != "200 Bourke St" {
		t.Fatal("UpdateAddress did not reflect properly in mock repository store")
	}

	// Delete
	err = repo.DeleteAddressByID(ctx, created.ID, nil)
	if err != nil || repo.addresses[created.ID].DeletedAt == nil {
		t.Fatal("Address elimination sequencing broken")
	}
}

func TestRepository_ContactLifecycle(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	contact := &ClinicContact{
		ClinicID:    clinicID,
		ContactType: "EMAIL",
		Value:       "reception@test.com",
	}

	// Create
	created, err := repo.CreateContact(ctx, contact, nil)
	if err != nil {
		t.Fatalf("CreateContact failed: %v", err)
	}

	// Count & List
	count, _ := repo.CountActiveContacts(ctx, clinicID, nil)
	list, _ := repo.ListContactsByClinicID(ctx, clinicID)
	if count != 1 || len(list) != 1 {
		t.Fatal("Contact dataset calculations unbalanced")
	}

	// Update
	created.Value = "reception-new@test.com"
	_, err = repo.UpdateContact(ctx, created, nil)
	if err != nil || repo.contacts[created.ID].Value != "reception-new@test.com" {
		t.Fatal("UpdateContact mutation matching failure")
	}

	// Delete
	err = repo.DeleteContactByID(ctx, created.ID, nil)
	if err != nil || repo.contacts[created.ID].DeletedAt == nil {
		t.Fatal("Delete contact operation path failure")
	}
}

// =========================================================================
// INDIVIDUAL SESSIONS, TOKENS, AND PASSWORD RESET ACTIONS
// =========================================================================

func TestRepository_SessionOperations(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	sess := &Session{
		ID:           uuid.New(),
		ClinicID:     clinicID,
		RefreshToken: "sample_token_string",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}

	// Create
	created, err := repo.CreateSession(ctx, sess)
	if err != nil {
		t.Fatalf("CreateSession execution failed: %v", err)
	}

	// Find
	fetched, err := repo.FindSessionByRefreshToken(ctx, "sample_token_string")
	if err != nil || fetched.ID != created.ID {
		t.Fatal("Token registration parsing failed structural mapping")
	}

	// Delete
	err = repo.DeleteSession(ctx, created.ID)
	if err != nil || repo.sessions["sample_token_string"].DeletedAt == nil {
		t.Fatal("Session logging validation rules broken")
	}
}

func TestRepository_VerificationTokens(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()
	role := "CLINIC"
	pass := "secret"

	_, _ = repo.CreateClinic(ctx, &Clinic{ID: clinicID, Email: "test@verify.com", Password: &pass}, nil)

	token := &VerificationToken{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		Role:      &role,
		Status:    TokenStatusPending,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// Create & Get
	_ = repo.CreateVerificationToken(ctx, token, nil)
	fetched, err := repo.GetToken(ctx, token.ID)
	if err != nil || fetched.Status != TokenStatusPending {
		t.Fatal("Token handling lifecycle broken context extraction layout")
	}

	// Deactivate
	err = repo.DeactivateOldTokens(ctx, clinicID)
	if err != nil || repo.tokens[token.ID].Status != TokenStatusResent {
		t.Fatal("Pivoting registration codes to active RESENT state broke execution loops")
	}

	// Mark User Verified
	repo.tokens[token.ID].Status = TokenStatusPending
	err = repo.MarkUserVerified(ctx, token)
	if err != nil || !repo.clinics[clinicID].Verified || repo.tokens[token.ID].Status != TokenStatusUsed {
		t.Fatal("Identity verification sequence execution parameters failed updates confirmation metrics")
	}
}

func TestRepository_PasswordResetTokens(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()
	pass := "old_hash"

	_, _ = repo.CreateClinic(ctx, &Clinic{ID: clinicID, Email: "reset@flow.com", Password: &pass}, nil)
	tokenHash := "sha256_mock_hash_sequence"

	// Save
	err := repo.SaveResetToken(ctx, clinicID.String(), tokenHash, time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatalf("SaveResetToken failure: %v", err)
	}

	// Complete
	err = repo.CompletePasswordReset(ctx, tokenHash, "super_secure_new_password_hash")
	if err != nil {
		t.Fatalf("CompletePasswordReset action flow failure: %v", err)
	}

	if *repo.clinics[clinicID].Password != "super_secure_new_password_hash" {
		t.Fatal("Target property values failed storage updates validation chains")
	}
}

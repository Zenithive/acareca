package contact

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
)

// =========================================================================
// INTERFACE STRUCTURAL CONTRACT TEST & MOCK SETUP
// =========================================================================

type MockRepository struct {
	contacts  map[uuid.UUID]*Contact
	addresses map[uuid.UUID]*Address
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		contacts:  make(map[uuid.UUID]*Contact),
		addresses: make(map[uuid.UUID]*Address),
	}
}

var _ Repository = (*MockRepository)(nil)

func (m *MockRepository) Create(ctx context.Context, contact Contact) (Contact, error) {
	if contact.ID == uuid.Nil {
		contact.ID = uuid.New()
	}

	now := time.Now()
	contact.CreatedAt = now
	contact.UpdatedAt = now

	for _, addr := range contact.Address {
		if addr.Id == uuid.Nil {
			addr.Id = uuid.New()
		}
		addr.ContactID = contact.ID
		addr.CreatedAt = now
		addr.UpdatedAt = now
		m.addresses[addr.Id] = addr
	}

	m.contacts[contact.ID] = &contact
	return contact, nil
}

func (m *MockRepository) Update(ctx context.Context, contact Contact) error {
	existing, ok := m.contacts[contact.ID]
	if !ok || existing.DeletedAt != nil {
		return errors.New("contact not found")
	}

	now := time.Now()
	contact.CreatedAt = existing.CreatedAt
	contact.UpdatedAt = now

	updatedIDs := make(map[uuid.UUID]bool)
	for _, addr := range contact.Address {
		if addr.Id == uuid.Nil {
			addr.Id = uuid.New()
		}
		updatedIDs[addr.Id] = true
	}

	// Soft-delete omitted addresses
	for _, addr := range m.addresses {
		if addr.ContactID == contact.ID && addr.DeletedAt == nil {
			if !updatedIDs[addr.Id] {
				addr.DeletedAt = &now
				addr.UpdatedAt = now
			}
		}
	}

	// Clear primary flag across sibling addresses before updates
	for _, addr := range m.addresses {
		if addr.ContactID == contact.ID && addr.DeletedAt == nil {
			addr.IsPrimary = false
			addr.UpdatedAt = now
		}
	}

	// Upsert addresses
	for _, addr := range contact.Address {
		addr.ContactID = contact.ID
		addr.UpdatedAt = now
		if _, exists := m.addresses[addr.Id]; !exists {
			addr.CreatedAt = now
		}
		m.addresses[addr.Id] = addr
	}

	m.contacts[contact.ID] = &contact
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, id uuid.UUID) error {
	contact, ok := m.contacts[id]
	if !ok || contact.DeletedAt != nil {
		return errors.New("contact not found")
	}

	now := time.Now()
	contact.DeletedAt = &now
	contact.UpdatedAt = now

	for _, addr := range m.addresses {
		if addr.ContactID == id && addr.DeletedAt == nil {
			addr.DeletedAt = &now
			addr.UpdatedAt = now
		}
	}

	return nil
}

func (m *MockRepository) Get(ctx context.Context, id uuid.UUID) (Contact, error) {
	contact, ok := m.contacts[id]
	if !ok || contact.DeletedAt != nil {
		return Contact{}, errors.New("contact not found")
	}

	result := *contact
	result.Address = []*Address{}

	for _, addr := range m.addresses {
		if addr.ContactID == id && addr.DeletedAt == nil {
			result.Address = append(result.Address, addr)
		}
	}

	return result, nil
}

func (m *MockRepository) List(ctx context.Context, clinicID uuid.UUID, f common.Filter) ([]Contact, int64, error) {
	var list []Contact
	var total int64

	for _, contact := range m.contacts {
		if contact.ClinicId == clinicID && contact.DeletedAt == nil {
			total++

			clonedContact := *contact
			clonedContact.Address = []*Address{}
			for _, addr := range m.addresses {
				if addr.ContactID == contact.ID && addr.DeletedAt == nil {
					clonedContact.Address = append(clonedContact.Address, addr)
				}
			}

			list = append(list, clonedContact)
		}
	}

	return list, total, nil
}

func (m *MockRepository) DeleteAddressByID(ctx context.Context, id uuid.UUID) error {
	addr, ok := m.addresses[id]
	if !ok || addr.DeletedAt != nil {
		return errors.New("address not found")
	}

	now := time.Now()
	addr.DeletedAt = &now
	addr.UpdatedAt = now
	return nil
}

// =========================================================================
// INDIVIDUAL CRUD & SUB-RESOURCE OPERATIONAL TESTS
// =========================================================================

func TestRepository_Create(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()
	addr2 := "Suite 4B"

	contactPayload := Contact{
		ClinicId: clinicID,
		Fname:    "Alice",
		Lname:    "Smith",
		Phone:    "+61411111111",
		Email:    "alice.smith@clinic.com",
		Address: []*Address{
			{
				AddressLine1: "100 Collins St",
				AddressLine2: &addr2,
				City:         "Melbourne",
				State:        "VIC",
				PostalCode:   "3000",
				Country:      "Australia",
				IsPrimary:    true,
			},
		},
	}

	created, err := repo.Create(ctx, contactPayload)
	if err != nil {
		t.Fatalf("Create operation failed unexpectedly: %v", err)
	}

	if created.ID == uuid.Nil {
		t.Fatal("Expected unique primary identity UUID to be auto-generated")
	}

	if len(created.Address) != 1 || created.Address[0].Id == uuid.Nil {
		t.Fatal("Expected address child structure initialization with valid ID assignment")
	}

	if created.Address[0].ContactID != created.ID {
		t.Fatal("Foreign key reference constraint broken on child sub-resource records")
	}
}

func TestRepository_Get(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	// Seed data
	seeded, _ := repo.Create(ctx, Contact{
		ClinicId: clinicID,
		Fname:    "Bob",
		Lname:    "Jones",
		Email:    "bob.jones@clinic.com",
	})

	t.Run("Successfully get active contact", func(t *testing.T) {
		fetched, err := repo.Get(ctx, seeded.ID)
		if err != nil {
			t.Fatalf("Get returned unexpected operational failure: %v", err)
		}
		if fetched.Fname != "Bob" {
			t.Fatalf("Data distortion: expected 'Bob', retrieved '%s'", fetched.Fname)
		}
	})

	t.Run("Return error on non-existent contact", func(t *testing.T) {
		_, err := repo.Get(ctx, uuid.New())
		if err == nil {
			t.Fatal("Expected standard missing resource exception, got nil error")
		}
	})
}

func TestRepository_Update(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	// Seed data with one address resource
	seeded, _ := repo.Create(ctx, Contact{
		ClinicId: clinicID,
		Fname:    "Charlie",
		Lname:    "Brown",
		Address: []*Address{
			{AddressLine1: "Old Lane Road", IsPrimary: true},
		},
	})

	// Retrieve profile state for update adjustments
	fetched, _ := repo.Get(ctx, seeded.ID)
	fetched.Fname = "Charles"

	// Append a second address and declare it primary
	fetched.Address = append(fetched.Address, &Address{
		AddressLine1: "New Boulevard Highway",
		IsPrimary:    true,
	})

	err := repo.Update(ctx, fetched)
	if err != nil {
		t.Fatalf("Update mutation failed execution: %v", err)
	}

	// Validate changes synced correctly
	updated, _ := repo.Get(ctx, seeded.ID)
	if updated.Fname != "Charles" {
		t.Fatalf("Expected core field update modification to match, got '%s'", updated.Fname)
	}

	if len(updated.Address) != 2 {
		t.Fatalf("Expected address sub-relation sizing to match 2, found %d", len(updated.Address))
	}

	// Verify that the original address IsPrimary was pivoted to false
	for _, addr := range updated.Address {
		if addr.AddressLine1 == "Old Lane Road" && addr.IsPrimary {
			t.Fatal("Synchronization strategy failed to balance singular Primary configurations")
		}
	}
}

func TestRepository_Delete(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	seeded, _ := repo.Create(ctx, Contact{
		ClinicId: clinicID,
		Fname:    "David",
		Lname:    "Miller",
		Address: []*Address{
			{AddressLine1: "Delete Testing Block Address"},
		},
	})

	err := repo.Delete(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("Delete workflow operation execution threw failure: %v", err)
	}

	// Read checks should fail immediately on a soft-deleted profile lookup
	_, err = repo.Get(ctx, seeded.ID)
	if err == nil {
		t.Fatal("Soft deleted contact profiles must be hidden from normal Get requests")
	}

	// Cascading soft-delete must flag all underlying sub-resource address relations
	for _, addr := range repo.addresses {
		if addr.ContactID == seeded.ID && addr.DeletedAt == nil {
			t.Fatal("Cascade validation asset failed: sub-resource relations remained unflagged")
		}
	}
}

func TestRepository_List(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	// Seed matching datasets
	_, _ = repo.Create(ctx, Contact{ClinicId: clinicID, Fname: "Contact One"})
	_, _ = repo.Create(ctx, Contact{ClinicId: clinicID, Fname: "Contact Two"})
	// Seed non-matching clinic entity block
	_, _ = repo.Create(ctx, Contact{ClinicId: uuid.New(), Fname: "Stranger Contact"})

	list, total, err := repo.List(ctx, clinicID, common.Filter{})
	if err != nil {
		t.Fatalf("List evaluation error: %v", err)
	}

	if total != 2 || len(list) != 2 {
		t.Fatalf("Isolation constraint breach: expected listing length of 2, found %d", total)
	}
}

func TestRepository_DeleteAddressByID(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	seeded, _ := repo.Create(ctx, Contact{
		ClinicId: uuid.New(),
		Fname:    "Eva",
		Address: []*Address{
			{AddressLine1: "Target Deletable Location Row"},
		},
	})

	targetAddressID := seeded.Address[0].Id

	err := repo.DeleteAddressByID(ctx, targetAddressID)
	if err != nil {
		t.Fatalf("Explicit sub-resource target eviction function failed: %v", err)
	}

	if repo.addresses[targetAddressID].DeletedAt == nil {
		t.Fatal("Expected operational timestamp signature mutation on targeted item index")
	}

	// Ensure secondary attempt correctly failures when evaluating an inactive sub-resource
	err = repo.DeleteAddressByID(ctx, targetAddressID)
	if err == nil {
		t.Fatal("Subsequent address elimination requests against missing references should throw an error")
	}
}

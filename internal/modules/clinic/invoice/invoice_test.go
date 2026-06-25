package invoice

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/invoice/section"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/jmoiron/sqlx"
)

// =========================================================================
// INTERFACE STRUCTURAL CONTRACT TEST & MOCK SETUP
// =========================================================================

type MockInvoiceRepository struct {
	invoices      map[uuid.UUID]*Invoice
	mailTemplates map[uuid.UUID]mockMailTemplate
	sequences     map[string]int
}

type mockMailTemplate struct {
	subject string
	body    string
}

func NewMockInvoiceRepository() *MockInvoiceRepository {
	return &MockInvoiceRepository{
		invoices:      make(map[uuid.UUID]*Invoice),
		mailTemplates: make(map[uuid.UUID]mockMailTemplate),
		sequences:     make(map[string]int),
	}
}

// Ensure MockInvoiceRepository strictly complies with IRepository interface defined in repository.go
var _ IRepository = (*MockInvoiceRepository)(nil)

func (m *MockInvoiceRepository) Create(ctx context.Context, invoice *Invoice) error {
	if invoice == nil {
		return errors.New("invoice payload cannot be nil")
	}
	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
	}
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m *MockInvoiceRepository) Update(ctx context.Context, invoice *Invoice) error {
	if _, ok := m.invoices[invoice.ID]; !ok {
		return ErrNotFound
	}
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m *MockInvoiceRepository) UpdateWithSections(ctx context.Context, invoice *Invoice, sections []section.Section, deleteSectionIDs []uuid.UUID, deleteItemIDs map[uuid.UUID][]uuid.UUID) error {
	if _, ok := m.invoices[invoice.ID]; !ok {
		return ErrNotFound
	}
	invoice.Sections = sections
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m *MockInvoiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.invoices[id]; !ok {
		return ErrNotFound
	}
	delete(m.invoices, id)
	return nil
}

func (m *MockInvoiceRepository) List(ctx context.Context, filter common.Filter) ([]*Invoice, int64, error) {
	var result []*Invoice
	for _, inv := range m.invoices {
		result = append(result, inv)
	}
	return result, int64(len(result)), nil
}

func (m *MockInvoiceRepository) GetByID(ctx context.Context, db sqlx.QueryerContext, id uuid.UUID) (*Invoice, error) {
	inv, ok := m.invoices[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return inv, nil
}

func (m *MockInvoiceRepository) GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
	tmpl, ok := m.mailTemplates[clinicID]
	if !ok {
		return "", "", errors.New("template not found")
	}
	return tmpl.subject, tmpl.body, nil
}

func (m *MockInvoiceRepository) SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error {
	m.mailTemplates[clinicID] = mockMailTemplate{
		subject: subject,
		body:    body,
	}
	return nil
}

func (m *MockInvoiceRepository) GetNextSequenceForYear(ctx context.Context, prefix string, year string) (string, error) {
	key := prefix + "-" + year
	m.sequences[key]++
	return string(rune(m.sequences[key])), nil
}

// =========================================================================
// INDIVIDUAL INVOICE CRUD & LIFECYCLE TESTS
// =========================================================================

func TestRepository_CreateInvoice(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()
	clinicID := uuid.New()
	contactID := uuid.New()

	inv := &Invoice{
		ClinicID:  clinicID,
		ContactID: &contactID,
		Name:      "Invoice Jan 2026",
		IssueDate: time.Now(),
	}

	err := repo.Create(ctx, inv)
	if err != nil {
		t.Fatalf("Create Invoice failed: %v", err)
	}

	if inv.ID == uuid.Nil {
		t.Fatal("Expected unique UUID generated for invoice record")
	}
	if repo.invoices[inv.ID].Name != "Invoice Jan 2026" {
		t.Errorf("Expected name to match, got %v", repo.invoices[inv.ID].Name)
	}
}

func TestRepository_UpdateInvoice(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()
	id := uuid.New()

	repo.invoices[id] = &Invoice{
		ID:   id,
		Name: "Original Invoice",
	}

	updatedInv := &Invoice{
		ID:   id,
		Name: "Updated Invoice Name",
	}

	err := repo.Update(ctx, updatedInv)
	if err != nil {
		t.Fatalf("Update operational failure: %v", err)
	}

	if repo.invoices[id].Name != "Updated Invoice Name" {
		t.Errorf("Expected structural profile changes to be updated, got %v", repo.invoices[id].Name)
	}
}

func TestRepository_UpdateWithSections(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()
	id := uuid.New()

	repo.invoices[id] = &Invoice{
		ID:   id,
		Name: "Sectioned Invoice",
	}

	sections := []section.Section{
		{ID: uuid.New(), InvoiceSection: "Consultation Services"},
	}

	err := repo.UpdateWithSections(ctx, repo.invoices[id], sections, nil, nil)
	if err != nil {
		t.Fatalf("UpdateWithSections failure: %v", err)
	}

	if len(repo.invoices[id].Sections) != 1 {
		t.Fatal("Expected sections list dataset assignment adjustments to map correctly")
	}
}

func TestRepository_GetInvoiceByID(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()
	id := uuid.New()

	repo.invoices[id] = &Invoice{
		ID:   id,
		Name: "Target Retrieval Invoice",
	}

	fetched, err := repo.GetByID(ctx, nil, id)
	if err != nil {
		t.Fatalf("GetByID operation failed: %v", err)
	}

	if fetched.Name != "Target Retrieval Invoice" {
		t.Errorf("Mismatch on data content recovery pipelines")
	}
}

func TestRepository_ListInvoices(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()

	id1 := uuid.New()
	id2 := uuid.New()
	repo.invoices[id1] = &Invoice{ID: id1, Name: "Inv 1"}
	repo.invoices[id2] = &Invoice{ID: id2, Name: "Inv 2"}

	list, count, err := repo.List(ctx, common.Filter{})
	if err != nil {
		t.Fatalf("List operational execution failure: %v", err)
	}

	if count != 2 || len(list) != 2 {
		t.Errorf("Unbalanced target dataset calculations inside list engine")
	}
}

func TestRepository_DeleteInvoice(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()
	id := uuid.New()

	repo.invoices[id] = &Invoice{
		ID:   id,
		Name: "To Delete",
	}

	err := repo.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete processing failure: %v", err)
	}

	if _, ok := repo.invoices[id]; ok {
		t.Fatal("Expected database storage context instance removal")
	}
}

func TestRepository_ClinicMailTemplateLifecycle(t *testing.T) {
	repo := NewMockInvoiceRepository()
	ctx := context.Background()
	clinicID := uuid.New()

	subject := "Your Invoice Update"
	body := "Please find attached your updated document details."

	// Save
	err := repo.SaveClinicMailTemplate(ctx, clinicID, subject, body)
	if err != nil {
		t.Fatalf("SaveClinicMailTemplate operational tracking failure: %v", err)
	}

	// Retrieve
	sub, bd, err := repo.GetSavedClinicMailTemplate(ctx, clinicID)
	if err != nil || sub != subject || bd != body {
		t.Fatal("Saved mail blueprint template validation chains broken")
	}
}

package template

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/util"
)

// =========================================================================
// REFACTORED MOCK IMPLEMENTATION
// =========================================================================

type MockTemplateRepository struct {
	templates     map[uuid.UUID]*Template
	settings      map[uuid.UUID]*Setting
	mappings      map[uuid.UUID]*Mapping
	documents     map[uuid.UUID]*file.Document
	invoices      map[uuid.UUID]*InvoiceResponse
	sections      map[uuid.UUID][]InvoiceSectionMeta
	mailTemplates map[uuid.UUID]struct{ subject, body string }
}

func NewMockTemplateRepository() *MockTemplateRepository {
	return &MockTemplateRepository{
		templates:     make(map[uuid.UUID]*Template),
		settings:      make(map[uuid.UUID]*Setting),
		mappings:      make(map[uuid.UUID]*Mapping),
		documents:     make(map[uuid.UUID]*file.Document),
		invoices:      make(map[uuid.UUID]*InvoiceResponse),
		sections:      make(map[uuid.UUID][]InvoiceSectionMeta),
		mailTemplates: make(map[uuid.UUID]struct{ subject, body string }),
	}
}

// Ensure compile-time interface adherence matching your latest IRepository
var _ IRepository = (*MockTemplateRepository)(nil)

func (m *MockTemplateRepository) Create(ctx context.Context, t *Template) error {
	if t.Id == uuid.Nil {
		t.Id = uuid.New()
	}
	t.CreatedAt = time.Now()
	m.templates[t.Id] = t
	return nil
}

func (m *MockTemplateRepository) BulkCreate(ctx context.Context, ts []Template) error {
	for i := range ts {
		if ts[i].Id == uuid.Nil {
			ts[i].Id = uuid.New()
		}
		ts[i].CreatedAt = time.Now()
		m.templates[ts[i].Id] = &ts[i]
	}
	return nil
}

func (m *MockTemplateRepository) Update(ctx context.Context, t *Template) error {
	if _, ok := m.templates[t.Id]; !ok {
		return ErrNotFound
	}
	now := time.Now()
	t.UpdatedAt = &now
	m.templates[t.Id] = t
	return nil
}

func (m *MockTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.templates[id]; !ok {
		return ErrNotFound
	}
	delete(m.templates, id)
	return nil
}

func (m *MockTemplateRepository) Get(ctx context.Context, id uuid.UUID) (*Template, error) {
	t, ok := m.templates[id]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (m *MockTemplateRepository) List(ctx context.Context, types []string) (*util.RsList, error) {
	var list []interface{}
	for _, t := range m.templates {
		if len(types) == 0 {
			list = append(list, t.ToRs())
			continue
		}
		for _, ty := range types {
			if t.Name == ty {
				list = append(list, t.ToRs())
				break
			}
		}
	}
	return &util.RsList{Items: list, Total: len(list)}, nil
}

func (m *MockTemplateRepository) GetSetting(ctx context.Context, templateId uuid.UUID) (*Setting, error) {
	st, ok := m.settings[templateId]
	if !ok {
		return nil, nil
	}
	return st, nil
}

func (m *MockTemplateRepository) UpdateSetting(ctx context.Context, st *Setting, templateId uuid.UUID) error {
	st.TemplateId = templateId
	m.settings[templateId] = st
	return nil
}

func (m *MockTemplateRepository) CreateSetting(ctx context.Context, st *Setting) error {
	if st.Id == uuid.Nil {
		st.Id = uuid.New()
	}
	m.settings[st.TemplateId] = st
	return nil
}

func (m *MockTemplateRepository) CreateMapping(ctx context.Context, mp *Mapping) error {
	if mp.ID == uuid.Nil {
		mp.ID = uuid.New()
	}
	m.mappings[mp.ID] = mp
	return nil
}

func (m *MockTemplateRepository) UpdateMapping(ctx context.Context, mp *Mapping) error {
	if _, ok := m.mappings[mp.ID]; !ok {
		return fmt.Errorf("mapping not found for update: %s", mp.ID)
	}
	m.mappings[mp.ID] = mp
	return nil
}

func (m *MockTemplateRepository) GetDocumentByID(ctx context.Context, id uuid.UUID) (*file.Document, error) {
	doc, ok := m.documents[id]
	if !ok {
		return nil, nil
	}
	return doc, nil
}

func (m *MockTemplateRepository) GetInvoice(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID) (*InvoiceResponse, error) {
	inv, ok := m.invoices[invoiceId]
	if !ok || inv.ClinicID != clinicId {
		return nil, ErrInvoiceNotFound
	}
	return inv, nil
}

func (m *MockTemplateRepository) GetInvoiceSectionMeta(ctx context.Context, invoiceId uuid.UUID) ([]InvoiceSectionMeta, error) {
	return m.sections[invoiceId], nil
}

func (m *MockTemplateRepository) GetSavedClinicMailTemplate(ctx context.Context, clinicID uuid.UUID) (string, string, error) {
	tmpl, ok := m.mailTemplates[clinicID]
	if !ok {
		return "", "", fmt.Errorf("no mail template")
	}
	return tmpl.subject, tmpl.body, nil
}

func (m *MockTemplateRepository) SaveClinicMailTemplate(ctx context.Context, clinicID uuid.UUID, subject, body string) error {
	m.mailTemplates[clinicID] = struct{ subject, body string }{subject: subject, body: body}
	return nil
}

func (m *MockTemplateRepository) GetInvoiceSetting(ctx context.Context, clinicId uuid.UUID, invoiceId uuid.UUID, templateIds []uuid.UUID) (*Setting, error) {
	if len(templateIds) == 0 {
		return nil, fmt.Errorf("at least one template ID is required")
	}
	// Fallback simulation matching fallback order precedence logic inside Repository
	for _, tid := range templateIds {
		if st, ok := m.settings[tid]; ok {
			return st, nil
		}
	}
	return nil, nil
}

func (m *MockTemplateRepository) ValidateTemplateAccess(ctx context.Context, templateIds []uuid.UUID) error {
	if len(templateIds) > 10 {
		return fmt.Errorf("too many template IDs provided, maximum is 10")
	}
	for _, tid := range templateIds {
		if _, ok := m.templates[tid]; !ok {
			return ErrUnauthorized
		}
	}
	return nil
}

// =========================================================================
// COMPREHENSIVE TEST CASES
// =========================================================================

func TestRepository_CreateAndGetTemplate(t *testing.T) {
	repo := NewMockTemplateRepository()
	ctx := context.Background()

	tmpl := &Template{
		Id:       uuid.New(),
		Name:     "Standard Healthcare Blueprint",
		IsActive: true,
		Html:     []byte("<html></html>"),
		Css:      []byte("body {}"),
	}

	err := repo.Create(ctx, tmpl)
	if err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	fetched, err := repo.Get(ctx, tmpl.Id)
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if fetched.Name != tmpl.Name {
		t.Errorf("Expected template name %s, got %s", tmpl.Name, fetched.Name)
	}
}

func TestRepository_TemplateSettingsLifecycle(t *testing.T) {
	repo := NewMockTemplateRepository()
	ctx := context.Background()
	templateID := uuid.New()

	st := &Setting{
		Id:           uuid.New(),
		TemplateId:   templateID,
		PrimaryColor: "#1F4E5F",
		AccentColor:  "#E5E7EB",
		IsLogo:       true,
		CreatedAt:    time.Now(),
	}

	err := repo.CreateSetting(ctx, st)
	if err != nil {
		t.Fatalf("Failed to register template settings: %v", err)
	}

	fetched, err := repo.GetSetting(ctx, templateID)
	if err != nil {
		t.Fatalf("Failed to fetch configuration settings: %v", err)
	}

	if fetched.PrimaryColor != "#1F4E5F" {
		t.Errorf("Color tracking mismatch: expected #1F4E5F, got %s", fetched.PrimaryColor)
	}
}

func TestRepository_InvoiceAndSectionMetaMapping(t *testing.T) {
	repo := NewMockTemplateRepository()
	ctx := context.Background()

	clinicID := uuid.New()
	invoiceID := uuid.New()

	// Mock data binding matching structural criteria
	repo.invoices[invoiceID] = &InvoiceResponse{
		ID:       invoiceID,
		ClinicID: clinicID,
		Status:   "PAID",
		Items: []InvoiceItem{
			{Name: "Consultation Fee", Amount: 150.00, EntryType: "CREDIT"},
		},
	}

	repo.sections[invoiceID] = []InvoiceSectionMeta{
		{
			SectionType:    "TAX_INVOICE",
			DocumentNumber: "INV-2026-001",
		},
	}

	inv, err := repo.GetInvoice(ctx, clinicID, invoiceID)
	if err != nil {
		t.Fatalf("Expected invoice to resolve cleanly, got: %v", err)
	}

	if len(inv.Items) != 1 || inv.Items[0].Amount != 150.00 {
		t.Errorf("Invalid invoice items content mapped inside data structures")
	}

	sections, err := repo.GetInvoiceSectionMeta(ctx, invoiceID)
	if err != nil || len(sections) != 1 || sections[0].DocumentNumber != "INV-2026-001" {
		t.Errorf("Section mapping matrix resolution error: %v", err)
	}
}

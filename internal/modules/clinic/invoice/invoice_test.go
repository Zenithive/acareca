package invoice

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/clinic/item"
	"github.com/iamarpitzala/acareca/internal/shared/testutil"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type InvoiceRepositoryTestSuite struct {
	suite.Suite
	db   *sqlx.DB
	repo IRepository
}

func (s *InvoiceRepositoryTestSuite) SetupSuite() {
	s.db = testutil.OpenTestDB(s.T())
	s.repo = NewRepository(s.db)
}

func (s *InvoiceRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *InvoiceRepositoryTestSuite) createTestInvoice(ctx context.Context) *Invoice {
	userID := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tbl_user (id, email, password, first_name, last_name, phone, role) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID,
		"invoice-test-"+uuid.NewString()+"@example.com",
		"password",
		"Invoice",
		"Creator",
		"3332221111",
		"PRACTITIONER",
	)
	s.Require().NoError(err)

	practitionerID := uuid.New()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tbl_practitioner (id, user_id, entity_type, entity_name, abn, acn, address, profession) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		practitionerID,
		userID,
		"SOLE_TRADER",
		"Invoice Practitioner",
		nil,
		nil,
		nil,
		nil,
	)
	s.Require().NoError(err)

	clinicID := uuid.New()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tbl_clinic (id, practitioner_id, name, is_active) VALUES ($1, $2, $3, $4)`,
		clinicID,
		practitionerID,
		"Invoice Clinic",
		true,
	)
	s.Require().NoError(err)

	contactID := uuid.New()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tbl_clinic_contact_person (id, clinic_id, fname, lname, email) VALUES ($1, $2, $3, $4, $5)`,
		contactID,
		clinicID,
		"Invoice",
		"Contact",
		"invoice-contact@example.com",
	)
	s.Require().NoError(err)

	invoice := &Invoice{
		ClinicID:      clinicID,
		ContactID:     &contactID,
		TemplateID:    uuid.New(),
		Name:          "Test Invoice",
		InvoiceNumber: "INV-001",
		Reference:     nil,
		PaymentMethod: nil,
		TaxMethod:     nil,
		IssueDate:     time.Now().Format("2006-01-02"),
		Status:        lo.ToPtr("draft"),
		Items: []*item.Item{
			{
				Name:        "Service A",
				Description: lo.ToPtr("Service item"),
				Quantity:    1,
				UnitPrice:   100,
				Discount:    nil,
				TaxRate:     nil,
				TaxAmount:   nil,
				TotalAmount: 100,
			},
			{
				Name:        "Service B",
				Description: lo.ToPtr("Second service"),
				Quantity:    2,
				UnitPrice:   50,
				Discount:    nil,
				TaxRate:     nil,
				TaxAmount:   nil,
				TotalAmount: 100,
			},
		},
	}

	err = s.repo.Create(ctx, invoice)
	s.Require().NoError(err)
	s.Require().NotEqual(uuid.Nil, invoice.ID)
	return invoice
}

func (s *InvoiceRepositoryTestSuite) TestCreateInvoice() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	invoice := s.createTestInvoice(ctx)
	testutil.CommitTx(s.T(), tx)

	s.Require().NotEqual(uuid.Nil, invoice.ID)
}

func (s *InvoiceRepositoryTestSuite) TestGetInvoice() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	invoice := s.createTestInvoice(ctx)
	testutil.CommitTx(s.T(), tx)

	fetched, err := s.repo.Get(ctx, invoice.ID)
	s.Require().NoError(err)
	s.Require().Equal(invoice.ID, fetched.ID)
	s.Require().Equal(2, len(fetched.Items))
}

func (s *InvoiceRepositoryTestSuite) TestDeleteInvoiceCascade() {
	ctx := context.Background()
	tx := testutil.BeginTx(s.T(), s.db)
	defer testutil.RollbackTx(s.T(), tx)

	invoice := s.createTestInvoice(ctx)
	testutil.CommitTx(s.T(), tx)

	err := s.repo.Delete(ctx, invoice.ID)
	s.Require().NoError(err)

	_, err = s.repo.Get(ctx, invoice.ID)
	s.Require().Error(err)
}

func TestInvoiceRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(InvoiceRepositoryTestSuite))
}

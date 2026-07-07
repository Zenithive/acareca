package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/util"
	"github.com/jmoiron/sqlx"
)

// dbInvoiceReader fetches invoice data directly via SQL to avoid importing
// the invoice package (which imports template, causing an import cycle).
type dbInvoiceReader struct {
	db *sqlx.DB
}

func newDBInvoiceReader(db *sqlx.DB) IInvoiceReader {
	return &dbInvoiceReader{db: db}
}

func (r *dbInvoiceReader) GetInvoiceMethod(ctx context.Context, invoiceId uuid.UUID) (util.InvoiceType, error) {
	var method sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT invoice_method FROM tbl_invoice WHERE id = $1 AND deleted_at IS NULL`,
		invoiceId,
	).Scan(&method)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("invoice %s not found", invoiceId)
		}
		return "", fmt.Errorf("failed to fetch invoice method: %w", err)
	}
	if !method.Valid || method.String == "" {
		return "", fmt.Errorf("invoice %s has no billing method set", invoiceId)
	}
	return util.InvoiceType(method.String), nil
}

// GetInvoiceRenderData fetches all data needed to render the invoice PDF and
// returns it as a map[string]interface{} shaped exactly as the Handlebars
// templates expect (bill_from.name, bill_to.address as a string, etc.).
func (r *dbInvoiceReader) GetInvoiceRenderData(ctx context.Context, invoiceId uuid.UUID) (map[string]interface{}, error) {
	// ── 1. Core invoice row + clinic + contact ──────────────────────────────
	const coreQ = `
		SELECT
			i.invoice_method,
			i.billing_period_from::text  AS billing_period_from,
			i.billing_period_to::text    AS billing_period_to,
			i.invoice_frequency,
			i.issue_date::text           AS issue_date,
			i.due_date::text             AS due_date,

			-- clinic (bill_from)
			cl.clinic_name,
			COALESCE(cl.abn,   '')       AS clinic_abn,
			COALESCE(cl.email, '')       AS clinic_email,
			COALESCE(
				(SELECT ca.address || ', ' || ca.city || ' ' || ca.state || ' ' || ca.postcode
				 FROM tbl_invoice_clinic_address ca
				 WHERE ca.clinic_id = cl.id AND ca.deleted_at IS NULL
				 ORDER BY ca.is_primary DESC, ca.created_at ASC
				 LIMIT 1),
				''
			) AS clinic_address,

			-- contact (bill_to)
			COALESCE(ct.fname, '')  AS contact_fname,
			COALESCE(ct.lname, '')  AS contact_lname,
			COALESCE(ct.email, '')  AS contact_email,
			COALESCE(ct.phone, '')  AS contact_phone,
			COALESCE(ct.abn,   '')  AS contact_abn,
			COALESCE(
				(SELECT addr.address_line1 || ', ' || addr.city || ' ' || addr.state || ' ' || addr.postal_code
				 FROM tbl_clinic_contact_person_address addr
				 WHERE addr.contact_id = ct.id AND addr.deleted_at IS NULL
				 ORDER BY addr.is_primary DESC, addr.created_at ASC
				 LIMIT 1),
				''
			) AS contact_address,

			-- contact payment details (for remittance)
			COALESCE(ct.payment_method,  '') AS contact_payment_method,
			COALESCE(ct.account_name,    '') AS contact_account_name,
			COALESCE(ct.bsb_number,      '') AS contact_bsb,
			COALESCE(ct.account_number,  '') AS contact_account_number

		FROM tbl_invoice i
		JOIN tbl_invoice_clinic cl ON cl.id = i.clinic_id AND cl.deleted_at IS NULL
		LEFT JOIN tbl_clinic_contact_person ct ON ct.id = i.contact_id AND ct.deleted_at IS NULL
		WHERE i.id = $1 AND i.deleted_at IS NULL
	`

	type coreRow struct {
		InvoiceMethod     sql.NullString `db:"invoice_method"`
		BillingPeriodFrom sql.NullString `db:"billing_period_from"`
		BillingPeriodTo   sql.NullString `db:"billing_period_to"`
		InvoiceFrequency  sql.NullString `db:"invoice_frequency"`
		IssueDate         sql.NullString `db:"issue_date"`
		DueDate           sql.NullString `db:"due_date"`

		ClinicName    sql.NullString `db:"clinic_name"`
		ClinicABN     sql.NullString `db:"clinic_abn"`
		ClinicEmail   sql.NullString `db:"clinic_email"`
		ClinicAddress sql.NullString `db:"clinic_address"`

		ContactFname         sql.NullString `db:"contact_fname"`
		ContactLname         sql.NullString `db:"contact_lname"`
		ContactEmail         sql.NullString `db:"contact_email"`
		ContactPhone         sql.NullString `db:"contact_phone"`
		ContactABN           sql.NullString `db:"contact_abn"`
		ContactAddress       sql.NullString `db:"contact_address"`
		ContactPaymentMethod sql.NullString `db:"contact_payment_method"`
		ContactAccountName   sql.NullString `db:"contact_account_name"`
		ContactBsb           sql.NullString `db:"contact_bsb"`
		ContactAccountNumber sql.NullString `db:"contact_account_number"`
	}

	var core coreRow
	if err := r.db.QueryRowxContext(ctx, coreQ, invoiceId).StructScan(&core); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("invoice %s not found", invoiceId)
		}
		return nil, fmt.Errorf("failed to fetch invoice core data: %w", err)
	}

	// ── 2. Sections (document numbers + payment dates) ──────────────────────
	const secQ = `
		SELECT invoice_section, document_number, COALESCE(payment_date::text, '') AS payment_date
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`
	type secRow struct {
		SectionType    sql.NullString `db:"invoice_section"`
		DocumentNumber sql.NullString `db:"document_number"`
		PaymentDate    sql.NullString `db:"payment_date"`
	}
	secRows, err := r.db.QueryxContext(ctx, secQ, invoiceId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sections: %w", err)
	}
	defer secRows.Close()
	var sections []secRow
	for secRows.Next() {
		var s secRow
		if err := secRows.StructScan(&s); err != nil {
			return nil, fmt.Errorf("failed to scan section: %w", err)
		}
		sections = append(sections, s)
	}
	_ = secRows.Err()

	// ── 3. Line items ────────────────────────────────────────────────────────
	const itemQ = `
		SELECT
			s.invoice_section   AS section_type,
			it.name,
			COALESCE(it.entry_type, '') AS entry_type,
			it.amount,
			COALESCE(it.bas_code, '')   AS bas_code,
			it.is_final,
			COALESCE(it.field_key, '')  AS field_key,
			it.sort_order,
			COALESCE(it.paid_by::text, '') AS paid_by
		FROM tbl_invoice_item it
		JOIN tbl_map_invoice_section s ON s.id = it.invoice_section_id AND s.deleted_at IS NULL
		WHERE it.invoice_id = $1
		  AND it.deleted_at IS NULL
		  AND it.parent_id IS NULL
		ORDER BY s.created_at ASC, it.sort_order ASC
	`
	type itemRow struct {
		SectionType sql.NullString  `db:"section_type"`
		Name        sql.NullString  `db:"name"`
		EntryType   sql.NullString  `db:"entry_type"`
		Amount      sql.NullFloat64 `db:"amount"`
		BASCode     sql.NullString  `db:"bas_code"`
		IsFinal     sql.NullBool    `db:"is_final"`
		FieldKey    sql.NullString  `db:"field_key"`
		SortOrder   sql.NullInt64   `db:"sort_order"`
		PaidBy      sql.NullString  `db:"paid_by"`
	}
	itemRowsCursor, err := r.db.QueryxContext(ctx, itemQ, invoiceId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch items: %w", err)
	}
	defer itemRowsCursor.Close()
	var allItems []itemRow
	for itemRowsCursor.Next() {
		var it itemRow
		if err := itemRowsCursor.StructScan(&it); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		allItems = append(allItems, it)
	}
	_ = itemRowsCursor.Err()

	// ── 4. Derive display values ─────────────────────────────────────────────
	invoiceNumber := ""
	paymentDate := ""
	for _, s := range sections {
		if invoiceNumber == "" && s.DocumentNumber.String != "" {
			invoiceNumber = s.DocumentNumber.String
		}
		if paymentDate == "" && s.PaymentDate.String != "" {
			paymentDate = s.PaymentDate.String
		}
	}

	billingPeriod := ""
	if core.BillingPeriodFrom.String != "" || core.BillingPeriodTo.String != "" {
		billingPeriod = core.BillingPeriodFrom.String + " to " + core.BillingPeriodTo.String
	}

	contactName := strings.TrimSpace(core.ContactFname.String + " " + core.ContactLname.String)

	// ── 5. Categorise items ──────────────────────────────────────────────────
	patientFeeItems := make([]map[string]interface{}, 0)
	treatmentCostItems := make([]map[string]interface{}, 0)
	netPatientFeeItems := make([]map[string]interface{}, 0)
	serviceFeeItems := make([]map[string]interface{}, 0)
	taxInvoiceItems := make([]map[string]interface{}, 0)
	remittanceItems := make([]map[string]interface{}, 0)

	var grandTotal, subtotal float64
	for _, it := range allItems {
		amount := it.Amount.Float64
		isNeg := amount < 0
		entry := map[string]interface{}{
			"label":       it.Name.String,
			"amount":      amount,
			"bas_code":    it.BASCode.String,
			"entry_type":  it.EntryType.String,
			"is_final":    it.IsFinal.Bool,
			"is_negative": isNeg,
			"paid_by":     it.PaidBy.String,
			"field_key":   it.FieldKey.String,
			"sort_order":  it.SortOrder.Int64,
		}
		if it.IsFinal.Bool {
			// final rows get a bold style class
			entry["row_class"] = "row-bold"
		}

		switch it.SectionType.String {
		case "CALCULATION_STATEMENT":
			switch it.FieldKey.String {
			case "patient_fee", "total_patient_fees", "gst_collected", "gst_1a":
				patientFeeItems = append(patientFeeItems, entry)
			case "treatment_cost", "lab_fee":
				treatmentCostItems = append(treatmentCostItems, entry)
			case "net_patient_fee", "net_fees":
				netPatientFeeItems = append(netPatientFeeItems, entry)
			default:
				serviceFeeItems = append(serviceFeeItems, entry)
			}
		case "SFA_INVOICE", "RCTI":
			taxInvoiceItems = append(taxInvoiceItems, entry)
			subtotal += amount
			grandTotal += amount
		case "REMITTANCE_INVOICE":
			remittanceItems = append(remittanceItems, entry)
		}
	}

	taxTotal := grandTotal * 0.1

	// ── 6. Build final render map ────────────────────────────────────────────
	data := map[string]interface{}{
		// Header
		"invoice_number":    invoiceNumber,
		"issue_date_display": core.IssueDate.String,
		"due_date_display":  core.DueDate.String,
		"billing_period":    billingPeriod,
		"invoice_frequency": core.InvoiceFrequency.String,
		"billing_method_type": core.InvoiceMethod.String,

		// Clinic = bill_from (templates use bill_from.name, bill_from.abn, etc.)
		"bill_from": map[string]interface{}{
			"name":    core.ClinicName.String,
			"abn":     core.ClinicABN.String,
			"email":   core.ClinicEmail.String,
			"phone":   "",
			"address": core.ClinicAddress.String,
		},

		// Contact = bill_to (templates use bill_to.name, bill_to.address, etc.)
		"bill_to": map[string]interface{}{
			"name":    contactName,
			"abn":     core.ContactABN.String,
			"email":   core.ContactEmail.String,
			"phone":   core.ContactPhone.String,
			"address": core.ContactAddress.String,
		},

		// Payment
		"payment_date_display":        paymentDate,
		"custom_payment_method":       core.ContactPaymentMethod.String,
		"custom_payment_account_name": core.ContactAccountName.String,
		"custom_payment_bsb":          core.ContactBsb.String,
		"custom_payment_account":      core.ContactAccountNumber.String,

		// Totals
		"grand_total": grandTotal,
		"subtotal":    subtotal,
		"tax_total":   taxTotal,

		// Item buckets
		"patient_fee_items":     patientFeeItems,
		"treatment_cost_items":  treatmentCostItems,
		"net_patient_fee_items": netPatientFeeItems,
		"service_fee_items":     serviceFeeItems,
		"tax_invoice_items":     taxInvoiceItems,
		"remittance_items":      remittanceItems,

		// Method flag helpers
		"is_method_c": core.InvoiceMethod.String == "INDEPENDENT_CONTRACTOR",
	}

	return data, nil
}

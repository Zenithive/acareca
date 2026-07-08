package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

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

type expressionNode struct {
	Type  string          `json:"type"`
	Key   string          `json:"key"`
	Op    string          `json:"op"`
	Left  *expressionNode `json:"left,omitempty"`
	Right *expressionNode `json:"right,omitempty"`
}

func parseExpressionString(expression string) string {
	if strings.TrimSpace(expression) == "" {
		return ""
	}

	var node expressionNode
	if err := json.Unmarshal([]byte(expression), &node); err != nil {
		return strings.TrimSpace(expression)
	}

	return formatExpressionNode(&node)
}

func formatExpressionNode(node *expressionNode) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "field":
		return node.Key
	case "operator":
		left := formatExpressionNode(node.Left)
		right := formatExpressionNode(node.Right)
		if left == "" || right == "" {
			return strings.TrimSpace(left + " " + right)
		}
		return fmt.Sprintf("%s %s %s", left, node.Op, right)
	default:
		if node.Key != "" {
			return node.Key
		}
		return ""
	}
}

func buildItemBucketKey(sortOrder int64, fieldKey string, label string) string {
	return fmt.Sprintf("%d|%s|%s", sortOrder, fieldKey, label)
}

func appendUniqueItem(dst *[]map[string]interface{}, seen map[string]map[string]struct{}, bucket string, entry map[string]interface{}, sortOrder int64, fieldKey string, label string) {
	if seen[bucket] == nil {
		seen[bucket] = map[string]struct{}{}
	}
	key := buildItemBucketKey(sortOrder, fieldKey, label)
	if _, exists := seen[bucket][key]; exists {
		return
	}
	seen[bucket][key] = struct{}{}
	*dst = append(*dst, entry)
}

func reformatToDDMMYYYY(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
		return parsed.Format("02-01-2006")
	}
	if parsed, err := time.Parse("02 January 2006", dateStr); err == nil {
		return parsed.Format("02-01-2006")
	}
	return dateStr
}

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
		InvoiceMethod        sql.NullString `db:"invoice_method"`
		BillingPeriodFrom    sql.NullString `db:"billing_period_from"`
		BillingPeriodTo      sql.NullString `db:"billing_period_to"`
		InvoiceFrequency     sql.NullString `db:"invoice_frequency"`
		IssueDate            sql.NullString `db:"issue_date"`
		DueDate              sql.NullString `db:"due_date"`
		ClinicName           sql.NullString `db:"clinic_name"`
		ClinicABN            sql.NullString `db:"clinic_abn"`
		ClinicEmail          sql.NullString `db:"clinic_email"`
		ClinicAddress        sql.NullString `db:"clinic_address"`
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

	// ── 2. Sections (Extracting explicit document numbers from DB) ──────────
	const secQ = `
		SELECT id::text AS section_id, invoice_section, document_number, COALESCE(payment_date::text, '') AS payment_date
		FROM tbl_map_invoice_section
		WHERE invoice_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC
	`
	type secRow struct {
		SectionID      sql.NullString `db:"section_id"`
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
	validSectionIDs := make(map[string]bool)

	var calculationDocNo, taxInvoiceDocNo, remittanceDocNo string
	var invoiceNumber, paymentDate string

	for secRows.Next() {
		var s secRow
		if err := secRows.StructScan(&s); err != nil {
			return nil, fmt.Errorf("failed to scan section: %w", err)
		}
		sections = append(sections, s)
		if s.SectionID.String != "" {
			validSectionIDs[s.SectionID.String] = true
		}

		if paymentDate == "" && s.PaymentDate.String != "" {
			paymentDate = reformatToDDMMYYYY(s.PaymentDate.String)
		}

		secTypeUpper := strings.ToUpper(strings.TrimSpace(s.SectionType.String))
		switch secTypeUpper {
		case "CALCULATION_STATEMENT":
			calculationDocNo = s.DocumentNumber.String
			if invoiceNumber == "" {
				invoiceNumber = s.DocumentNumber.String
			}
		case "SFA_INVOICE", "TAX_INVOICE", "COMMISSION", "RCTI":
			taxInvoiceDocNo = s.DocumentNumber.String
			if invoiceNumber == "" {
				invoiceNumber = s.DocumentNumber.String
			}
		case "REMITTANCE_INVOICE", "REMITTANCE_ADVICE", "NET_SETTLEMENT":
			remittanceDocNo = s.DocumentNumber.String
			if invoiceNumber == "" {
				invoiceNumber = s.DocumentNumber.String
			}
		}
	}
	_ = secRows.Err()

	// ── 3. Line items ────────────────────────────────────────────────────────
	const itemQ = `
		SELECT
			s.invoice_section   AS section_type,
			s.id::text          AS section_id,
			it.invoice_section_id::text AS invoice_section_id,
			it.name,
			COALESCE(it.entry_type, '') AS entry_type,
			it.amount,
			COALESCE(it.bas_code, '')   AS bas_code,
			it.is_final,
			COALESCE(it.field_key, '')  AS field_key,
			it.sort_order,
			CASE WHEN it.paid_by IS NULL THEN '' ELSE it.paid_by::text END AS paid_by,
			COALESCE(it.expression, '') AS expression,
			COALESCE(it.description, '') AS description
		FROM tbl_invoice_item it
		JOIN tbl_map_invoice_section s
			ON s.id = it.invoice_section_id
			AND s.invoice_id = $1
			AND s.deleted_at IS NULL
		WHERE it.deleted_at IS NULL
		  AND it.parent_id IS NULL
		ORDER BY s.created_at ASC, it.sort_order ASC
	`
	type itemRow struct {
		SectionType      sql.NullString  `db:"section_type"`
		SectionID        sql.NullString  `db:"section_id"`
		InvoiceSectionID sql.NullString  `db:"invoice_section_id"`
		Name             sql.NullString  `db:"name"`
		EntryType        sql.NullString  `db:"entry_type"`
		Amount           sql.NullFloat64 `db:"amount"`
		BASCode          sql.NullString  `db:"bas_code"`
		IsFinal          sql.NullBool    `db:"is_final"`
		FieldKey         sql.NullString  `db:"field_key"`
		SortOrder        sql.NullInt64   `db:"sort_order"`
		PaidBy           sql.NullString  `db:"paid_by"`
		Expression       sql.NullString  `db:"expression"`
		Description      sql.NullString  `db:"description"`
	}
	itemRowsCursor, err := r.db.QueryxContext(ctx, itemQ, invoiceId)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch items: %w", err)
	}
	defer itemRowsCursor.Close()
	var allItems []itemRow

	maxSortOrderPerSection := make(map[string]int64)
	itemCountPerSection := make(map[string]int)

	for itemRowsCursor.Next() {
		var it itemRow
		if err := itemRowsCursor.StructScan(&it); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		allItems = append(allItems, it)

		sid := it.InvoiceSectionID.String
		itemCountPerSection[sid]++
		if it.SortOrder.Int64 > maxSortOrderPerSection[sid] {
			maxSortOrderPerSection[sid] = it.SortOrder.Int64
		}
	}
	_ = itemRowsCursor.Err()

	log.Printf("invoice %s: fetched %d raw item rows from tbl_invoice_item", invoiceId, len(allItems))

	// ── 4. Derive display values using strict DD-MM-YYYY layout ──────────────
	issueDateDisplay := reformatToDDMMYYYY(core.IssueDate.String)
	dueDateDisplay := reformatToDDMMYYYY(core.DueDate.String)

	billingPeriod := ""
	if core.BillingPeriodFrom.String != "" || core.BillingPeriodTo.String != "" {
		fromFormatted := reformatToDDMMYYYY(core.BillingPeriodFrom.String)
		toFormatted := reformatToDDMMYYYY(core.BillingPeriodTo.String)
		billingPeriod = fromFormatted + " to " + toFormatted
	}

	contactName := strings.TrimSpace(core.ContactFname.String + " " + core.ContactLname.String)

	// ── 5. Extract Fee Rate Descriptions & Custom Fee Details ───────────────
	var customFeeRateDisplay string
	var serviceDescriptionItems []string
	seenDescriptionLines := make(map[string]bool)

	for _, it := range allItems {
		if it.FieldKey.String == "" {
			continue
		}
		keyUpper := strings.ToUpper(strings.TrimSpace(it.FieldKey.String))
		if keyUpper == "FEE_RATE" || keyUpper == "SERVICE_FEE_RATE" {
			feeRateVal := it.Amount.Float64
			if feeRateVal > 0 && feeRateVal <= 1 {
				feeRateVal *= 100
			}
			customFeeRateDisplay = fmt.Sprintf("%.1f%%", feeRateVal)

			if it.Description.String != "" {
				normalized := strings.ReplaceAll(it.Description.String, "\r\n", "\n")
				lines := strings.Split(normalized, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !seenDescriptionLines[line] {
						seenDescriptionLines[line] = true
						serviceDescriptionItems = append(serviceDescriptionItems, line)
					}
				}
			}
		}
	}

	// ── 6. Categorise items & Apply CSS Stylings ─────────────────────────────
	patientFeeItems := make([]map[string]interface{}, 0)
	treatmentCostItems := make([]map[string]interface{}, 0)
	netPatientFeeItems := make([]map[string]interface{}, 0)
	serviceFeeItems := make([]map[string]interface{}, 0)
	invoiceFeeItems := make([]map[string]interface{}, 0)
	rctiFeeItems := make([]map[string]interface{}, 0)
	remittanceItems := make([]map[string]interface{}, 0)
	settlementItems := make([]map[string]interface{}, 0)

	bucketSeen := map[string]map[string]struct{}{}
	var grandTotal, subtotal, taxTotal float64
	sfaItemIndex := 0

	for _, it := range allItems {
		if it.InvoiceSectionID.String == "" || !validSectionIDs[it.InvoiceSectionID.String] {
			continue
		}

		keyUpper := strings.ToUpper(strings.TrimSpace(it.FieldKey.String))
		isPercentage := keyUpper == "FEE_RATE" || keyUpper == "SERVICE_FEE_RATE"

		if isPercentage {
			continue
		}

		amount := it.Amount.Float64
		entryTypeUpper := strings.ToUpper(strings.TrimSpace(it.EntryType.String))
		isDebit := entryTypeUpper == "DEBIT"
		isNegative := isDebit || amount < 0

		rowClass := ""
		valueClass := ""
		hasFormula := it.Expression.String != ""
		isBold := it.IsFinal.Bool || hasFormula

		sid := it.InvoiceSectionID.String
		isLastItemInSection := it.SortOrder.Int64 == maxSortOrderPerSection[sid] && itemCountPerSection[sid] > 1

		if it.IsFinal.Bool {
			rowClass = "bg-sky-row"
		} else if isLastItemInSection {
			rowClass = "row-total"
		}

		if !hasFormula && !it.IsFinal.Bool {
			valueClass = "txt-blue-val"
		}

		lowerName := strings.ToLower(strings.TrimSpace(it.Name.String))

		displayLabel := it.Name.String
		if hasFormula {
			if cleanFormula := parseExpressionString(it.Expression.String); cleanFormula != "" {
				displayLabel = fmt.Sprintf("%s [ %s ]", it.Name.String, cleanFormula)
			}
		}

		if strings.Contains(lowerName, "net balance") {
			displayLabel = "Net balance [ Total Income - Total Expenses ]"
			if amount > 0 {
				valueClass = "amt-pos"
			} else if amount < 0 {
				valueClass = "amt-neg"
			}
		}

		if isDebit {
			displayLabel = "Less: " + displayLabel
		}

		if it.FieldKey.String != "" {
			displayLabel = fmt.Sprintf("(%s) %s", it.FieldKey.String, displayLabel)
		}

		entry := map[string]interface{}{
			"label":         displayLabel,
			"amount":        amount,
			"bas_code":      strings.TrimSpace(it.BASCode.String),
			"entry_type":    it.EntryType.String,
			"is_final":      it.IsFinal.Bool,
			"is_bold":       isBold,
			"is_negative":   isNegative,
			"is_percentage": isPercentage,
			"paid_by":       it.PaidBy.String,
			"field_key":     it.FieldKey.String,
			"sort_order":    it.SortOrder.Int64,
			"expression":    it.Expression.String,
			"row_class":     rowClass,
			"value_class":   valueClass,
		}

		secTypeUpper := strings.ToUpper(strings.TrimSpace(it.SectionType.String))
		switch secTypeUpper {
		case "PATIENT_FEE":
			appendUniqueItem(&patientFeeItems, bucketSeen, "patient_fee_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
		case "TREATMENT_COST":
			appendUniqueItem(&treatmentCostItems, bucketSeen, "treatment_cost_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
		case "NET_PATIENT_FEES":
			appendUniqueItem(&netPatientFeeItems, bucketSeen, "net_patient_fee_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
		case "SERVICE_FACILITY", "SERVICE_FACILITY_FEE", "SERVICE_AND_FACILITY_FEE":
			appendUniqueItem(&serviceFeeItems, bucketSeen, "service_fee_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
			appendUniqueItem(&invoiceFeeItems, bucketSeen, "invoice_fee_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)

			if it.IsFinal.Bool {
				grandTotal = amount
			} else {
				switch sfaItemIndex {
				case 0:
					subtotal = amount
				}
				sfaItemIndex++
			}
		case "COMMISSION":
			if core.InvoiceMethod.String == "INDEPENDENT_CONTRACTOR" {
				appendUniqueItem(&serviceFeeItems, bucketSeen, "service_fee_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
			}
			appendUniqueItem(&rctiFeeItems, bucketSeen, "rcti_fee_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)

			if it.IsFinal.Bool {
				grandTotal = amount
			} else {
				switch sfaItemIndex {
				case 0:
					subtotal = amount
				}
				sfaItemIndex++
			}
		case "NET_SETTLEMENT", "REMITTANCE_INVOICE", "REMITTANCE_ADVICE":
			appendUniqueItem(&remittanceItems, bucketSeen, "remittance_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
			appendUniqueItem(&settlementItems, bucketSeen, "settlement_items", entry, it.SortOrder.Int64, it.FieldKey.String, displayLabel)
		default:
			log.Printf(
				"WARNING: invoice %s item %q has unmapped section_type %q — item skipped from render data",
				invoiceId, it.Name.String, it.SectionType.String,
			)
		}
	}

	taxTotal = grandTotal * 0.1

	// ── 7. Build Explicit Scopes to align with Handlebars structures ──────
	data := map[string]interface{}{
		"invoice_number":          invoiceNumber,
		"issue_date_display":      issueDateDisplay,
		"due_date_display":        dueDateDisplay,
		"billing_period":          billingPeriod,
		"invoice_frequency":       core.InvoiceFrequency.String,
		"billing_method_type":     core.InvoiceMethod.String,
		"custom_fee_rate_display": customFeeRateDisplay,

		"bill_from": map[string]interface{}{
			"name":    core.ClinicName.String,
			"abn":     core.ClinicABN.String,
			"email":   core.ClinicEmail.String,
			"phone":   "",
			"address": core.ClinicAddress.String,
		},

		"bill_to": map[string]interface{}{
			"name":    contactName,
			"abn":     core.ContactABN.String,
			"email":   core.ContactEmail.String,
			"phone":   core.ContactPhone.String,
			"address": core.ContactAddress.String,
		},

		"payment_date_display":        paymentDate,
		"custom_payment_method":       core.ContactPaymentMethod.String,
		"custom_payment_account_name": core.ContactAccountName.String,
		"custom_payment_bsb":          core.ContactBsb.String,
		"custom_payment_account":      core.ContactAccountNumber.String,

		"grand_total": grandTotal,
		"subtotal":    subtotal,
		"tax_total":   taxTotal,

		"patient_fee_items":     patientFeeItems,
		"treatment_cost_items":  treatmentCostItems,
		"net_patient_fee_items": netPatientFeeItems,
		"service_fee_items":     serviceFeeItems,
		"invoice_fee_items":     invoiceFeeItems,
		"rcti_fee_items":        rctiFeeItems,
		"remittance_items":      remittanceItems,
		"settlement_items":      settlementItems,

		"tax_invoice_description_items": serviceDescriptionItems,

		"service_fee_rate_intro": map[string]interface{}{
			"label":            fmt.Sprintf("Service and facility fee for the period %s, calculated at the agreed rate on net patient fees, comprising:", billingPeriod),
			"fee_rate_display": customFeeRateDisplay,
			"amount_display":   "",
		},

		"template_settings": map[string]interface{}{
			"calculation_invoice_number": calculationDocNo,
			"tax_invoice_number":         taxInvoiceDocNo,
			"remittance_invoice_number":  remittanceDocNo,
		},

		"is_method_b": core.InvoiceMethod.String == "SFA_DENTIST_COLLECTS",
		"is_method_c": core.InvoiceMethod.String == "INDEPENDENT_CONTRACTOR",
	}

	return data, nil
}

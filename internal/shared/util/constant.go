package util

const (
	RoleAdmin        = "ADMIN"
	RolePractitioner = "PRACTITIONER"
	RoleAccountant   = "ACCOUNTANT"
)

const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
)

const (
	SectionTypeCollection         = "COLLECTION"
	SectionTypeCost               = "COST"
	SectionTypeOtherCost          = "OTHER_COST"
	SectionTypeEquityWithdrawal   = "EQUITY_WITHDRAWAL"   // For owner drawings
	SectionTypeEquityContribution = "EQUITY_CONTRIBUTION" // For owner funds introduced
)

const (
	PaymentResponsibilityOwner  = "OWNER"
	PaymentResponsibilityClinic = "CLINIC"
)

const (
	TaxTypeInclusive = "INCLUSIVE"
	TaxTypeExclusive = "EXCLUSIVE"
	TaxTypeManual    = "MANUAL"
)

const (
	ExpenseEntry = "EXPENSE_ENTRY"
)

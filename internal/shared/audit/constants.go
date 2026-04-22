package audit

// Module constants
const (
	ModuleAuth      = "auth"
	ModuleAdmin     = "admin"
	ModuleClinic    = "clinic"
	ModuleForms     = "forms"
	ModuleReporting = "reporting"
	ModuleBilling   = "billing"
	ModuleIdentity  = "identity"
	ModuleEngine    = "engine"
	ModuleBusiness  = "business"
)

// Action constants - Auth module
const (
	ActionUserRegistered  = "user.registered"
	ActionUserUpdated     = "user.updated"
	ActionUserDeleted     = "user.deleted"
	ActionUserLoggedIn    = "user.logged_in"
	ActionUserLoggedOut   = "user.logged_out"
	ActionPasswordReset   = "user.password_reset"
	ActionPasswordChanged = "user.password_changed"
	ActionEmailVerified   = "user.email_verified"
	ActionOAuthLinked     = "user.oauth_linked"
	ActionSessionCreated  = "session.created"
	ActionSessionRevoked  = "session.revoked"
)

// Action constants - Admin module
const (
	ActionSubscriptionCreated = "subscription.created"
	ActionSubscriptionUpdated = "subscription.updated"
	ActionSubscriptionDeleted = "subscription.deleted"
	ActionPermissionGranted   = "permission.granted"
	ActionPermissionRevoked   = "permission.revoked"
)

// Action constants - Business module
const (
	ActionClinicCreated       = "clinic.created"
	ActionClinicUpdated       = "clinic.updated"
	ActionClinicDeleted       = "clinic.deleted"
	ActionPractitionerCreated = "practitioner.created"
	ActionPractitionerUpdated = "practitioner.updated"
	ActionPractitionerDeleted = "practitioner.deleted"
	ActionSettingUpdated      = "setting.updated"
	ActionCOACreated          = "coa.created"
	ActionCOAUpdated          = "coa.updated"
	ActionCOADeleted          = "coa.deleted"
	ActionFYCreated           = "fy.created"
	ActionFYUpdated           = "fy.updated"
	ActionFYClosed            = "fy.closed"
	ActionFYActivated         = "fy.activated"
)

// Action constants - Forms module
const (
	ActionFormCreated    = "form.created"
	ActionFormUpdated    = "form.updated"
	ActionFormDeleted    = "form.deleted"
	ActionEntryCreated   = "entry.created"
	ActionEntryUpdated   = "entry.updated"
	ActionEntryConfirmed = "entry.confirmed"
	ActionEntryDeleted   = "entry.deleted"
)

// Entity type constants
const (
	EntityUser                   = "tbl_user"
	EntitySession                = "tbl_session"
	EntityAuthProvider           = "tbl_auth_provider"
	EntitySubscription           = "tbl_subscription"
	EntityPractitioner           = "tbl_practitioner"
	EntityClinic                 = "tbl_clinic"
	EntityClinicAddress          = "tbl_clinic_address"
	EntityClinicContact          = "tbl_clinic_contact"
	EntityCOA                    = "tbl_chart_of_accounts"
	EntityFinancialYear          = "tbl_financial_year"
	EntityFinancialSettings      = "tbl_financial_settings"
	EntityForm                   = "tbl_form"
	EntityFormFieldEntry         = "tbl_form_entry"
	EntityPlanPermission         = "tbl_plan_permission"
	EntitySubscriptionPermission = "tbl_subscription_permission"
	EntityVerificationToken      = "tbl_verification_token"
	EntityInvitation             = "tbl_invitation"
	EntityPermission             = "tbl_invite_permissions"
	EntitySystem                 = "system"
)

// Action Constants -Accountant module
const (
	ActionInviteAccepted  = "accountant.invite_accepted"
	ActionInviteRejected  = "accountant.invite_rejected"
	ActionInviteSent      = "accountant.invite_sent"
	ActionInviteExpired   = "accountant.invite_expired"
	ActionInviteCompleted = "accountant.invite_completed"
	ActionInviteRevoked   = "accountant.invite_revoked"
)

// Action Constants - Invite Permissions
const (
	ActionPermissionAssigned = "invite.permission_assigned"
	ActionPermissionUpdated  = "invite.permission_updated"
)

// Module constant - System
const ModuleSystem = "system"

// Action constants - System-level errors and warnings
const (
	ActionSystemError   = "system.error"
	ActionSystemWarning = "system.warning"
)

// Report Export Logs
const ModuleReport = "report"

const (
	EntityBASReport         = "bas_report"
	EntityPLReport          = "pl_report"
	EntityActivityStatement = "activity_statement"
	EntityTransactions      = "transactions"
)

const (
	ActionBASReportExported         = "bas_report.exported"
	ActionPLReportExported          = "pl_report.exported"
	ActionActivityStatementExported = "activity_statement.exported"
	ActitionTransactionsExported    = "transactions.exported"
)

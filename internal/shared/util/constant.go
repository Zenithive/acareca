package util

const (
	RoleAdmin        = "ADMIN"
	RolePractitioner = "PRACTITIONER"
	RoleAccountant   = "ACCOUNTANT"
	RoleClinic       = "CLINIC"
)

type Status string

const (
	StatusUnread    Status = "UNREAD"
	StatusRead      Status = "READ"
	StatusDismissed Status = "DISMISSED"
)

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "PENDING"
	DeliveryDelivered DeliveryStatus = "DELIVERED"
	DeliveryFailed    DeliveryStatus = "FAILED"
)

type EventType string

const (
	EventInviteSent        EventType = "invite.sent"
	EventInviteAccepted    EventType = "invite.accepted"
	EventInviteDeclined    EventType = "invite.declined"
	EventPermissionUpdated EventType = "permission.updated"

	EventClinicUpdated              EventType = "clinic.updated"
	EventFormSubmitted              EventType = "form.submitted"
	EventFormUpdated                EventType = "form.updated"
	EventTransactionCreated         EventType = "transaction.created"
	EventTransactionUpdated         EventType = "transaction.status_changed"
	EventDocumentUploaded           EventType = "document.uploaded"
	EventTransactionReportExport    EventType = "transaction.event.export"
	EventPLReportGenerated          EventType = "pl.report.generated"
	EventPLReportExport             EventType = "pl.report.export"
	EventBASReportGenerated         EventType = "bas.report.generated"
	EventBASReportExport            EventType = "bas.report.export"
	EventBalanceSheetGenerated      EventType = "balance_sheet.generated"
	EventBalanceSheetExport         EventType = "balance_sheet.export"
	EventActivityStatementGenerated EventType = "activity_statement.generated"
	EventActivityStatementExport    EventType = "activity_statement.export"

	//Pratitioner
	EventPractitionerTransactionCreated EventType = "pratitioner.transaction.created"

	EventAuditLogCreated EventType = "audit_log.created"
	EventSystemError     EventType = "system.error"
	EventSystemWarning   EventType = "system.warning"

	// Admin-specific event types
	EventUserRegistered        EventType = "user.registered"
	EventPractitionerCreated   EventType = "practitioner.created"
	EventBillingPaymentSuccess EventType = "billing.payment_success"
	EventBillingPaymentFailed  EventType = "billing.payment_failed"
	EventSubscriptionCreated   EventType = "subscription.created"
	EventSubscriptionUpdated   EventType = "subscription.updated"
	EventSubscriptionDeleted   EventType = "subscription.deleted"
)

type EntityType string

const (
	EntityClinic       EntityType = "clinic"
	EntityForm         EntityType = "form"
	EntityTransaction  EntityType = "transaction"
	EntityDocument     EntityType = "document"
	EntityInvite       EntityType = "invite"
	EntityAuditLog     EntityType = "audit_log"
	EntitySystem       EntityType = "system"
	EntityReport       EntityType = "report"
	EntitySubscription EntityType = "subscription"
)

type ActorType string

const (
	ActorPractitioner ActorType = "PRACTITIONER"
	ActorAccountant   ActorType = "ACCOUNTANT"
	ActorAdmin        ActorType = "ADMIN"
	ActorSystem       ActorType = "SYSTEM"
)

type Channel string

const (
	ChannelInApp Channel = "in_app"
	ChannelPush  Channel = "push"
	ChannelEmail Channel = "email"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelInApp, ChannelPush, ChannelEmail:
		return true
	default:
		return false
	}
}

type NotificationEventType string

const (
	// Shared (practitioner + accountant)
	EventNewTransaction            NotificationEventType = "new.transaction"
	EventAccountantActivityAlert   NotificationEventType = "accountant.activity.alert"
	EventPractitionerActivityAlert NotificationEventType = "practitioner.activity.alert"

	// Admin-specific notification preference categories
	EventSystemActivityAlert   NotificationEventType = "system.activity.alert"   // general audit log activity
	EventSystemErrorAlert      NotificationEventType = "system.error.alert"      // system.error only (critical)
	EventSystemWarningAlert    NotificationEventType = "system.warning.alert"    // system.warning only
	EventBillingAlert          NotificationEventType = "billing.alert"           // payment success/failure
	EventSubscriptionAlert     NotificationEventType = "subscription.alert"      // subscription created/updated/deleted
	EventUserRegistrationAlert NotificationEventType = "user.registration.alert" // new practitioner registered
)

type InvoiceType string

const (
	InvoiceTypeSFAClinicCollects     InvoiceType = "SFA_CLINIC_COLLECTS"
	InvoiceTypeSFADentistCollects    InvoiceType = "SFA_DENTIST_COLLECTS"
	InvoiceTypeIndependentContractor InvoiceType = "INDEPENDENT_CONTRACTOR"
)

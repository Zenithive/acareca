package invitation

import "errors"

// Sentinel errors for invitation operations
var (
	ErrInvitationNotFound     = errors.New("invitation not found")
	ErrInvitationExpired      = errors.New("invitation expired")
	ErrInvitationInvalidated  = errors.New("this invitation link is no longer valid as a new one has been sent")
	ErrInvitationAlreadyUsed  = errors.New("invitation already processed")
	ErrUnauthorizedInvite     = errors.New("unauthorized: you did not send this invitation")
	ErrInvalidAction          = errors.New("invalid action: must be ACCEPT or REJECT")
	ErrDailyLimitReached      = errors.New("daily invitation limit reached for this email (max 5 per 24h)")
	ErrCannotRevokeStatus     = errors.New("cannot revoke invitation with current status")
	ErrUnauthorizedAssociation = errors.New("unauthorized association")
	ErrConfigMissing          = errors.New("system configuration error: frontend application URL is not defined")
)

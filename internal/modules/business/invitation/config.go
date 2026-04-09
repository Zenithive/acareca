package invitation

import "time"

// InvitationConfig holds configurable values for invitation management
type InvitationConfig struct {
	ExpirationDays     int
	DailyInviteLimit   int
	EmailTimeout       time.Duration
}

// DefaultConfig returns default invitation configuration
func InviteDefaultConfig() InvitationConfig {
	return InvitationConfig{
		ExpirationDays:   7,
		DailyInviteLimit: 5,
		EmailTimeout:     10 * time.Second,
	}
}

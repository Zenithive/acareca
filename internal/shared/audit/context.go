package audit

import (
	"context"
)

// Context keys for audit metadata
type contextKey string

const (
	contextKeyUserID     contextKey = "audit_user_id"
	contextKeyPracticeID contextKey = "audit_practice_id"
	contextKeyIPAddress  contextKey = "audit_ip_address"
	contextKeyUserAgent  contextKey = "audit_user_agent"
)

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// GetUserID retrieves user ID from context
func GetUserID(ctx context.Context) *string {
	if v := ctx.Value(contextKeyUserID); v != nil {
		if userID, ok := v.(string); ok {
			return &userID
		}
	}
	return nil
}

// WithPracticeID adds practice ID to context
func WithPracticeID(ctx context.Context, practiceID string) context.Context {
	return context.WithValue(ctx, contextKeyPracticeID, practiceID)
}

// GetPracticeID retrieves practice ID from context
func GetPracticeID(ctx context.Context) *string {
	if v := ctx.Value(contextKeyPracticeID); v != nil {
		if practiceID, ok := v.(string); ok {
			return &practiceID
		}
	}
	return nil
}

// WithIPAddress adds IP address to context
func WithIPAddress(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, contextKeyIPAddress, ip)
}

// GetIPAddress retrieves IP address from context
func GetIPAddress(ctx context.Context) *string {
	if v := ctx.Value(contextKeyIPAddress); v != nil {
		if ip, ok := v.(string); ok {
			return &ip
		}
	}
	return nil
}

// WithUserAgent adds user agent to context
func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, contextKeyUserAgent, ua)
}

// GetUserAgent retrieves user agent from context
func GetUserAgent(ctx context.Context) *string {
	if v := ctx.Value(contextKeyUserAgent); v != nil {
		if ua, ok := v.(string); ok {
			return &ua
		}
	}
	return nil
}

// Metadata holds all audit context information
type Metadata struct {
	UserID     *string
	PracticeID *string
	IPAddress  *string
	UserAgent  *string
}

// GetMetadata extracts all audit metadata from context
func GetMetadata(ctx context.Context) *Metadata {
	return &Metadata{
		UserID:     GetUserID(ctx),
		PracticeID: GetPracticeID(ctx),
		IPAddress:  GetIPAddress(ctx),
		UserAgent:  GetUserAgent(ctx),
	}
}

// WithMetadata adds all audit metadata to context
func WithMetadata(ctx context.Context, meta *Metadata) context.Context {
	if meta.UserID != nil {
		ctx = WithUserID(ctx, *meta.UserID)
	}
	if meta.PracticeID != nil {
		ctx = WithPracticeID(ctx, *meta.PracticeID)
	}
	if meta.IPAddress != nil {
		ctx = WithIPAddress(ctx, *meta.IPAddress)
	}
	if meta.UserAgent != nil {
		ctx = WithUserAgent(ctx, *meta.UserAgent)
	}
	return ctx
}

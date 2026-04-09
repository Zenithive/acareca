-- +goose Up
-- Add indexes for analytics performance optimization
-- These indexes significantly improve query performance for admin analytics endpoints

-- Audit log indexes for activity tracking
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON tbl_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity_type ON tbl_audit_log(entity_type);
CREATE INDEX IF NOT EXISTS idx_audit_log_user_id ON tbl_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON tbl_audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity_type_created_at ON tbl_audit_log(entity_type, created_at);

-- Practitioner subscription indexes for subscription metrics
CREATE INDEX IF NOT EXISTS idx_practitioner_subscription_status ON tbl_practitioner_subscription(status);
CREATE INDEX IF NOT EXISTS idx_practitioner_subscription_created_at ON tbl_practitioner_subscription(created_at);
CREATE INDEX IF NOT EXISTS idx_practitioner_subscription_practitioner_id ON tbl_practitioner_subscription(practitioner_id);
CREATE INDEX IF NOT EXISTS idx_practitioner_subscription_status_created_at ON tbl_practitioner_subscription(status, created_at);

-- User indexes for user growth analytics
CREATE INDEX IF NOT EXISTS idx_user_role ON tbl_user(role);
CREATE INDEX IF NOT EXISTS idx_user_created_at ON tbl_user(created_at);
CREATE INDEX IF NOT EXISTS idx_user_role_created_at ON tbl_user(role, created_at);

-- Clinic indexes for practitioner details
CREATE INDEX IF NOT EXISTS idx_clinic_practitioner_id ON tbl_clinic(practitioner_id);
CREATE INDEX IF NOT EXISTS idx_clinic_created_at ON tbl_clinic(created_at);

-- Subscription indexes for revenue analytics
CREATE INDEX IF NOT EXISTS idx_subscription_deleted_at ON tbl_subscription(deleted_at);

-- Practitioner indexes
CREATE INDEX IF NOT EXISTS idx_practitioner_deleted_at ON tbl_practitioner(deleted_at);

-- Accountant indexes
CREATE INDEX IF NOT EXISTS idx_accountant_deleted_at ON tbl_accountant(deleted_at);
CREATE INDEX IF NOT EXISTS idx_accountant_user_id ON tbl_accountant(user_id);

-- +goose Down
-- Remove analytics indexes
DROP INDEX IF EXISTS idx_audit_log_created_at;
DROP INDEX IF EXISTS idx_audit_log_entity_type;
DROP INDEX IF EXISTS idx_audit_log_user_id;
DROP INDEX IF EXISTS idx_audit_log_action;
DROP INDEX IF EXISTS idx_audit_log_entity_type_created_at;
DROP INDEX IF EXISTS idx_practitioner_subscription_status;
DROP INDEX IF EXISTS idx_practitioner_subscription_created_at;
DROP INDEX IF EXISTS idx_practitioner_subscription_practitioner_id;
DROP INDEX IF EXISTS idx_practitioner_subscription_status_created_at;
DROP INDEX IF EXISTS idx_user_role;
DROP INDEX IF EXISTS idx_user_created_at;
DROP INDEX IF EXISTS idx_user_role_created_at;
DROP INDEX IF EXISTS idx_clinic_practitioner_id;
DROP INDEX IF EXISTS idx_clinic_created_at;
DROP INDEX IF EXISTS idx_subscription_deleted_at;
DROP INDEX IF EXISTS idx_practitioner_deleted_at;
DROP INDEX IF EXISTS idx_accountant_deleted_at;
DROP INDEX IF EXISTS idx_accountant_user_id;

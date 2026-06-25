-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- FIX: Add deleted_at IS NULL to form_entry_value subqueries
-- Issue: Deleted entries were still appearing in reports because subqueries
-- were missing the deleted_at IS NULL filter
-- Solution: Drop and recreate views by re-running migration 20260619170358
-- ============================================================================

-- First, drop all views to allow recreation
DROP VIEW IF EXISTS vw_pl_fy_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_by_financial_year CASCADE;
DROP VIEW IF EXISTS vw_pl_by_responsibility CASCADE;
DROP VIEW IF EXISTS vw_pl_summary_monthly CASCADE;
DROP VIEW IF EXISTS vw_pl_by_account CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_double_entry_entry_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_line_items CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
DROP VIEW IF EXISTS vw_double_entry_line_items CASCADE;

-- Note: After running this migration, you need to manually reapply migration:
-- 20260619170358_consolidated_financial_reporting_views.sql
-- The views in that file have been updated with proper deleted_at checks

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rolling back will drop the views
DROP VIEW IF EXISTS vw_pl_fy_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_by_financial_year CASCADE;
DROP VIEW IF EXISTS vw_pl_by_responsibility CASCADE;
DROP VIEW IF EXISTS vw_pl_summary_monthly CASCADE;
DROP VIEW IF EXISTS vw_pl_by_account CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_summary CASCADE;
DROP VIEW IF EXISTS vw_bas_monthly CASCADE;
DROP VIEW IF EXISTS vw_bas_by_account CASCADE;
DROP VIEW IF EXISTS vw_bas_summary CASCADE;
DROP VIEW IF EXISTS vw_double_entry_entry_summary CASCADE;
DROP VIEW IF EXISTS vw_pl_line_items CASCADE;
DROP VIEW IF EXISTS vw_balance_sheet_line_items CASCADE;
DROP VIEW IF EXISTS vw_bas_line_items CASCADE;
DROP VIEW IF EXISTS vw_double_entry_line_items CASCADE;
-- +goose StatementEnd

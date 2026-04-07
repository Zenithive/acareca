-- +goose Up
-- +goose StatementBegin
ALTER TYPE invitation_status ADD VALUE IF NOT EXISTS 'REVOKED';
-- +goose StatementEnd

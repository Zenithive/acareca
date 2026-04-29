-- +goose Up
-- +goose StatementBegin
ALTER TYPE enum_shared_event_entity ADD VALUE IF NOT EXISTS 'REPORT';
-- +goose StatementEnd
-- +goose Up
-- +goose StatementBegin
ALTER TYPE enum_shared_event_entity ADD VALUE 'REPORT';
-- +goose StatementEnd
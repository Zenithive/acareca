-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invite_permissions;
DROP TYPE IF EXISTS invite_entity_type;

CREATE TABLE tbl_invite_permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    practitioner_id UUID NOT NULL REFERENCES tbl_practitioner(id),
    accountant_id UUID NOT NULL REFERENCES tbl_accountant(id),
    permission_name TEXT NOT NULL CHECK (
        permission_name IN (
            'sales_purchases',
            'lock_dates',
            'manage_users',
            'reports_view_download'
        )
    ),
    can_read BOOLEAN NOT NULL DEFAULT false,
    can_write BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (practitioner_id, accountant_id, permission_name)
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invite_permissions;
-- +goose StatementEnd
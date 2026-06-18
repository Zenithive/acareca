-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_chart_of_accounts_template (
    id UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    account_type_id SMALLINT NOT NULL REFERENCES tbl_account_type(id),
    account_tax_id SMALLINT NOT NULL REFERENCES tbl_account_tax(id),
    code SMALLINT NOT NULL CHECK (
        code >= 100
        AND code <= 9999
    ),
    name VARCHAR(255) NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    is_cos BOOLEAN,
    is_capital BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_by UUID REFERENCES tbl_admin(id),
    updated_by UUID REFERENCES tbl_admin(id),
    
    CONSTRAINT uq_chart_of_accounts_template_code UNIQUE(code)
);
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_chart_of_accounts_template;
-- +goose StatementEnd
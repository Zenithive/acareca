-- +goose Up
-- +goose StatementBegin
INSERT INTO tbl_account_type (name) VALUES
    ('Direct Cost'),
    ('Bank'),
    ('Other - ITR Reporting Item')
ON CONFLICT (name) DO NOTHING;

DROP TABLE IF EXISTS tbl_chart_of_accounts CASCADE;

CREATE TABLE tbl_chart_of_accounts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    practitioner_id UUID NOT NULL,
    template_id     UUID NULL REFERENCES tbl_chart_of_accounts_template(id),
    account_type_id INT2 NULL,                   
    account_tax_id  INT2 NULL,                   
    code            INT2 NULL,                   
    name            VARCHAR(255) NULL,           
    is_system       BOOLEAN NULL,  
    is_cos          BOOLEAN NULL, 
    is_capital      BOOLEAN NULL, 
    is_custom       BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ(6) NULL,
    deleted_at      TIMESTAMPTZ(6) NULL
);

CREATE INDEX IF NOT EXISTS idx_coa_template_id ON tbl_chart_of_accounts(template_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_coa_practitioner_template ON tbl_chart_of_accounts(practitioner_id, template_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_chart_of_accounts;

CREATE TABLE tbl_chart_of_accounts (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  practitioner_id uuid NOT NULL,
  account_type_id smallint NULL,
  account_tax_id smallint NULL,
  code smallint NULL,
  name character varying(255) NULL,
  is_system boolean NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL,
  deleted_at timestamp with time zone NULL,
  key character varying(255) NULL,
  classification account_classification NULL,
  template_id uuid NULL,
  is_custom boolean NOT NULL DEFAULT false
);

DELETE FROM tbl_account_type 
WHERE name IN ('Direct Cost', 'Bank', 'Other - ITR Reporting Item');

-- +goose StatementEnd
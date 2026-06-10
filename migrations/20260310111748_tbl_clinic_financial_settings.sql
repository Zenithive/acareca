-- +goose Up
CREATE TABLE IF NOT EXISTS tbl_financial_settings (
    id                  UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(), 
    clinic_id           UUID NOT NULL REFERENCES tbl_clinic(id),
    financial_year_id   UUID NOT NULL REFERENCES tbl_financial_year(id), -- example: ab290cajdh
    lock_date           DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS tbl_financial_settings;

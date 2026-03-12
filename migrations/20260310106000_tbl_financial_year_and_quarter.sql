-- +goose Up
-- +goose StatementBegin
CREATE TABLE tbl_financial_year (
    id              UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    label           TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_active       BOOLEAN NOT NULL DEFAULT FALSE,
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    EXCLUDE USING gist (
        daterange(start_date, end_date, '[]') WITH &&
    )
);

CREATE TABLE tbl_financial_quarter (
    id                  UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    financial_year_id   UUID REFERENCES tbl_financial_year(id),
    label               TEXT NOT NULL CHECK (label IN ('Q1', 'Q2', 'Q3', 'Q4')),
    start_date          DATE NOT NULL,
    end_date            DATE NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_financial_quarter CASCADE;
DROP TABLE IF EXISTS tbl_financial_year CASCADE;
-- +goose StatementEnd

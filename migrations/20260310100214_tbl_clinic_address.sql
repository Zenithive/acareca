-- +goose Up
CREATE TABLE IF NOT EXISTS tbl_clinic_address (
   id               UUID PRIMARY KEY NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
   clinic_id        UUID NOT NULL REFERENCES tbl_clinic(id),
   address          TEXT,
   city             TEXT,
   state            TEXT,
   postcode         VARCHAR(4),
   is_primary       BOOLEAN NOT NULL DEFAULT FALSE,
   created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
   updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS tbl_clinic_address;
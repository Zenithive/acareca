-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field
ADD COLUMN key VARCHAR(5) NOT NULL,
ADD COLUMN slug VARCHAR(100),
ADD COLUMN is_computed BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX uniq_form_field_key
ON tbl_form_field(form_version_id, key);

CREATE TABLE tbl_formula ( 
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    form_version_id UUID NOT NULL REFERENCES tbl_custom_form_version(id) ON DELETE CASCADE,
    field_id UUID NOT NULL REFERENCES tbl_form_field(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,

    created_at TIMESTAMPTZ DEFAULT now(),

    UNIQUE(form_version_id, field_id)
);

CREATE TYPE node_type AS ENUM ('OPERATOR', 'FIELD', 'CONSTANT');

CREATE TABLE tbl_formula_node (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    formula_id UUID NOT NULL REFERENCES tbl_formula(id) ON DELETE CASCADE,

    parent_id UUID NULL REFERENCES tbl_formula_node(id) ON DELETE CASCADE,

    node_type node_type NOT NULL,

    operator VARCHAR(5),        -- + - * /
    field_id UUID,              -- reference field
    constant_value NUMERIC(12,4),

    position SMALLINT NOT NULL, -- 0 = left, 1 = right

    created_at TIMESTAMPTZ DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd

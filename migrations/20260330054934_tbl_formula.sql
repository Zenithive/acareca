-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_form_field
ADD COLUMN field_key VARCHAR(5) NOT NULL DEFAULT '',
ADD COLUMN slug VARCHAR(100),
ADD COLUMN is_computed BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX uniq_form_field_key
ON tbl_form_field(form_version_id, field_key);

CREATE TABLE tbl_formula (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    form_version_id UUID NOT NULL REFERENCES tbl_custom_form_version(id) ON DELETE CASCADE,
    field_id UUID NOT NULL REFERENCES tbl_form_field(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,

    created_at TIMESTAMPTZ DEFAULT now(),

    UNIQUE(form_version_id, field_id)
);

CREATE TYPE formula_node_type AS ENUM ('OPERATOR', 'FIELD', 'CONSTANT');

CREATE TABLE tbl_formula_node (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    formula_id UUID NOT NULL REFERENCES tbl_formula(id) ON DELETE CASCADE,

    parent_id UUID NULL REFERENCES tbl_formula_node(id) ON DELETE CASCADE,

    node_type formula_node_type NOT NULL,

    operator VARCHAR(5),        -- + - * /
    field_id UUID,              -- reference field
    constant_value NUMERIC(12,4),

    position SMALLINT,          -- 0 = left, 1 = right (NULL for root node)

    created_at TIMESTAMPTZ DEFAULT now(),

    CONSTRAINT chk_node_type_fields CHECK (
        (node_type = 'OPERATOR' AND operator IS NOT NULL AND field_id IS NULL AND constant_value IS NULL) OR
        (node_type = 'FIELD'    AND field_id IS NOT NULL AND operator IS NULL AND constant_value IS NULL) OR
        (node_type = 'CONSTANT' AND constant_value IS NOT NULL AND operator IS NULL AND field_id IS NULL)
    ),
    CONSTRAINT chk_position CHECK (
        (parent_id IS NULL AND position IS NULL) OR
        (parent_id IS NOT NULL AND position IS NOT NULL)
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_formula_node;
DROP TYPE IF EXISTS formula_node_type;
DROP TABLE IF EXISTS tbl_formula;
DROP INDEX IF EXISTS uniq_form_field_key;
ALTER TABLE tbl_form_field
    DROP COLUMN IF EXISTS field_key,
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS is_computed;
-- +goose StatementEnd

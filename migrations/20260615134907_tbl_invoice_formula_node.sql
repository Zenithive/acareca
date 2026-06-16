-- +goose Up
-- +goose StatementBegin

CREATE TYPE invoice_formula_node_type AS ENUM ('OPERATOR', 'FIELD', 'CONSTANT');

CREATE TABLE tbl_invoice_formula_node (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    invoice_item_id UUID NOT NULL REFERENCES tbl_invoice_item(id) ON DELETE CASCADE,

    parent_id UUID NULL REFERENCES tbl_invoice_formula_node(id) ON DELETE CASCADE,

    node_type invoice_formula_node_type NOT NULL,

    operator VARCHAR(5),        -- + - * /
    item_id UUID,              -- reference field
    constant_value NUMERIC(12,4),

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
DROP TABLE IF EXISTS tbl_invoice_formula_node;
-- +goose StatementEnd

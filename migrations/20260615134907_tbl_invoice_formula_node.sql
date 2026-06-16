-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'invoice_formula_node_type') THEN
        CREATE TYPE invoice_formula_node_type AS ENUM ('OPERATOR', 'FIELD_REFERENCE', 'CONSTANT');
    END IF;
END$$;

-- Table 1: Base formula mapping for an invoice section field key
CREATE TABLE tbl_invoice_formula (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES tbl_invoice(id) ON DELETE CASCADE,
    invoice_section_id UUID NOT NULL REFERENCES tbl_map_invoice_section(id) ON DELETE CASCADE,
    
    -- Target variables matching requirements (e.g., "G1", "1A", "G3", "NET_PATIENT", "SFA_FEE", "1B", "G11")
    field_key VARCHAR(50) NOT NULL,
    label VARCHAR(255) NULL, 
    
    -- Sorting and configuration metadata
    is_percentage BOOLEAN DEFAULT FALSE,
    is_negative_display BOOLEAN DEFAULT FALSE,
    sort_order INT NOT NULL,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_section_formula_key UNIQUE (invoice_section_id, field_key)
);

CREATE INDEX idx_invoice_formulas_seq ON tbl_invoice_formula (invoice_section_id, sort_order);

-- Table 2: Tree-structured AST nodes matching the system schema syntax
CREATE TABLE tbl_invoice_formula_node (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_formula_id UUID NOT NULL REFERENCES tbl_invoice_formula(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES tbl_invoice_formula_node(id) ON DELETE CASCADE,
    
    node_type invoice_formula_node_type NOT NULL,
    operator VARCHAR(5) DEFAULT NULL,             -- e.g., '+', '-', '*', '/'
    referenced_field_key VARCHAR(50) DEFAULT NULL, -- Points to dependent field_key variable
    constant_value NUMERIC(15, 4) DEFAULT NULL,    -- Used if node_type = 'CONSTANT'
    
    position SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_invoice_formula_tree ON tbl_invoice_formula_node (invoice_formula_id, parent_id, position);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_invoice_formula_node;
DROP TABLE IF EXISTS tbl_invoice_formula;
-- +goose StatementEnd
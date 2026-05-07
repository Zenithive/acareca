-- +goose Up
-- +goose StatementBegin
CREATE TABLE tbl_map_entry_document (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    entry_id UUID NOT NULL
        REFERENCES tbl_form_entry(id),
    document_id UUID NOT NULL
        REFERENCES tbl_document(id),

    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tbl_map_entry_document_entry_id_document_id_key UNIQUE(entry_id, document_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_map_entry_document;
-- +goose StatementEnd

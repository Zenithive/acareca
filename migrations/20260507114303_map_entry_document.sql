-- +goose Up
-- +goose StatementBegin
CREATE TABLE tbl_map_entry_document (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    form_entry_id UUID NOT NULL
        REFERENCES tbl_form_entry(id),
    document_id UUID NOT NULL
        REFERENCES tbl_document(id),

    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(form_entry_id, document_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd

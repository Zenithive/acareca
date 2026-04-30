-- +goose Up
-- +goose StatementBegin
ALTER TABLE tbl_invite_permissions 
ADD COLUMN deleted_at TIMESTAMP NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tbl_invite_permissions 
DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd

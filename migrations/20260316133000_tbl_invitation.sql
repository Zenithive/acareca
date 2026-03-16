-- +goose Up
CREATE TYPE invitation_status AS ENUM ('SENT', 'ACCEPTED', 'COMPLETED', 'REJECTED');

CREATE TABLE tbl_invitation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practitioner_id UUID NOT NULL,
    entity_id UUID NULL,
    email VARCHAR(255) NOT NULL,
    status invitation_status NOT NULL DEFAULT 'SENT',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE tbl_invitation;
DROP TYPE invitation_status;
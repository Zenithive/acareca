-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tbl_session (
    id            VARCHAR(40)  PRIMARY KEY NOT NULL UNIQUE,

    user_id       VARCHAR(40)  NOT NULL REFERENCES tbl_user(id),

    refresh_token TEXT         NOT NULL,

    user_agent    TEXT,
    ip_address    VARCHAR(45),

    expires_at    TIMESTAMPTZ  NOT NULL,

    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    deleted_at    TIMESTAMPTZ  NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tbl_session;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS users (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sub        TEXT        NOT NULL UNIQUE,
    email      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE pastes
    ADD CONSTRAINT pastes_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES users(id);

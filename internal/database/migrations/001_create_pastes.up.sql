CREATE TABLE IF NOT EXISTS pastes (
    id         BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT        NOT NULL UNIQUE,
    content    TEXT        NOT NULL,
    language   VARCHAR(64) NOT NULL DEFAULT 'plaintext',
    title      VARCHAR(255),
    size_bytes INTEGER     NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    owner_id   BIGINT,

    CONSTRAINT size_positive         CHECK (size_bytes >= 1),
    CONSTRAINT key_length            CHECK (char_length(key) BETWEEN 10 AND 32),
    CONSTRAINT expiry_after_creation CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS pastes_expires_at_idx ON pastes (expires_at);
CREATE INDEX IF NOT EXISTS pastes_owner_id_idx   ON pastes (owner_id);

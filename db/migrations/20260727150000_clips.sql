-- +goose Up
CREATE TABLE clips (
    slug TEXT PRIMARY KEY,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX clips_expires_at_idx ON clips (expires_at);

-- +goose Down
DROP TABLE IF EXISTS clips;

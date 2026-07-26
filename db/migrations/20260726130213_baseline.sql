-- +goose Up
-- Domain-neutral baseline: initializes goose version tracking without app tables.
SELECT 1;

-- +goose Down
SELECT 1;

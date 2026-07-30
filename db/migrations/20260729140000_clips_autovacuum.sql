-- +goose Up
ALTER TABLE clips SET (
    autovacuum_vacuum_scale_factor = 0,
    autovacuum_vacuum_threshold = 50,
    autovacuum_analyze_scale_factor = 0,
    autovacuum_analyze_threshold = 50
);

-- +goose Down
ALTER TABLE clips RESET (
    autovacuum_vacuum_scale_factor,
    autovacuum_vacuum_threshold,
    autovacuum_analyze_scale_factor,
    autovacuum_analyze_threshold
);

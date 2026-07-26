-- name: ServerTime :one
SELECT NOW()::timestamptz AS server_time;

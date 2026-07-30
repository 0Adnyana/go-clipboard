-- name: GetLiveClip :one
SELECT slug, body, created_at, expires_at
FROM clips
WHERE slug = $1
  AND expires_at > now()
LIMIT 1;

-- name: ClaimClip :one
INSERT INTO clips (slug, body, created_at, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (slug) DO UPDATE
SET body = EXCLUDED.body,
    created_at = EXCLUDED.created_at,
    expires_at = EXCLUDED.expires_at
WHERE clips.expires_at <= now()
RETURNING slug, body, created_at, expires_at;

-- name: DeleteExpiredClips :execrows
DELETE FROM clips
WHERE expires_at <= now();

-- name: SlugIsLive :one
SELECT EXISTS(
    SELECT 1
    FROM clips
    WHERE slug = $1
      AND expires_at > now()
) AS live;

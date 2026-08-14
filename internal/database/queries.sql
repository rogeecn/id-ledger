-- name: InsertCollectedID :execrows
INSERT INTO collected_ids (project_key, remote_id, created_at)
VALUES (sqlc.arg(project_key), sqlc.arg(remote_id), sqlc.arg(created_at))
ON CONFLICT (project_key, remote_id) DO NOTHING;

-- name: ListCollectedIDs :many
SELECT remote_id, created_at
FROM collected_ids
WHERE project_key = sqlc.arg(project_key)
  AND created_at >= sqlc.arg(since)
  AND created_at < sqlc.arg(until)
ORDER BY created_at DESC, remote_id DESC
LIMIT sqlc.arg(page_size);
-- name: ListCollectedIDsAfter :many
SELECT remote_id, created_at
FROM collected_ids
WHERE project_key = sqlc.arg(project_key)
  AND created_at >= sqlc.arg(since)
  AND created_at < sqlc.arg(until)
  AND (
    created_at < sqlc.arg(cursor_created_at)
    OR (created_at = sqlc.arg(cursor_created_at) AND remote_id < sqlc.arg(cursor_remote_id))
  )
ORDER BY created_at DESC, remote_id DESC
LIMIT sqlc.arg(page_size);

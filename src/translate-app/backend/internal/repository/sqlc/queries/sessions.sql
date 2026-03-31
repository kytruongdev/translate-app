-- name: GetSessions :many
SELECT * FROM sessions
WHERE status NOT IN ('deleted', 'archived')
ORDER BY
  CASE WHEN status = 'pinned' THEN 0 ELSE 1 END ASC,
  updated_at DESC;

-- name: CreateSession :exec
INSERT INTO sessions (
  id, title, status, target_lang, style, model, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateSessionTitle :exec
UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?;

-- name: UpdateSessionStatus :exec
UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?;

-- name: UpdateSessionTargetLang :exec
UPDATE sessions SET target_lang = ?, updated_at = ? WHERE id = ?;

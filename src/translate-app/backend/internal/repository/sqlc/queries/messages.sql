-- name: GetMessagesBySessionCursor :many
SELECT * FROM messages
WHERE session_id = sqlc.arg(session_id)
  AND (sqlc.arg(cursor) = 0 OR display_order < sqlc.arg(cursor_before))
ORDER BY display_order DESC
LIMIT sqlc.arg(row_limit);

-- name: GetMaxDisplayOrder :one
SELECT COALESCE(MAX(display_order), 0) AS max_order FROM messages WHERE session_id = ?;

-- name: InsertMessage :exec
INSERT INTO messages (
  id, session_id, role, display_order, display_mode,
  original_content, translated_content, file_id, source_lang, target_lang,
  style, model_used, original_message_id, tokens, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateMessageTranslated :exec
UPDATE messages
SET translated_content = ?, tokens = ?, updated_at = ?
WHERE id = ?;

-- name: GetMessageById :one
SELECT * FROM messages WHERE id = ? LIMIT 1;

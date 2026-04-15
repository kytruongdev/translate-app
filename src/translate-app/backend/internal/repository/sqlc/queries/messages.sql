-- NOTE: GetMessagesBySessionCursor uses dynamic cursor logic and is implemented
-- as raw SQL in message_repo.go - not generated via SQLC.

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

-- name: UpdateMessageOriginalContent :exec
UPDATE messages
SET original_content = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateMessageSourceLang :exec
UPDATE messages SET source_lang = ?, updated_at = ? WHERE id = ?;

-- name: DeleteMessagesByFileID :exec
DELETE FROM messages WHERE file_id = ?;

-- name: GetMessageById :one
SELECT m.*, COALESCE(f.file_size, 0) AS file_size
FROM messages m
LEFT JOIN files f ON f.id = m.file_id
WHERE m.id = ? LIMIT 1;

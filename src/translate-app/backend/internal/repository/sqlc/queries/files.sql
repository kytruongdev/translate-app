-- name: InsertFile :exec
INSERT INTO files (
  id, session_id, file_name, file_type, file_size, original_path, source_path, translated_path,
  char_count, page_count, style, model_used, status, error_msg, output_format, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateFileStatus :exec
UPDATE files SET status = ?, error_msg = ?, updated_at = ? WHERE id = ?;

-- name: UpdateFileTranslated :exec
UPDATE files
SET source_path = ?, translated_path = ?, status = ?, char_count = ?, page_count = ?, model_used = ?, output_format = ?, updated_at = ?
WHERE id = ?;

-- name: UpdateFileExtracted :exec
UPDATE files
SET source_path = ?, char_count = ?, page_count = ?, updated_at = ?
WHERE id = ?;

-- name: GetFileById :one
SELECT * FROM files WHERE id = ? LIMIT 1;

-- name: DeleteFileByID :exec
DELETE FROM files WHERE id = ?;

-- name: GetCancelledFileIdsBySession :many
SELECT id FROM files WHERE session_id = ? AND status = 'cancelled';

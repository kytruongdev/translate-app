-- name: ListDocTypes :many
SELECT id, name FROM doc_types ORDER BY name;

-- name: InsertDocType :exec
INSERT OR IGNORE INTO doc_types (id, name) VALUES (?, ?);

-- name: CreateGlossaryEntry :exec
INSERT INTO glossary_entries (id, source_lang, target_lang, target, doc_type, status, current_file_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetGlossaryEntry :one
SELECT * FROM glossary_entries WHERE id = ? LIMIT 1;

-- name: ListGlossaryEntries :many
SELECT * FROM glossary_entries ORDER BY created_at DESC;

-- name: UpdateGlossaryEntryStatus :exec
UPDATE glossary_entries SET status = ?, updated_at = ? WHERE id = ?;

-- name: DeleteGlossaryEntry :exec
DELETE FROM glossary_entries WHERE id = ?;

-- name: CreateGlossaryVariant :exec
INSERT OR IGNORE INTO glossary_variants (id, entry_id, source) VALUES (?, ?, ?);

-- name: ListVariantsByEntry :many
SELECT * FROM glossary_variants WHERE entry_id = ? ORDER BY source;

-- name: DeleteVariantsByEntry :exec
DELETE FROM glossary_variants WHERE entry_id = ?;

-- FindEntryBySourceAndDocType finds a glossary entry matching a source variant and doc_type.
-- Used to check for duplicates before inserting.
-- name: FindEntryBySourceAndDocType :one
SELECT e.id, e.source_lang, e.target_lang, e.target, e.doc_type, e.status, e.current_file_name, e.created_at, e.updated_at
FROM glossary_entries e
JOIN glossary_variants v ON v.entry_id = e.id
WHERE v.source = ?
  AND e.source_lang = ?
  AND e.target_lang = ?
  AND e.doc_type = ?
LIMIT 1;

-- name: SetGlossaryCurrentFile :exec
UPDATE glossary_entries SET current_file_name = ?, updated_at = ? WHERE id = ?;

-- name: ClearFileGlossary :exec
UPDATE glossary_entries SET current_file_name = NULL, updated_at = ?
WHERE current_file_name = ?;

-- name: LoadGlossaryForFile :many
SELECT v.source, e.target
FROM glossary_variants v
JOIN glossary_entries e ON v.entry_id = e.id
WHERE e.current_file_name = ?
ORDER BY e.id, v.source;

-- name: CreateTranslationRule :exec
INSERT INTO translation_rules (id, name, content, doc_type, target_lang, enabled, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListTranslationRules :many
SELECT * FROM translation_rules ORDER BY sort_order, created_at;

-- name: LoadActiveRules :many
SELECT * FROM translation_rules
WHERE enabled = 1
  AND (doc_type IS NULL OR doc_type = ?)
  AND (target_lang IS NULL OR target_lang = ?)
ORDER BY sort_order;

-- name: UpdateTranslationRule :exec
UPDATE translation_rules
SET name = ?, content = ?, doc_type = ?, target_lang = ?, enabled = ?, sort_order = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteTranslationRule :exec
DELETE FROM translation_rules WHERE id = ?;

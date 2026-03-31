-- Migration 002: Add 'file' display_mode for DOCX file translation messages.
--
-- SQLite does not support ALTER COLUMN or DROP CONSTRAINT, so we rebuild
-- the messages table with the updated CHECK constraint.
--
-- Existing bilingual messages that are linked to a file (file_id IS NOT NULL)
-- are migrated to display_mode = 'file'.

PRAGMA foreign_keys = OFF;

CREATE TABLE messages_new (
    id                  TEXT PRIMARY KEY,
    session_id          TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role                TEXT NOT NULL CHECK (role IN ('user','assistant')),
    display_order       INTEGER NOT NULL,
    display_mode        TEXT NOT NULL DEFAULT 'bubble' CHECK (display_mode IN ('bubble','bilingual','file')),
    original_content    TEXT NOT NULL DEFAULT '',
    translated_content  TEXT,
    file_id             TEXT REFERENCES files(id) ON DELETE SET NULL,
    source_lang         TEXT,
    target_lang         TEXT,
    style               TEXT CHECK (style IN ('casual','business','academic')),
    model_used          TEXT,
    original_message_id TEXT REFERENCES messages(id),
    tokens              INTEGER DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

INSERT INTO messages_new
SELECT
    id,
    session_id,
    role,
    display_order,
    CASE
        WHEN file_id IS NOT NULL AND display_mode = 'bilingual' THEN 'file'
        ELSE display_mode
    END,
    original_content,
    translated_content,
    file_id,
    source_lang,
    target_lang,
    style,
    model_used,
    original_message_id,
    tokens,
    created_at,
    updated_at
FROM messages;

DROP TABLE messages;
ALTER TABLE messages_new RENAME TO messages;

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_order   ON messages(session_id, display_order);
CREATE INDEX        IF NOT EXISTS idx_messages_session ON messages(session_id, display_order);

PRAGMA foreign_keys = ON;

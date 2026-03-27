-- Migration 003: Add 'cancelled' to files.status CHECK constraint.
-- SQLite does not support ALTER TABLE ... MODIFY CONSTRAINT, so we recreate the table.

PRAGMA foreign_keys=OFF;

CREATE TABLE files_new (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    file_name       TEXT NOT NULL,
    file_type       TEXT NOT NULL CHECK (file_type IN ('pdf','docx')),
    file_size       INTEGER NOT NULL DEFAULT 0,
    original_path   TEXT,
    source_path     TEXT,
    translated_path TEXT,
    char_count      INTEGER DEFAULT 0,
    page_count      INTEGER DEFAULT 0,
    style           TEXT CHECK (style IN ('casual','business','academic')),
    model_used      TEXT,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','error','cancelled')),
    error_msg       TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

INSERT INTO files_new SELECT * FROM files;
DROP TABLE files;
ALTER TABLE files_new RENAME TO files;
CREATE INDEX IF NOT EXISTS idx_files_session ON files(session_id);

PRAGMA foreign_keys=ON;

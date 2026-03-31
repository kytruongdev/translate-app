-- sessions
CREATE TABLE IF NOT EXISTS sessions (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','pinned','archived','deleted')),
    target_lang      TEXT,
    style            TEXT CHECK (style IN ('casual','business','academic')),
    model            TEXT,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

-- files (before messages: messages.file_id → files)
CREATE TABLE IF NOT EXISTS files (
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
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','error')),
    error_msg       TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_files_session ON files(session_id);

-- messages
CREATE TABLE IF NOT EXISTS messages (
    id                  TEXT PRIMARY KEY,
    session_id          TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role                TEXT NOT NULL CHECK (role IN ('user','assistant')),
    display_order       INTEGER NOT NULL,
    display_mode        TEXT NOT NULL DEFAULT 'bubble' CHECK (display_mode IN ('bubble','bilingual')),
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_order ON messages(session_id, display_order);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, display_order);

-- settings
CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

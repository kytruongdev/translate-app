-- Migration 009: Add glossary and translation rules tables.

CREATE TABLE doc_types (
    id   TEXT PRIMARY KEY,  -- e.g. "legal", "real_estate", "tax"
    name TEXT NOT NULL      -- e.g. "Legal / Notary", "Real Estate", "Tax"
);

CREATE TABLE glossary_entries (
    id          TEXT PRIMARY KEY,
    source_lang TEXT NOT NULL,
    target_lang TEXT NOT NULL,
    target      TEXT NOT NULL,
    doc_type    TEXT REFERENCES doc_types(id) ON DELETE SET NULL,  -- null = global
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'approved')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX idx_glossary_entries_lang ON glossary_entries(source_lang, target_lang);
CREATE INDEX idx_glossary_entries_status ON glossary_entries(status);

CREATE TABLE glossary_variants (
    id       TEXT PRIMARY KEY,
    entry_id TEXT NOT NULL REFERENCES glossary_entries(id) ON DELETE CASCADE,
    source   TEXT NOT NULL,
    UNIQUE (source, entry_id)
);

CREATE INDEX idx_glossary_variants_entry ON glossary_variants(entry_id);

CREATE TABLE translation_rules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    content     TEXT NOT NULL,
    doc_type    TEXT REFERENCES doc_types(id) ON DELETE SET NULL,  -- null = global
    target_lang TEXT,                                              -- null = all languages
    enabled     INTEGER NOT NULL DEFAULT 1,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX idx_translation_rules_enabled ON translation_rules(enabled, sort_order);

-- Initial doc_types
INSERT INTO doc_types (id, name) VALUES
    ('legal',       'Legal / Notary'),
    ('real_estate', 'Real Estate'),
    ('tax',         'Tax');

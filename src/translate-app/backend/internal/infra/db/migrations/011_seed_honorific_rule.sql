-- Migration 011: Seed default translation rule for Vietnamese honorifics.
-- Applies globally (doc_type = NULL, target_lang = NULL) so it fires for all
-- document types and all target languages when translating PDF files.

INSERT OR IGNORE INTO translation_rules (id, name, content, doc_type, target_lang, enabled, sort_order, created_at, updated_at)
VALUES (
    'rule-honorific-vn',
    'Vietnamese Honorifics',
    'HONORIFICS:
- "Ông" → "Mr." (male title)
- "Bà" → "Ms." (female title)
Apply these mappings wherever the titles appear, including inside tables and form fields.',
    NULL,
    NULL,
    1,
    10,
    datetime('now'),
    datetime('now')
);

-- Migration 010: Add current_file_name to glossary_entries for per-file glossary scoping.
-- This field temporarily tags glossary entries to a specific file during translation,
-- so batch prompts only load terms relevant to the current file (not all approved terms).
-- After translation completes, current_file_name is reset to NULL.

ALTER TABLE glossary_entries ADD COLUMN current_file_name TEXT;

CREATE INDEX idx_glossary_entries_file ON glossary_entries(current_file_name);

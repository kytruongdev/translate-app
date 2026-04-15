-- Migration 008: Add output_format to files table to support .html export for structured PDF.

ALTER TABLE files ADD COLUMN output_format TEXT NOT NULL DEFAULT 'docx';

-- All existing files will have 'docx' by default. 
-- New structured PDF translations will update this to 'html'.

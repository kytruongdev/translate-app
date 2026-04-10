package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"translate-app/internal/repository/sqlcgen"
)

// GlossaryRepo manages glossary entries, variants, and doc types.
type GlossaryRepo interface {
	// ListDocTypes returns all known document types as a comma-separated string.
	ListDocTypeIDs(ctx context.Context) ([]string, error)

	// EnsureDocType inserts a new doc_type if it doesn't already exist.
	EnsureDocType(ctx context.Context, id, name string) error

	// SaveExtractedGlossary upserts a list of extracted glossary terms for a given
	// doc_type and file. Existing entries (matched by source + doc_type) get their
	// current_file_name updated. New entries are inserted as approved.
	SaveExtractedGlossary(ctx context.Context, sourceLang, targetLang, docType, fileName string, terms []GlossaryTerm) error

	// LoadGlossaryForFile returns all glossary entries tagged to the given file name
	// as a formatted string ready for prompt injection.
	LoadGlossaryForFile(ctx context.Context, fileName string) (string, error)

	// ClearFileGlossary removes the current_file_name tag from all entries
	// associated with the given file name after translation completes.
	ClearFileGlossary(ctx context.Context, fileName string) error

	// LoadActiveRules returns all enabled translation rules for the given
	// doc_type and target_lang, concatenated as a single rules string ready
	// for prompt injection. Rules with NULL doc_type/target_lang match any value.
	LoadActiveRules(ctx context.Context, docType, targetLang string) (string, error)
}

// GlossaryTerm is a single extracted glossary entry with one or more source variants.
type GlossaryTerm struct {
	Sources []string // Vietnamese variants (e.g. ["UBND", "Ủy ban nhân dân"])
	Target  string   // English translation
}

type glossaryRepo struct {
	q *sqlcgen.Queries
}

func (r *glossaryRepo) ListDocTypeIDs(ctx context.Context) ([]string, error) {
	rows, err := r.q.ListDocTypes(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (r *glossaryRepo) EnsureDocType(ctx context.Context, id, name string) error {
	// Convert snake_case id to Title Case name if name not provided.
	if name == "" {
		words := strings.Split(id, "_")
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		name = strings.Join(words, " ")
	}
	return r.q.InsertDocType(ctx, sqlcgen.InsertDocTypeParams{
		ID:   id,
		Name: name,
	})
}

func (r *glossaryRepo) SaveExtractedGlossary(ctx context.Context, sourceLang, targetLang, docType, fileName string, terms []GlossaryTerm) error {
	now := time.Now().UTC().Format(time.RFC3339)
	nullFileName := sql.NullString{String: fileName, Valid: fileName != ""}
	nullDocType := sql.NullString{String: docType, Valid: docType != ""}

	for _, term := range terms {
		if len(term.Sources) == 0 || strings.TrimSpace(term.Target) == "" {
			continue
		}

		// Check if any variant already exists for this doc_type.
		var existingID string
		for _, src := range term.Sources {
			row, err := r.q.FindEntryBySourceAndDocType(ctx, sqlcgen.FindEntryBySourceAndDocTypeParams{
				Source:     src,
				SourceLang: sourceLang,
				TargetLang: targetLang,
				DocType:    sql.NullString{String: docType, Valid: docType != ""},
			})
			if err == nil {
				existingID = row.ID
				break
			}
		}

		if existingID != "" {
			// Entry exists — just update current_file_name.
			if err := r.q.SetGlossaryCurrentFile(ctx, sqlcgen.SetGlossaryCurrentFileParams{
				CurrentFileName: nullFileName,
				UpdatedAt:       now,
				ID:              existingID,
			}); err != nil {
				return err
			}
		} else {
			// New entry — insert with status=approved.
			entryID := uuid.New().String()
			if err := r.q.CreateGlossaryEntry(ctx, sqlcgen.CreateGlossaryEntryParams{
				ID:              entryID,
				SourceLang:      sourceLang,
				TargetLang:      targetLang,
				Target:          strings.TrimSpace(term.Target),
				DocType:         nullDocType,
				Status:          "approved",
				CurrentFileName: nullFileName,
				CreatedAt:       now,
				UpdatedAt:       now,
			}); err != nil {
				return err
			}
			// Insert all source variants.
			for _, src := range term.Sources {
				src = strings.TrimSpace(src)
				if src == "" {
					continue
				}
				_ = r.q.CreateGlossaryVariant(ctx, sqlcgen.CreateGlossaryVariantParams{
					ID:      uuid.New().String(),
					EntryID: entryID,
					Source:  src,
				})
			}
		}
	}
	return nil
}

func (r *glossaryRepo) LoadGlossaryForFile(ctx context.Context, fileName string) (string, error) {
	rows, err := r.q.LoadGlossaryForFile(ctx, sql.NullString{String: fileName, Valid: true})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for _, row := range rows {
		sb.WriteString(row.Source)
		sb.WriteString(" → ")
		sb.WriteString(row.Target)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func (r *glossaryRepo) ClearFileGlossary(ctx context.Context, fileName string) error {
	return r.q.ClearFileGlossary(ctx, sqlcgen.ClearFileGlossaryParams{
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
		CurrentFileName: sql.NullString{String: fileName, Valid: true},
	})
}

func (r *glossaryRepo) LoadActiveRules(ctx context.Context, docType, targetLang string) (string, error) {
	rows, err := r.q.LoadActiveRules(ctx, sqlcgen.LoadActiveRulesParams{
		DocType:    sql.NullString{String: docType, Valid: docType != ""},
		TargetLang: sql.NullString{String: targetLang, Valid: targetLang != ""},
	})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	var parts []string
	for _, row := range rows {
		if strings.TrimSpace(row.Content) != "" {
			parts = append(parts, strings.TrimSpace(row.Content))
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

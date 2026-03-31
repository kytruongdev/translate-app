package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open opens SQLite at UserConfigDir/TranslateApp/data.db, runs migrations, seeds defaults.
func Open() (*sql.DB, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(dir, "TranslateApp")
	filesDir := filepath.Join(appDir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(appDir, "data.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := seedSettings(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func runMigrations(db *sql.DB) error {
	// Ensure migration tracking table exists.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename   TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var count int
		_ = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, name).Scan(&count)
		if count > 0 {
			continue // already applied
		}

		body, err := migrationFS.ReadFile(path.Join("migrations", name))
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}

		if _, err := db.Exec(
			`INSERT INTO schema_migrations (filename, applied_at) VALUES (?, ?)`,
			name, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

func seedSettings(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	defaults := map[string]string{
		"theme":            "system",
		"active_provider":  "openai",
		"active_model":     "gpt-4o-mini",
		"active_style":     "casual",
		"last_target_lang": "en-US",
	}
	for k, v := range defaults {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES (?, ?, ?)`,
			k, v, now,
		); err != nil {
			return err
		}
	}
	return nil
}

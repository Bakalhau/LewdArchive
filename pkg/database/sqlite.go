package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func NewSQLite(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_url TEXT NOT NULL,
		entry_id INTEGER NOT NULL,
		hash TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		published_at DATETIME NOT NULL,
		content TEXT,
		author TEXT,
		category_id INTEGER,
		category_title TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_posts_hash ON posts(hash);
	CREATE INDEX IF NOT EXISTS idx_posts_url ON posts(url);
	CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts(published_at);
	CREATE INDEX IF NOT EXISTS idx_posts_author ON posts(author);
	`

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}
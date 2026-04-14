// Package db owns the SQLite connection and schema lifecycle for strands.
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

const (
	// DirName is the per-repo directory that holds strands state.
	DirName = ".strands"
	// DBName is the SQLite file inside DirName.
	DBName = "strands.db"
	// currentSchemaVersion tracks the schema as of this binary.
	currentSchemaVersion = 1
)

// Paths resolves the strands directory and database file relative to root.
// root is typically the current working directory.
func Paths(root string) (dir, dbPath string) {
	dir = filepath.Join(root, DirName)
	dbPath = filepath.Join(dir, DBName)
	return
}

// Exists reports whether a strands db already lives under root.
func Exists(root string) bool {
	_, dbPath := Paths(root)
	_, err := os.Stat(dbPath)
	return err == nil
}

// Open opens (but does not create) an existing strands db under root.
// It enforces foreign keys and returns a ready-to-use *sql.DB.
func Open(root string) (*sql.DB, error) {
	_, dbPath := Paths(root)
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("strands db not found at %s: run 'strands init' first", dbPath)
	}
	return openDSN(dbPath)
}

// Init creates the .strands directory, opens the db file (creating it if
// needed), runs the embedded schema, and stamps the schema_version row.
// Returns an error if the db already exists — strands init is not idempotent
// on purpose, so users notice an accidental re-init.
func Init(root string) (string, error) {
	dir, dbPath := Paths(root)

	if _, err := os.Stat(dbPath); err == nil {
		return "", fmt.Errorf("strands already initialized at %s", dbPath)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}

	conn, err := openDSN(dbPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Exec(schemaSQL); err != nil {
		return "", fmt.Errorf("apply schema: %w", err)
	}

	_, err = conn.Exec(
		`INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`,
		currentSchemaVersion,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("stamp schema version: %w", err)
	}

	return dbPath, nil
}

func openDSN(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return conn, nil
}

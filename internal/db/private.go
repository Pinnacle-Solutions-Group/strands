package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Pinnacle-Solutions-Group/strands/internal/ids"
)

// ErrAlreadyPrivate is returned when attempting to flag a strand that is
// already in the private store. We refuse rather than overwrite to avoid
// clobbering the sidecar file with a now-empty body from the main db.
var ErrAlreadyPrivate = errors.New("strand is already private")

// PrivateDir returns the directory that holds sidecar files for flagged strands.
func PrivateDir(root string) string {
	dir, _ := Paths(root)
	return filepath.Join(dir, "private")
}

// PrivateFilePath returns the absolute path of a strand's sidecar file.
func PrivateFilePath(root, strandID string) string {
	return filepath.Join(PrivateDir(root), strandID+".md")
}

// IsPrivate reports whether a strand has a private_flags row, returning the
// reason if so.
func IsPrivate(conn *sql.DB, strandID string) (bool, string, error) {
	var reason string
	err := conn.QueryRow(
		`SELECT reason FROM private_flags WHERE strand_id = ?`,
		strandID,
	).Scan(&reason)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, reason, nil
}

// FlagPrivate moves a strand's body into a sidecar file and flags the row.
// Fails if the strand is already private. Writes the file first so a db
// failure leaves an orphan file (harmless) rather than a flag-without-file
// state that show --include-private could not resolve.
func FlagPrivate(conn *sql.DB, root, strandID, reason string) error {
	already, _, err := IsPrivate(conn, strandID)
	if err != nil {
		return err
	}
	if already {
		return ErrAlreadyPrivate
	}

	s, err := GetStrand(conn, strandID)
	if err != nil {
		return err
	}

	if err := WritePrivateBody(root, s.ID, s.Body); err != nil {
		return err
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE strands SET body = '' WHERE id = ?`, s.ID); err != nil {
		return fmt.Errorf("clear body: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO private_flags (strand_id, reason, flagged_at) VALUES (?, ?, ?)`,
		s.ID, reason, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("insert flag: %w", err)
	}
	return tx.Commit()
}

// CreatePrivateStrand inserts a strand whose body is stored only in the
// sidecar file from the start. The body column is empty and the private_flags
// row is written in the same transaction as the strand row.
func CreatePrivateStrand(conn *sql.DB, root, sessionID, topic, body, reason string) (string, error) {
	tx, err := conn.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	id := ids.New()
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := tx.Exec(
		`INSERT INTO strands (id, session_id, topic, body, created_at) VALUES (?, ?, ?, '', ?)`,
		id, sessionID, topic, now,
	); err != nil {
		return "", fmt.Errorf("insert strand: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO private_flags (strand_id, reason, flagged_at) VALUES (?, ?, ?)`,
		id, reason, now,
	); err != nil {
		return "", fmt.Errorf("insert flag: %w", err)
	}

	if err := WritePrivateBody(root, id, body); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(PrivateFilePath(root, id))
		return "", err
	}
	return id, nil
}

// WritePrivateBody writes a strand's body to its sidecar file, ensuring the
// private directory exists and is gitignored via a .gitignore drop-in.
func WritePrivateBody(root, strandID, body string) error {
	dir := PrivateDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); os.IsNotExist(err) {
		if err := os.WriteFile(gi, []byte("*\n"), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", gi, err)
		}
	}
	path := PrivateFilePath(root, strandID)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ReadPrivateBody reads a strand's body from its sidecar file.
func ReadPrivateBody(root, strandID string) (string, error) {
	path := PrivateFilePath(root, strandID)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

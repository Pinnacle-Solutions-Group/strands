package db

import (
	"database/sql"
	"time"

	"github.com/Pinnacle-Solutions-Group/strands/internal/ids"
)

// CreateSession inserts a new session row and returns its id.
func CreateSession(conn *sql.DB, workdir string) (string, error) {
	id := ids.New()
	_, err := conn.Exec(
		`INSERT INTO sessions (id, started_at, workdir) VALUES (?, ?, ?)`,
		id, time.Now().UTC().Format(time.RFC3339), workdir,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

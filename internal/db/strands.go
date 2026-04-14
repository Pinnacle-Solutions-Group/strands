package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kevinmrohr/strands/internal/ids"
)

// Strand is the full record of a stored conversation chunk.
type Strand struct {
	ID        string
	SessionID string
	Topic     string
	Body      string
	CreatedAt time.Time
}

// StrandSummary is the lightweight row used by list operations — omits Body.
type StrandSummary struct {
	ID        string
	SessionID string
	Topic     string
	CreatedAt time.Time
	IsPrivate bool
}

// ErrNotFound is returned when a strand id does not match any row.
var ErrNotFound = errors.New("strand not found")

// ErrAmbiguous is returned when a prefix matches more than one strand.
var ErrAmbiguous = errors.New("strand id prefix is ambiguous")

// CreateStrand inserts a new strand and returns its generated id.
func CreateStrand(conn *sql.DB, sessionID, topic, body string) (string, error) {
	id := ids.New()
	_, err := conn.Exec(
		`INSERT INTO strands (id, session_id, topic, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, sessionID, topic, body, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetStrand resolves a full or prefix id to a single strand. Prefix matches
// are convenient since full ids are timestamp-shaped and tedious to type.
func GetStrand(conn *sql.DB, idOrPrefix string) (*Strand, error) {
	rows, err := conn.Query(
		`SELECT id, session_id, topic, body, created_at
		 FROM strands WHERE id LIKE ? || '%' ORDER BY id LIMIT 2`,
		idOrPrefix,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []Strand
	for rows.Next() {
		var s Strand
		var createdAt string
		if err := rows.Scan(&s.ID, &s.SessionID, &s.Topic, &s.Body, &createdAt); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		matches = append(matches, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %s", ErrNotFound, idOrPrefix)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("%w: %s matches at least %s and %s",
			ErrAmbiguous, idOrPrefix, matches[0].ID, matches[1].ID)
	}
}

// ListStrands returns the most recently created strands, newest first.
// limit <= 0 is treated as "no limit".
func ListStrands(conn *sql.DB, limit int) ([]StrandSummary, error) {
	effectiveLimit := limit
	if limit <= 0 {
		effectiveLimit = -1 // sqlite: negative = no limit
	}
	rows, err := conn.Query(
		`SELECT s.id, s.session_id, s.topic, s.created_at,
		        CASE WHEN p.strand_id IS NULL THEN 0 ELSE 1 END AS is_private
		 FROM strands s
		 LEFT JOIN private_flags p ON p.strand_id = s.id
		 ORDER BY s.created_at DESC, s.id DESC LIMIT ?`,
		effectiveLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StrandSummary
	for rows.Next() {
		var s StrandSummary
		var createdAt string
		var isPrivate int
		if err := rows.Scan(&s.ID, &s.SessionID, &s.Topic, &createdAt, &isPrivate); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.IsPrivate = isPrivate == 1
		out = append(out, s)
	}
	return out, rows.Err()
}

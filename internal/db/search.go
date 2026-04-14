package db

import (
	"database/sql"
	"time"
)

// SearchHit is a single FTS5 match.
type SearchHit struct {
	ID        string
	SessionID string
	Topic     string
	CreatedAt time.Time
	Snippet   string
}

// SearchStrands runs an FTS5 MATCH against topic + body, ordered by rank.
// The query string is passed to FTS5 directly, so users can use FTS5 syntax
// (phrase "hello world", prefix foo*, column scoping topic:auth, etc.).
func SearchStrands(conn *sql.DB, query string, limit int) ([]SearchHit, error) {
	effectiveLimit := limit
	if limit <= 0 {
		effectiveLimit = -1
	}
	rows, err := conn.Query(
		`SELECT s.id, s.session_id, s.topic, s.created_at,
		        snippet(strands_fts, 1, '[', ']', ' … ', 12) AS snip
		 FROM strands_fts
		 JOIN strands s ON s.rowid = strands_fts.rowid
		 LEFT JOIN private_flags p ON p.strand_id = s.id
		 WHERE strands_fts MATCH ? AND p.strand_id IS NULL
		 ORDER BY rank
		 LIMIT ?`,
		query, effectiveLimit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		var createdAt string
		if err := rows.Scan(&h.ID, &h.SessionID, &h.Topic, &createdAt, &h.Snippet); err != nil {
			return nil, err
		}
		h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ValidRelations lists the allowed link relations (mirrors schema CHECK).
var ValidRelations = []string{"produced", "discussed", "blocked-on", "discovered"}

// ParseBeadSpec parses "bd-42" or "bd-42:produced" into (bead, relation).
// A missing relation defaults to "discussed".
func ParseBeadSpec(raw string) (string, string, error) {
	parts := strings.SplitN(raw, ":", 2)
	bead := strings.TrimSpace(parts[0])
	if bead == "" {
		return "", "", fmt.Errorf("bead id must not be empty")
	}
	relation := "discussed"
	if len(parts) == 2 {
		relation = strings.TrimSpace(parts[1])
	}
	if !isValidRelation(relation) {
		return "", "", fmt.Errorf("invalid relation %q (want one of %v)", relation, ValidRelations)
	}
	return bead, relation, nil
}

// LinkStrandToBead inserts a strand<->bead link. The strand must exist; the
// bead id is stored verbatim and is not validated against any external db.
// Duplicate (strand, bead, relation) triples are silently ignored so re-linking
// is idempotent.
func LinkStrandToBead(conn *sql.DB, strandID, beadID, relation string) error {
	if !isValidRelation(relation) {
		return fmt.Errorf("invalid relation %q (want one of %v)", relation, ValidRelations)
	}
	_, err := conn.Exec(
		`INSERT OR IGNORE INTO strand_bead_links (strand_id, bead_id, relation) VALUES (?, ?, ?)`,
		strandID, beadID, relation,
	)
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}
	return nil
}

// ListStrandsByBead returns all strands linked to the given bead id.
func ListStrandsByBead(conn *sql.DB, beadID string, limit int) ([]StrandSummary, error) {
	effectiveLimit := limit
	if limit <= 0 {
		effectiveLimit = -1
	}
	rows, err := conn.Query(
		`SELECT s.id, s.session_id, s.topic, s.created_at,
		        CASE WHEN p.strand_id IS NULL THEN 0 ELSE 1 END AS is_private
		 FROM strands s
		 JOIN strand_bead_links l ON l.strand_id = s.id
		 LEFT JOIN private_flags p ON p.strand_id = s.id
		 WHERE l.bead_id = ?
		 ORDER BY s.created_at DESC, s.id DESC
		 LIMIT ?`,
		beadID, effectiveLimit,
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

func isValidRelation(r string) bool {
	for _, v := range ValidRelations {
		if v == r {
			return true
		}
	}
	return false
}

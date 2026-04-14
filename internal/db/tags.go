package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// ValidTagTypes mirrors the schema CHECK on strand_tags.tag_type.
var ValidTagTypes = []string{"read", "user", "corrected", "inferred", "tested", "narrative"}

// ParseTag parses a "type" or "type:value" string into (type, value).
// An empty value is represented as "" (stored as NULL in the db).
func ParseTag(raw string) (string, string, error) {
	parts := strings.SplitN(raw, ":", 2)
	tagType := strings.TrimSpace(parts[0])
	if !isValidTagType(tagType) {
		return "", "", fmt.Errorf("invalid tag type %q (want one of %v)", tagType, ValidTagTypes)
	}
	if len(parts) == 1 {
		return tagType, "", nil
	}
	return tagType, strings.TrimSpace(parts[1]), nil
}

// AddStrandTag inserts a tag row, deduping on (strand_id, tag_type, tag_value).
// An absent value is stored as the empty string so the composite primary key
// still dedupes reliably — SQLite treats NULLs in composite PKs as distinct,
// which would let duplicate (strand, 'user', NULL) rows accumulate.
func AddStrandTag(conn *sql.DB, strandID, tagType, tagValue string) error {
	_, err := conn.Exec(
		`INSERT OR IGNORE INTO strand_tags (strand_id, tag_type, tag_value) VALUES (?, ?, ?)`,
		strandID, tagType, tagValue,
	)
	if err != nil {
		return fmt.Errorf("insert tag: %w", err)
	}
	return nil
}

func isValidTagType(t string) bool {
	for _, v := range ValidTagTypes {
		if v == t {
			return true
		}
	}
	return false
}

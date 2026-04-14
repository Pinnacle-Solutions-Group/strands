// Package ids generates sortable short identifiers for strands rows.
// Format: YYYYMMDDTHHMMSS-xxxx (UTC timestamp + 2 random bytes hex).
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func New() string {
	ts := time.Now().UTC().Format("20060102T150405")
	var b [2]byte
	_, _ = rand.Read(b[:])
	return ts + "-" + hex.EncodeToString(b[:])
}

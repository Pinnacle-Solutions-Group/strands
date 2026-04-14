package claudehook

import (
	"encoding/json"
	"reflect"
	"testing"
)

// parseSettings is a small test helper that unmarshals a JSON literal into
// the same generic map shape Scrub operates on.
func parseSettings(t *testing.T, raw string) map[string]any {
	t.Helper()
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("unmarshal fixture: %v\n%s", err, raw)
	}
	return settings
}

// Models the lab-notes/.claude/settings.json layout observed in the wild:
// shelf hooks sit in their own outer matcher entries alongside bd prime
// matchers and a docker-compose-fix matcher. Scrub should drop the shelf
// matchers and leave every other matcher intact.
func TestScrub_LabNotesLayout(t *testing.T) {
	settings := parseSettings(t, `{
		"hooks": {
			"PreCompact": [
				{"matcher": "", "hooks": [{"type": "command", "command": ".claude/hooks/context-shelf-trigger.sh"}]},
				{"matcher": "", "hooks": [{"type": "command", "command": "bd prime"}]}
			],
			"PreToolUse": [
				{"matcher": "", "hooks": [{"type": "command", "command": ".claude/hooks/docker-compose-fix.sh"}]}
			],
			"SessionStart": [
				{"matcher": "", "hooks": [{"type": "command", "command": ".claude/hooks/context-shelf-session-start.sh"}]},
				{"matcher": "", "hooks": [{"type": "command", "command": "bd prime"}]}
			]
		}
	}`)

	removed := Scrub(settings, NewShelfPredicate())

	if len(removed) != 2 {
		t.Fatalf("removed hooks = %d, want 2 (one SessionStart + one PreCompact)", len(removed))
	}

	// SessionStart should keep only the bd prime matcher.
	sessionStart := settings["hooks"].(map[string]any)["SessionStart"].([]any)
	if len(sessionStart) != 1 {
		t.Fatalf("SessionStart entries = %d, want 1", len(sessionStart))
	}
	ssInner := sessionStart[0].(map[string]any)["hooks"].([]any)
	if cmd := ssInner[0].(map[string]any)["command"].(string); cmd != "bd prime" {
		t.Errorf("surviving SessionStart command = %q, want 'bd prime'", cmd)
	}

	// PreCompact should keep only the bd prime matcher.
	preCompact := settings["hooks"].(map[string]any)["PreCompact"].([]any)
	if len(preCompact) != 1 {
		t.Fatalf("PreCompact entries = %d, want 1", len(preCompact))
	}
	pcInner := preCompact[0].(map[string]any)["hooks"].([]any)
	if cmd := pcInner[0].(map[string]any)["command"].(string); cmd != "bd prime" {
		t.Errorf("surviving PreCompact command = %q, want 'bd prime'", cmd)
	}

	// PreToolUse must be untouched.
	preTool := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preTool) != 1 {
		t.Fatalf("PreToolUse entries = %d, want 1", len(preTool))
	}
}

// Models the eoq-financials-wizard layout: a compound SessionStart matcher
// with two inner hooks — a GSD hook the user doesn't want touched and a
// shelf hook the user wants removed. The matcher entry must survive; only
// the inner shelf entry gets filtered out. PreCompact has a solo shelf
// matcher with no neighbors, so the matcher entry is dropped entirely.
func TestScrub_EoqCompoundLayout(t *testing.T) {
	settings := parseSettings(t, `{
		"hooks": {
			"SessionStart": [
				{
					"hooks": [
						{"type": "command", "command": "node .claude/hooks/gsd-check-update.js"},
						{"type": "command", "command": ".claude/hooks/context-shelf-session-start.sh"}
					]
				}
			],
			"PostToolUse": [
				{"hooks": [{"type": "command", "command": "node .claude/hooks/gsd-context-monitor.js"}]}
			],
			"PreCompact": [
				{"matcher": "", "hooks": [{"type": "command", "command": ".claude/hooks/context-shelf-trigger.sh"}]}
			]
		}
	}`)

	removed := Scrub(settings, NewShelfPredicate())

	if len(removed) != 2 {
		t.Fatalf("removed hooks = %d, want 2", len(removed))
	}

	hooks := settings["hooks"].(map[string]any)

	// SessionStart compound matcher should survive with only the GSD inner entry.
	ss := hooks["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Fatalf("SessionStart entries = %d, want 1 (compound matcher must survive)", len(ss))
	}
	inner := ss[0].(map[string]any)["hooks"].([]any)
	if len(inner) != 1 {
		t.Fatalf("SessionStart inner hooks = %d, want 1 (GSD must survive)", len(inner))
	}
	if cmd := inner[0].(map[string]any)["command"].(string); cmd != "node .claude/hooks/gsd-check-update.js" {
		t.Errorf("surviving inner command = %q, want GSD", cmd)
	}

	// PostToolUse is untouched.
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("PostToolUse was dropped, should survive")
	}

	// PreCompact had only one shelf hook and should now be gone entirely.
	if _, ok := hooks["PreCompact"]; ok {
		t.Error("PreCompact should have been deleted — it held only a shelf hook")
	}
}

// When every hook in settings is a shelf hook, Scrub should remove the
// top-level "hooks" key entirely so the file stops carrying an empty object.
func TestScrub_EmptyAfterRemoval(t *testing.T) {
	settings := parseSettings(t, `{
		"mcpServers": {"foo": {}},
		"hooks": {
			"SessionStart": [
				{"hooks": [{"type": "command", "command": ".claude/hooks/context-shelf-session-start.sh"}]}
			],
			"PreCompact": [
				{"hooks": [{"type": "command", "command": ".claude/hooks/context-shelf-trigger.sh"}]}
			]
		}
	}`)

	removed := Scrub(settings, NewShelfPredicate())

	if len(removed) != 2 {
		t.Fatalf("removed = %d, want 2", len(removed))
	}
	if _, ok := settings["hooks"]; ok {
		t.Error("empty hooks map should have been deleted")
	}
	// Unrelated top-level keys must survive.
	if _, ok := settings["mcpServers"]; !ok {
		t.Error("mcpServers was dropped — Scrub must preserve unrelated top-level keys")
	}
}

// When nothing matches, Scrub should report zero removals and leave the
// settings map byte-identical when re-marshalled.
func TestScrub_NoMatch(t *testing.T) {
	raw := `{
		"hooks": {
			"SessionStart": [
				{"hooks": [{"type": "command", "command": "bd prime"}]}
			]
		}
	}`
	settings := parseSettings(t, raw)
	before, _ := json.Marshal(settings)

	removed := Scrub(settings, NewShelfPredicate())

	if len(removed) != 0 {
		t.Errorf("removed = %d, want 0", len(removed))
	}
	after, _ := json.Marshal(settings)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("no-match scrub mutated settings\nbefore: %s\nafter:  %s", before, after)
	}
}

// --also-remove patterns should stack on top of the shelf patterns, so a
// single Scrub pass can take out shelf and GSD together on the user's
// local machine.
func TestScrub_AlsoRemovePattern(t *testing.T) {
	settings := parseSettings(t, `{
		"hooks": {
			"SessionStart": [
				{
					"hooks": [
						{"type": "command", "command": "node .claude/hooks/gsd-check-update.js"},
						{"type": "command", "command": ".claude/hooks/context-shelf-session-start.sh"},
						{"type": "command", "command": "bd prime"}
					]
				}
			]
		}
	}`)

	removed := Scrub(settings, NewShelfPredicate("gsd-"))

	if len(removed) != 2 {
		t.Fatalf("removed = %d, want 2 (shelf + gsd)", len(removed))
	}
	inner := settings["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)["hooks"].([]any)
	if len(inner) != 1 {
		t.Fatalf("surviving inner hooks = %d, want 1 (bd prime only)", len(inner))
	}
	if cmd := inner[0].(map[string]any)["command"].(string); cmd != "bd prime" {
		t.Errorf("surviving command = %q, want 'bd prime'", cmd)
	}
}

// Settings with no "hooks" key at all should scrub cleanly without errors.
func TestScrub_MissingHooksKey(t *testing.T) {
	settings := parseSettings(t, `{"mcpServers": {"foo": {}}}`)
	removed := Scrub(settings, NewShelfPredicate())
	if len(removed) != 0 {
		t.Errorf("removed = %d, want 0", len(removed))
	}
	if _, ok := settings["mcpServers"]; !ok {
		t.Error("mcpServers must be preserved")
	}
}

// The predicate must match each documented shelf pattern substring.
func TestNewShelfPredicate_DefaultPatterns(t *testing.T) {
	pred := NewShelfPredicate()
	matches := []string{
		".claude/hooks/context-shelf-session-start.sh",
		"/Users/me/.claude/hooks/context-shelf-trigger.sh",
		"cat .claude/history/toc.md",
		"cat .claude/plans/COMPLETED.md",
		"cat .claude/plans/CANCELLED.md",
		"cat .claude/plans/DEPENDENCIES.md",
		"cat .claude/private/toc.md",
	}
	for _, cmd := range matches {
		if !pred(cmd) {
			t.Errorf("predicate did not match %q", cmd)
		}
	}

	nonMatches := []string{
		"bd prime",
		"strands list --limit 0",
		"node .claude/hooks/gsd-check-update.js",
		".claude/hooks/docker-compose-fix.sh",
	}
	for _, cmd := range nonMatches {
		if pred(cmd) {
			t.Errorf("predicate incorrectly matched %q", cmd)
		}
	}
}

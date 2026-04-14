package claudehook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstall_CreatesSettingsWhenMissing(t *testing.T) {
	root := t.TempDir()

	replaced, err := Install(root, 0)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if replaced {
		t.Fatal("replaced=true on fresh install, want false")
	}

	settings := readSettings(t, root)
	hooks := mustSessionStart(t, settings)
	if len(hooks) != 1 {
		t.Fatalf("SessionStart entries = %d, want 1", len(hooks))
	}
	assertStrandsCommandPresent(t, hooks, "--limit 0")
}

func TestInstall_PreservesExistingHooks(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{"type": "command", "command": "bd prime"},
					},
				},
			},
			"PreCompact": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo compacting"},
					},
				},
			},
		},
		"theme": "dark",
	}
	writeJSON(t, filepath.Join(claudeDir, "settings.json"), original)

	if _, err := Install(root, 200); err != nil {
		t.Fatalf("Install: %v", err)
	}

	settings := readSettings(t, root)
	if settings["theme"] != "dark" {
		t.Errorf("theme lost: %v", settings["theme"])
	}

	hooks := settings["hooks"].(map[string]any)
	preCompact := hooks["PreCompact"].([]any)
	if len(preCompact) != 1 {
		t.Errorf("PreCompact lost: %v", preCompact)
	}

	sessionStart := hooks["SessionStart"].([]any)
	if len(sessionStart) != 2 {
		t.Fatalf("SessionStart len = %d, want 2 (bd prime + strands)", len(sessionStart))
	}
	// First entry should still be the bd prime matcher untouched.
	first := sessionStart[0].(map[string]any)
	firstActions := first["hooks"].([]any)
	if cmd := firstActions[0].(map[string]any)["command"].(string); cmd != "bd prime" {
		t.Errorf("bd prime hook clobbered: %q", cmd)
	}
	assertStrandsCommandPresent(t, sessionStart, "--limit 200")
}

func TestInstall_IdempotentReplacesLimit(t *testing.T) {
	root := t.TempDir()

	if _, err := Install(root, 0); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	replaced, err := Install(root, 500)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !replaced {
		t.Error("replaced=false on re-install, want true")
	}

	sessionStart := mustSessionStart(t, readSettings(t, root))
	// Expect exactly one strands hook, not two.
	strandsCount := 0
	for _, entry := range sessionStart {
		actions := entry.(map[string]any)["hooks"].([]any)
		for _, a := range actions {
			cmd := a.(map[string]any)["command"].(string)
			if strings.Contains(cmd, markerSubstring) {
				strandsCount++
			}
		}
	}
	if strandsCount != 1 {
		t.Fatalf("strands hook count = %d, want 1", strandsCount)
	}
	assertStrandsCommandPresent(t, sessionStart, "--limit 500")
}

func TestInstall_RemovesEmptiedMatcher(t *testing.T) {
	// A matcher that originally held ONLY a strands hook should be removed
	// entirely rather than left as an empty-hooks stub.
	root := t.TempDir()

	if _, err := Install(root, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(root, 50); err != nil {
		t.Fatal(err)
	}

	sessionStart := mustSessionStart(t, readSettings(t, root))
	if len(sessionStart) != 1 {
		t.Errorf("expected 1 matcher after re-install, got %d: %+v", len(sessionStart), sessionStart)
	}
}

func TestInstall_MalformedSettingsErrors(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(root, 0); err == nil {
		t.Fatal("expected error on malformed settings.json, got nil")
	}
}

func TestCommand_IncludesHeaderAndLimit(t *testing.T) {
	got := Command(0)
	if !strings.Contains(got, "## Strands TOC") {
		t.Errorf("Command missing header: %q", got)
	}
	if !strings.Contains(got, "strands list --limit 0") {
		t.Errorf("Command missing list invocation: %q", got)
	}
}

// ---- helpers ----

func readSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	return out
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSessionStart(t *testing.T, settings map[string]any) []any {
	t.Helper()
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("settings.hooks missing or wrong type: %v", settings["hooks"])
	}
	entries, ok := hooks["SessionStart"].([]any)
	if !ok {
		t.Fatalf("SessionStart missing or wrong type: %v", hooks["SessionStart"])
	}
	return entries
}

func assertStrandsCommandPresent(t *testing.T, entries []any, mustContain string) {
	t.Helper()
	for _, entry := range entries {
		matcher, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		actions, _ := matcher["hooks"].([]any)
		for _, a := range actions {
			act, _ := a.(map[string]any)
			cmd, _ := act["command"].(string)
			if strings.Contains(cmd, markerSubstring) && strings.Contains(cmd, mustContain) {
				return
			}
		}
	}
	t.Errorf("no strands hook containing %q found in %+v", mustContain, entries)
}

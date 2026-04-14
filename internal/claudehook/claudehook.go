// Package claudehook installs a Claude Code SessionStart hook that injects
// strands list output as in-context additional context. It merges into any
// existing .claude/settings.json without clobbering unrelated hooks, and is
// idempotent — re-running replaces the strands hook (identified by the
// markerSubstring in its command string) so flags like --limit take effect.
package claudehook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudeDir    = ".claude"
	settingsFile = "settings.json"
	// markerSubstring identifies a strands-owned hook so we can replace it
	// on re-install without touching the user's other hooks.
	markerSubstring = "strands list"
)

// Command returns the shell command string the SessionStart hook will run.
// The printf header makes the injected context self-describing to Claude.
func Command(limit int) string {
	return fmt.Sprintf(
		`printf '## Strands TOC — lookup body via: strands show <id>\n\n'; strands list --limit %d`,
		limit,
	)
}

// Install writes the strands SessionStart hook into <root>/.claude/settings.json.
// It preserves all other settings fields and hooks. If a prior strands hook is
// found (by markerSubstring), it is replaced so the new limit takes effect.
// Returns replaced=true if an existing strands hook was overwritten.
func Install(root string, limit int) (replaced bool, err error) {
	dir := filepath.Join(root, claudeDir)
	path := filepath.Join(dir, settingsFile)

	settings, err := loadSettings(path)
	if err != nil {
		return false, err
	}

	replaced = upsertStrandsHook(settings, Command(limit))

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", dir, err)
	}
	if err := writeSettings(path, settings); err != nil {
		return false, err
	}
	return replaced, nil
}

// loadSettings reads settings.json into a generic map so we don't drop
// unknown fields when we write it back. A missing file is not an error.
func loadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	// Use a streaming encoder with HTML escaping disabled so shell-meaningful
	// characters like <id> stay readable instead of becoming \u003cid\u003e.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(settings); err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// upsertStrandsHook mutates settings in place. It finds any SessionStart
// matcher that contains a strands-owned hook action, drops that action,
// and then appends a fresh strands matcher with the new command. Non-strands
// actions and matchers are preserved untouched. Returns true if an existing
// strands hook was removed before appending.
func upsertStrandsHook(settings map[string]any, command string) (replaced bool) {
	hooks := asMap(settings, "hooks")
	sessionStart := asSlice(hooks, "SessionStart")

	cleaned := make([]any, 0, len(sessionStart)+1)
	for _, entry := range sessionStart {
		matcher, ok := entry.(map[string]any)
		if !ok {
			cleaned = append(cleaned, entry)
			continue
		}
		actions, _ := matcher["hooks"].([]any)
		keptActions := make([]any, 0, len(actions))
		for _, action := range actions {
			act, ok := action.(map[string]any)
			if !ok {
				keptActions = append(keptActions, action)
				continue
			}
			cmd, _ := act["command"].(string)
			if strings.Contains(cmd, markerSubstring) {
				replaced = true
				continue
			}
			keptActions = append(keptActions, action)
		}
		// Drop matcher entries that become empty after removing the strands action.
		if len(keptActions) == 0 {
			continue
		}
		matcher["hooks"] = keptActions
		cleaned = append(cleaned, matcher)
	}

	cleaned = append(cleaned, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	})

	hooks["SessionStart"] = cleaned
	settings["hooks"] = hooks
	return replaced
}

// asMap returns settings[key] as a map, creating and storing an empty map
// if the key is missing or the wrong type. Lossy for non-map values — but
// Claude Code settings.json shouldn't have a non-map "hooks" field.
func asMap(parent map[string]any, key string) map[string]any {
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	parent[key] = m
	return m
}

func asSlice(parent map[string]any, key string) []any {
	if s, ok := parent[key].([]any); ok {
		return s
	}
	return nil
}

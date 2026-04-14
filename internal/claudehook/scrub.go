package claudehook

import (
	"os"
	"path/filepath"
	"strings"
)

// ShelfHookPatterns is the default substring set identifying a context-shelf
// hook in a Claude Code settings.json. Every shelf hook command in the wild
// either references a context-shelf-*.sh script or one of the shelf's TOC
// files — the substring match covers both the scripts installed by the old
// shelving system and any hand-rolled inline hooks that cat a TOC file.
var ShelfHookPatterns = []string{
	"context-shelf",
	".claude/history/toc.md",
	".claude/plans/COMPLETED.md",
	".claude/plans/CANCELLED.md",
	".claude/plans/DEPENDENCIES.md",
	".claude/private/toc.md",
}

// NewShelfPredicate builds a predicate that matches any hook command
// containing one of ShelfHookPatterns or any of the extra substrings.
// The extras come from --also-remove flags so users can take out
// neighboring stale hooks (e.g. "gsd-") in the same scrub pass.
func NewShelfPredicate(extra ...string) func(string) bool {
	patterns := make([]string, 0, len(ShelfHookPatterns)+len(extra))
	patterns = append(patterns, ShelfHookPatterns...)
	for _, p := range extra {
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return func(cmd string) bool {
		for _, p := range patterns {
			if strings.Contains(cmd, p) {
				return true
			}
		}
		return false
	}
}

// RemovedHook records a single hook that Scrub removed from a settings map.
// Event is the Claude Code event name ("SessionStart", "PreCompact", etc.)
// and Command is the shell command string that was dropped.
type RemovedHook struct {
	Event   string
	Command string
}

// Scrub walks every Claude Code hook event in settings and removes any
// inner hook entry whose command matches predicate. It operates at the
// inner .hooks[] array level so that shelf hooks sharing a matcher with
// unrelated neighbors (like a GSD hook) get removed surgically without
// nuking the neighbors.
//
// Cleanup rules applied in order:
//   - If an inner hook's command matches predicate, drop that inner entry.
//   - If an outer matcher's inner .hooks[] array becomes empty, drop the
//     matcher entry.
//   - If an event's outer array becomes empty, delete the event key.
//   - If the top-level "hooks" map becomes empty, delete it from settings.
//
// Every other top-level key in settings (mcpServers, enabledPlugins, etc.)
// is left untouched. The function mutates settings in place and returns
// the removed hooks in deterministic walk order for dry-run display.
func Scrub(settings map[string]any, predicate func(string) bool) []RemovedHook {
	removed := []RemovedHook{}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || len(hooks) == 0 {
		return removed
	}

	// Stable event order so tests and dry-run output don't depend on map
	// iteration order. Walk a fixed list of known events plus any others
	// the file happens to hold.
	knownOrder := []string{
		"SessionStart",
		"UserPromptSubmit",
		"PreToolUse",
		"PostToolUse",
		"Stop",
		"SubagentStop",
		"PreCompact",
		"Notification",
	}
	seen := map[string]bool{}
	events := make([]string, 0, len(hooks))
	for _, e := range knownOrder {
		if _, ok := hooks[e]; ok {
			events = append(events, e)
			seen[e] = true
		}
	}
	for k := range hooks {
		if !seen[k] {
			events = append(events, k)
		}
	}

	for _, event := range events {
		outer, ok := hooks[event].([]any)
		if !ok {
			continue
		}
		keptOuter := make([]any, 0, len(outer))
		for _, outerEntry := range outer {
			entry, ok := outerEntry.(map[string]any)
			if !ok {
				keptOuter = append(keptOuter, outerEntry)
				continue
			}
			inner, _ := entry["hooks"].([]any)
			keptInner := make([]any, 0, len(inner))
			for _, innerEntry := range inner {
				action, ok := innerEntry.(map[string]any)
				if !ok {
					keptInner = append(keptInner, innerEntry)
					continue
				}
				cmd, _ := action["command"].(string)
				if predicate(cmd) {
					removed = append(removed, RemovedHook{Event: event, Command: cmd})
					continue
				}
				keptInner = append(keptInner, innerEntry)
			}
			if len(keptInner) == 0 {
				continue
			}
			entry["hooks"] = keptInner
			keptOuter = append(keptOuter, entry)
		}
		if len(keptOuter) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = keptOuter
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	return removed
}

// ScrubFile loads a Claude Code settings file at path, runs Scrub with
// predicate, and — if apply is true — writes the result back using the
// same formatter as Install. A missing file is not an error; it returns
// (nil, nil) so callers can loop over candidate paths without pre-checking.
func ScrubFile(path string, predicate func(string) bool, apply bool) ([]RemovedHook, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	settings, err := loadSettings(path)
	if err != nil {
		return nil, err
	}
	removed := Scrub(settings, predicate)
	if apply && len(removed) > 0 {
		if err := writeSettings(path, settings); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

// DefaultGlobalSettingsPath returns the absolute path of the user's
// global Claude Code settings file (~/.claude/settings.json).
func DefaultGlobalSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, claudeDir, settingsFile), nil
}

// RepoSettingsPaths returns the two candidate settings files inside a
// repository: .claude/settings.json (committed) and .claude/settings.local.json
// (per-user, typically gitignored). Either or both may be absent — ScrubFile
// handles missing files gracefully.
func RepoSettingsPaths(root string) []string {
	return []string{
		filepath.Join(root, claudeDir, settingsFile),
		filepath.Join(root, claudeDir, "settings.local.json"),
	}
}

// DefaultGlobalHooksDir returns ~/.claude/hooks, where the old shelving
// system installed its context-shelf-*.sh scripts.
func DefaultGlobalHooksDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, claudeDir, "hooks"), nil
}

// RepoHooksDir returns <root>/.claude/hooks.
func RepoHooksDir(root string) string {
	return filepath.Join(root, claudeDir, "hooks")
}

// FindShelfScripts returns every file in dir whose name matches the
// context-shelf-*.sh pattern. A missing dir returns nil with no error.
// Use RemoveFiles to delete the returned paths.
func FindShelfScripts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var matched []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "context-shelf-") && strings.HasSuffix(name, ".sh") {
			matched = append(matched, filepath.Join(dir, name))
		}
	}
	return matched, nil
}

// RemoveFiles deletes every file in paths. Stops on the first error and
// returns it alongside the list of files successfully removed before the
// failure, so callers can report partial progress.
func RemoveFiles(paths []string) (removed []string, err error) {
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			return removed, err
		}
		removed = append(removed, p)
	}
	return removed, nil
}

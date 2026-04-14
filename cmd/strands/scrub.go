package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Pinnacle-Solutions-Group/strands/internal/claudehook"
)

func newScrubShelfHooksCmd() *cobra.Command {
	var (
		repo           string
		apply          bool
		alsoRemove     []string
		removeScripts  bool
	)

	cmd := &cobra.Command{
		Use:   "scrub-shelf-hooks",
		Short: "Remove context-shelf hooks from Claude Code settings.json",
		Long: "scrub-shelf-hooks removes stale context-shelf hooks (context-shelf-session-start.sh, " +
			"context-shelf-trigger.sh, TOC-loading inline commands) from Claude Code settings.json. " +
			"By default it targets ~/.claude/settings.json (global). Pass --repo <path> to target a " +
			"single repository's .claude/settings.json + .claude/settings.local.json. Compound hook " +
			"entries sharing a matcher with unrelated hooks (GSD, docker-compose-fix, etc.) are " +
			"surgically edited — only the shelf inner hooks are removed, neighbors are preserved.\n\n" +
			"Runs in dry-run mode by default. Pass --apply to actually rewrite the settings files. " +
			"Pass --remove-scripts to also delete the corresponding context-shelf-*.sh scripts from " +
			"the hooks directory at the same scope (~/.claude/hooks global, or <repo>/.claude/hooks " +
			"when --repo is set). Pass --also-remove <substring> (repeatable) to append extra " +
			"substring matches to the predicate — useful for taking out neighboring stale hooks " +
			"in the same pass without touching the default pattern set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			predicate := claudehook.NewShelfPredicate(alsoRemove...)

			targets, hooksDir, scopeLabel, err := resolveScrubScope(repo)
			if err != nil {
				return err
			}

			return runScrub(targets, hooksDir, scopeLabel, predicate, apply, removeScripts)
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "scrub this repo's .claude/settings.json and settings.local.json instead of the global ~/.claude/settings.json")
	cmd.Flags().BoolVar(&apply, "apply", false, "write changes to disk (default is dry-run)")
	cmd.Flags().StringSliceVar(&alsoRemove, "also-remove", nil, "extra substring to match (repeatable) — appended to the default context-shelf pattern set")
	cmd.Flags().BoolVar(&removeScripts, "remove-scripts", false, "also delete context-shelf-*.sh scripts from the hooks directory at the same scope")

	return cmd
}

// resolveScrubScope returns the settings files to scrub, the hooks
// directory holding context-shelf-*.sh scripts, and a human-readable
// scope label for output. If repo is empty, scope is global
// (~/.claude/settings.json + ~/.claude/hooks); otherwise scope is the
// named repository.
func resolveScrubScope(repo string) (targets []string, hooksDir, label string, err error) {
	if repo == "" {
		globalSettings, err := claudehook.DefaultGlobalSettingsPath()
		if err != nil {
			return nil, "", "", err
		}
		globalHooks, err := claudehook.DefaultGlobalHooksDir()
		if err != nil {
			return nil, "", "", err
		}
		return []string{globalSettings}, globalHooks, "global (~/.claude)", nil
	}
	abs, err := absPath(repo)
	if err != nil {
		return nil, "", "", err
	}
	return claudehook.RepoSettingsPaths(abs), claudehook.RepoHooksDir(abs), fmt.Sprintf("repo (%s)", abs), nil
}

// runScrub executes the scrub against every target settings file, then
// optionally against hook scripts, printing a human-readable report of
// what was (or would be) removed.
func runScrub(targets []string, hooksDir, scopeLabel string, predicate func(string) bool, apply, removeScripts bool) error {
	fmt.Printf("scope: %s\n", scopeLabel)
	if !apply {
		fmt.Println("mode:  dry-run (re-run with --apply to write changes)")
	} else {
		fmt.Println("mode:  apply")
	}
	fmt.Println()

	totalHooks := 0
	touchedFiles := 0
	for _, path := range targets {
		removed, err := claudehook.ScrubFile(path, predicate, apply)
		if err != nil {
			return fmt.Errorf("scrub %s: %w", path, err)
		}
		if removed == nil {
			// File did not exist.
			fmt.Printf("%s: not present, skipped\n", displayPath(path))
			continue
		}
		if len(removed) == 0 {
			fmt.Printf("%s: no shelf hooks found\n", displayPath(path))
			continue
		}

		touchedFiles++
		totalHooks += len(removed)

		// Group the removed hooks by event for readable output.
		byEvent := map[string][]string{}
		for _, r := range removed {
			byEvent[r.Event] = append(byEvent[r.Event], r.Command)
		}
		events := make([]string, 0, len(byEvent))
		for e := range byEvent {
			events = append(events, e)
		}
		sort.Strings(events)

		fmt.Printf("%s:\n", displayPath(path))
		for _, e := range events {
			for _, cmd := range byEvent[e] {
				fmt.Printf("  %s  %s\n", e, cmd)
			}
		}
	}

	if removeScripts {
		fmt.Println()
		scripts, err := claudehook.FindShelfScripts(hooksDir)
		if err != nil {
			return fmt.Errorf("scan %s: %w", hooksDir, err)
		}
		if len(scripts) == 0 {
			fmt.Printf("%s: no context-shelf-*.sh scripts found\n", displayPath(hooksDir))
		} else {
			fmt.Printf("%s:\n", displayPath(hooksDir))
			for _, s := range scripts {
				fmt.Printf("  %s\n", displayPath(s))
			}
			if apply {
				if _, err := claudehook.RemoveFiles(scripts); err != nil {
					return fmt.Errorf("delete scripts: %w", err)
				}
			}
		}
	}

	fmt.Println()
	verb := "would remove"
	if apply {
		verb = "removed"
	}
	fmt.Printf("summary: %s %d hook(s) from %d file(s)", verb, totalHooks, touchedFiles)
	if removeScripts {
		fmt.Print(" and any listed scripts")
	}
	fmt.Println(".")

	if !apply && totalHooks == 0 && !removeScripts {
		fmt.Println("nothing to do — no shelf hooks matched the default pattern set.")
	}
	return nil
}

// absPath resolves repo to an absolute path, anchoring relative paths at
// cwd. Used so downstream output shows a stable canonical path instead
// of whatever the user typed.
func absPath(repo string) (string, error) {
	if repo == "" {
		return os.Getwd()
	}
	if repo[0] == '/' {
		return repo, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd + "/" + repo, nil
}

// displayPath collapses the user's home directory to ~ for compact output.
// Everything else is shown verbatim.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if len(path) >= len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}

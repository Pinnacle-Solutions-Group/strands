package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/Pinnacle-Solutions-Group/strands/internal/claudehook"
	"github.com/Pinnacle-Solutions-Group/strands/internal/db"
)

func main() {
	root := &cobra.Command{
		Use:   "strands",
		Short: "Conversation shelving for Claude Code sessions",
		Long: "strands stores distilled, topic-grouped conversation chunks with " +
			"provenance tags, optionally linked to beads (bd-) issue IDs.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newInitCmd())
	root.AddCommand(newIngestCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newLinkCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newPrivateCmd())
	root.AddCommand(newInstallHookCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newInitCmd() *cobra.Command {
	var noHook bool
	var limit int
	var limitSet bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create .strands/ and initialize the database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			dbPath, err := db.Init(cwd)
			if err != nil {
				return err
			}
			fmt.Printf("initialized strands db at %s\n", dbPath)

			if noHook {
				return nil
			}

			limitSet = cmd.Flags().Changed("limit")
			chosen, ok := resolveHookLimit(os.Stdin, os.Stdout, limit, limitSet)
			if !ok {
				fmt.Println("skipped Claude Code hook install — run 'strands install-hook' later to enable the session-start TOC")
				return nil
			}
			if _, err := claudehook.Install(cwd, chosen); err != nil {
				// Hook install is best-effort — don't fail init over it.
				fmt.Fprintf(os.Stderr, "warning: could not install Claude Code hook: %v\n", err)
				fmt.Fprintln(os.Stderr, "  run 'strands install-hook' after fixing the issue")
				return nil
			}
			fmt.Printf("installed Claude Code SessionStart hook (.claude/settings.json, --limit %d)\n", chosen)
			return nil
		},
	}

	cmd.Flags().BoolVar(&noHook, "no-hook", false, "skip installing the Claude Code SessionStart hook")
	cmd.Flags().IntVar(&limit, "limit", 0, "strand limit for the SessionStart TOC hook (0 = unlimited); bypasses the interactive prompt")
	return cmd
}

func newInstallHookCmd() *cobra.Command {
	var limit int
	var global bool

	cmd := &cobra.Command{
		Use:   "install-hook",
		Short: "Install the Claude Code SessionStart hook (global by default)",
		Long: "install-hook writes (or updates) a SessionStart hook in ~/.claude/settings.json " +
			"by default, so every Claude Code session — in any repo with a .strands/ db — auto-" +
			"loads the strand topic TOC as in-context additional context. The hook is guarded " +
			"with a .strands/strands.db existence check so it silently no-ops in non-strands " +
			"projects. Pass --local to install into the current repo's .claude/settings.json " +
			"instead. Safe to re-run — existing strands hooks are replaced so flags like --limit " +
			"take effect; other hooks are left untouched.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, label, err := resolveInstallRoot(global)
			if err != nil {
				return err
			}
			replaced, err := claudehook.Install(root, limit)
			if err != nil {
				return err
			}
			verb := "installed"
			if replaced {
				verb = "updated"
			}
			fmt.Printf("%s Claude Code SessionStart hook (%s, --limit %d)\n", verb, label, limit)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "strand limit for the SessionStart TOC hook (0 = unlimited)")
	cmd.Flags().BoolVar(&global, "global", true, "install to ~/.claude/settings.json (every repo); pass --global=false for project-local install")
	return cmd
}

// resolveInstallRoot returns the directory whose .claude/settings.json the
// installer should target. Global mode uses $HOME so the hook applies to every
// Claude session. Local mode uses cwd so the hook only applies in this repo.
// The returned label is a human-readable path for the confirmation message.
func resolveInstallRoot(global bool) (root, label string, err error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", err
		}
		return home, "~/.claude/settings.json", nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	return cwd, ".claude/settings.json", nil
}

// resolveHookLimit decides what --limit value to bake into the SessionStart
// hook. Precedence: explicit --limit flag, then interactive prompt on a TTY,
// then default 0 (unlimited) for non-TTY / scripted init. The bool return is
// false only if the user declines the install at the prompt.
func resolveHookLimit(in io.Reader, out io.Writer, flagLimit int, flagSet bool) (int, bool) {
	if flagSet {
		return flagLimit, true
	}
	f, isFile := in.(*os.File)
	if !isFile || !isatty.IsTerminal(f.Fd()) {
		return 0, true
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "strands can install a Claude Code SessionStart hook that injects your strand")
	fmt.Fprintln(out, "topic list as in-context additional context on every new session. This is the")
	fmt.Fprintln(out, "live replacement for the old .claude/history/toc.md file: you get a TOC you can")
	fmt.Fprintln(out, "scan, and Claude pulls bodies on demand with 'strands show <id>'.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "How many strands should the TOC show each session?")
	fmt.Fprintln(out, "  0   show all strands (recommended — unbounded like the old toc.md)")
	fmt.Fprintln(out, "  N   cap at N most recent strands (use if history gets noisy later)")
	fmt.Fprintln(out, "  s   skip — don't install the hook; re-run 'strands install-hook' anytime")
	fmt.Fprintln(out, "")
	fmt.Fprint(out, "Limit [0]: ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		// EOF or read error — fall back to the default rather than failing init.
		return 0, true
	}
	answer := strings.TrimSpace(line)
	switch {
	case answer == "":
		return 0, true
	case strings.EqualFold(answer, "s") || strings.EqualFold(answer, "skip"):
		return 0, false
	}
	n, err := strconv.Atoi(answer)
	if err != nil || n < 0 {
		fmt.Fprintf(out, "could not parse %q as a non-negative integer; using 0 (unlimited)\n", answer)
		return 0, true
	}
	return n, true
}

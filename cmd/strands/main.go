package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kevinmrohr/strands/internal/db"
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

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
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
			return nil
		},
	}
}

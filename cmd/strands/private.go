package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kevinmrohr/strands/internal/db"
)

func newPrivateCmd() *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "private <id>",
		Short: "Move a strand's body into the private sidecar store",
		Long: "Writes the strand's body to .strands/private/<id>.md, clears the " +
			"body column in the main database (so FTS cannot index it), and inserts " +
			"a private_flags row with the given reason. Refuses if the strand is " +
			"already private.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reason = strings.TrimSpace(reason)
			if reason == "" {
				return fmt.Errorf("--reason is required")
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			conn, err := db.Open(cwd)
			if err != nil {
				return err
			}
			defer conn.Close()

			s, err := db.GetStrand(conn, args[0])
			if err != nil {
				return err
			}
			if err := db.FlagPrivate(conn, cwd, s.ID, reason); err != nil {
				return err
			}
			fmt.Printf("flagged %s as private: %s\n", s.ID, reason)
			fmt.Printf("body stored at %s\n", db.PrivateFilePath(cwd, s.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "why this strand is private (required)")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

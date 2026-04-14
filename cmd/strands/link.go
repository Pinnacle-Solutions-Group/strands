package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kevinmrohr/strands/internal/db"
)

func newLinkCmd() *cobra.Command {
	var relation string

	cmd := &cobra.Command{
		Use:   "link <strand> <bd-id>",
		Short: "Link a strand to a beads issue id",
		Long: "Records an advisory reference from a strand to a beads (bd-) issue. " +
			"The bead id is not validated against an actual beads db — references " +
			"are soft by design.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			strandIDOrPrefix, beadID := args[0], strings.TrimSpace(args[1])
			if beadID == "" {
				return fmt.Errorf("bead id must not be empty")
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

			s, err := db.GetStrand(conn, strandIDOrPrefix)
			if err != nil {
				return err
			}
			if err := db.LinkStrandToBead(conn, s.ID, beadID, relation); err != nil {
				return err
			}
			fmt.Printf("linked %s -[%s]-> %s\n", s.ID, relation, beadID)
			return nil
		},
	}

	cmd.Flags().StringVar(&relation, "relation", "discussed",
		fmt.Sprintf("link relation (one of %v)", db.ValidRelations))
	return cmd
}

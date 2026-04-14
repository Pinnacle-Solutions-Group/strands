package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kevinmrohr/strands/internal/db"
)

func newShowCmd() *cobra.Command {
	var includePrivate bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Print a strand's body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			private, reason, err := db.IsPrivate(conn, s.ID)
			if err != nil {
				return err
			}

			fmt.Printf("# %s\n", s.Topic)
			fmt.Printf("id: %s\nsession: %s\ncreated: %s\n",
				s.ID, s.SessionID, s.CreatedAt.Format("2006-01-02 15:04:05 MST"))

			if private {
				fmt.Printf("private: yes (%s)\n", reason)
				if !includePrivate {
					fmt.Println()
					return fmt.Errorf("strand is flagged private; pass --include-private to view")
				}
				body, err := db.ReadPrivateBody(cwd, s.ID)
				if err != nil {
					return err
				}
				fmt.Println()
				fmt.Println(body)
				return nil
			}

			fmt.Println()
			fmt.Println(s.Body)
			return nil
		},
	}

	cmd.Flags().BoolVar(&includePrivate, "include-private", false, "allow printing bodies of private strands")
	return cmd
}

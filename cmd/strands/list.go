package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kevinmrohr/strands/internal/db"
)

func newListCmd() *cobra.Command {
	var limit int
	var bead string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List strands, newest first",
		Args:  cobra.NoArgs,
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

			var rows []db.StrandSummary
			if bead != "" {
				rows, err = db.ListStrandsByBead(conn, bead, limit)
			} else {
				rows, err = db.ListStrands(conn, limit)
			}
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Println("no strands yet")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tCREATED\tTOPIC")
			for _, r := range rows {
				topic := r.Topic
				if r.IsPrivate {
					topic = "[private] " + topic
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n",
					r.ID,
					r.CreatedAt.Format("2006-01-02 15:04"),
					topic,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "max rows to show (<=0 for all)")
	cmd.Flags().StringVar(&bead, "bead", "", "filter to strands linked to the given bead id")
	return cmd
}

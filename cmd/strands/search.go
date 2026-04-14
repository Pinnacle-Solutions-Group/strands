package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Pinnacle-Solutions-Group/strands/internal/db"
)

func newSearchCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search over strand topics and bodies",
		Long: "Runs an FTS5 MATCH against strand topics and bodies. The query is " +
			"passed to FTS5 directly so you can use phrase queries (\"hello world\"), " +
			"prefix matches (foo*), column scoping (topic:auth), and boolean operators.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			conn, err := db.Open(cwd)
			if err != nil {
				return err
			}
			defer conn.Close()

			hits, err := db.SearchStrands(conn, query, limit)
			if err != nil {
				return err
			}
			if len(hits) == 0 {
				fmt.Println("no matches")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tTOPIC\tSNIPPET")
			for _, h := range hits {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", h.ID, h.Topic, collapseWhitespace(h.Snippet))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "max hits to show (<=0 for all)")
	return cmd
}

// collapseWhitespace makes snippets fit nicely on one tabwriter line by
// replacing runs of whitespace (including newlines) with a single space.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

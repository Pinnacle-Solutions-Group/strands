package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pinnacle-Solutions-Group/strands/internal/db"
)

func newIngestCmd() *cobra.Command {
	var topic, sessionID string
	var private bool
	var privateReason string
	var tagSpecs []string
	var beadSpecs []string

	cmd := &cobra.Command{
		Use:   "ingest [file]",
		Short: "Insert a strand from a file or stdin",
		Long: "Reads a markdown chunk from the given file (or stdin if '-') and " +
			"stores it as a new strand. If --session is omitted, a fresh session " +
			"is created automatically so ad-hoc ingests do not orphan rows.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic = strings.TrimSpace(topic)
			if topic == "" {
				return fmt.Errorf("--topic is required")
			}

			body, err := readBody(args[0])
			if err != nil {
				return err
			}
			if strings.TrimSpace(body) == "" {
				return fmt.Errorf("strand body is empty")
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

			if private && strings.TrimSpace(privateReason) == "" {
				return fmt.Errorf("--private requires --reason")
			}

			tags := make([][2]string, 0, len(tagSpecs))
			for _, raw := range tagSpecs {
				t, v, err := db.ParseTag(raw)
				if err != nil {
					return err
				}
				tags = append(tags, [2]string{t, v})
			}
			beads := make([][2]string, 0, len(beadSpecs))
			for _, raw := range beadSpecs {
				b, r, err := db.ParseBeadSpec(raw)
				if err != nil {
					return err
				}
				beads = append(beads, [2]string{b, r})
			}

			if sessionID == "" {
				sessionID, err = db.CreateSession(conn, cwd)
				if err != nil {
					return fmt.Errorf("create session: %w", err)
				}
			}

			var id string
			if private {
				id, err = db.CreatePrivateStrand(conn, cwd, sessionID, topic, body, privateReason)
				if err != nil {
					return fmt.Errorf("create private strand: %w", err)
				}
			} else {
				id, err = db.CreateStrand(conn, sessionID, topic, body)
				if err != nil {
					return fmt.Errorf("create strand: %w", err)
				}
			}

			for _, t := range tags {
				if err := db.AddStrandTag(conn, id, t[0], t[1]); err != nil {
					return err
				}
			}
			for _, b := range beads {
				if err := db.LinkStrandToBead(conn, id, b[0], b[1]); err != nil {
					return err
				}
			}

			if private {
				fmt.Printf("ingested private strand %s (session %s)\n", id, sessionID)
				fmt.Printf("body stored at %s\n", db.PrivateFilePath(cwd, id))
			} else {
				fmt.Printf("ingested strand %s (session %s)\n", id, sessionID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "topic label for the strand (required)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session id; a new session is created if omitted")
	cmd.Flags().BoolVar(&private, "private", false, "write the body straight to the sidecar store; must be combined with --reason")
	cmd.Flags().StringVar(&privateReason, "reason", "", "reason when using --private")
	cmd.Flags().StringArrayVar(&tagSpecs, "tag", nil, "provenance tag 'type' or 'type:value' (repeatable); type ∈ read|user|corrected|inferred|tested|narrative")
	cmd.Flags().StringArrayVar(&beadSpecs, "bead", nil, "bead link 'bd-id' or 'bd-id:relation' (repeatable); relation ∈ produced|discussed|blocked-on|discovered, default discussed")
	_ = cmd.MarkFlagRequired("topic")
	return cmd
}

func readBody(path string) (string, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

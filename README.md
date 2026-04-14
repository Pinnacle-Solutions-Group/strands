# strands

Conversation shelving for Claude Code sessions.

`strands` stores distilled, topic-grouped conversation chunks in a local SQLite
database so long agent sessions can offload context without losing the
narrative. Each strand carries provenance tags, optional links to
[beads](https://github.com/steveyegge/beads) (`bd-`) issue IDs, and is indexed
by FTS5 for fast full-text retrieval. Sensitive chunks can be marked private so
their bodies live in a sidecar file store instead of the main db.

It is a single pure-Go binary — no CGO, no external services.

## Install

```bash
go install github.com/kevinmrohr/strands/cmd/strands@latest
```

Or from a checkout:

```bash
go build -o strands ./cmd/strands
```

## Quick start

```bash
# Create .strands/ and the database in the current repo
strands init

# Ingest a markdown chunk as a new strand
strands ingest chunk.md \
  --topic "auth middleware refactor" \
  --tag read:src/middleware/auth.ts \
  --tag user \
  --bead bd-42:produced

# Or pipe from stdin
cat chunk.md | strands ingest - --topic "db schema changes"

# Browse and search
strands list
strands list --bead bd-42
strands search 'topic:auth OR jwt*'
strands show <id>
```

## Data model

- **sessions** — one per `strands init`-scoped workdir, auto-created on ad-hoc
  ingests so rows are never orphaned.
- **strands** — topic + body + `created_at`, indexed by an FTS5 virtual table
  over `(topic, body)`.
- **strand_tags** — provenance tags: `read`, `user`, `corrected`, `inferred`,
  `tested`, `narrative`. `--tag type` or `--tag type:value`, repeatable.
- **strand_bead_links** — soft references to beads issues with a relation:
  `produced`, `discussed` (default), `blocked-on`, `discovered`. Not validated
  against a live beads db.
- **private_flags** — strands whose body is stored on disk at
  `.strands/private/<id>.md` rather than in SQLite. The `body` column is
  cleared so FTS cannot index it.

Schema lives at `internal/db/schema.sql` and is embedded in the binary via
`go:embed`.

## Commands

| Command | Purpose |
|---|---|
| `strands init` | Create `.strands/` and initialize the database. Non-idempotent — refuses to re-init so accidental runs are visible. |
| `strands ingest <file\|->` | Insert a strand from a file or stdin. Requires `--topic`. |
| `strands list` | List strands newest first. Filter with `--bead <id>`. |
| `strands show <id>` | Show a single strand by ID or unique prefix. |
| `strands search <query>` | FTS5 `MATCH` over topics and bodies. Supports phrases, `topic:` scoping, prefix (`foo*`), booleans. |
| `strands link <strand> <bd-id>` | Attach a strand to a beads issue. `--relation` defaults to `discussed`. |
| `strands private <id>` | Move an existing strand's body into the sidecar store and flag it private. Requires `--reason`. |

Run any command with `-h` for full flag reference.

## Privacy

Private strands never have their body in SQLite — it lives at
`.strands/private/<id>.md`. Listings and searches still return the topic, but
search results filter out private rows at query time with a `LEFT JOIN`
against `private_flags`, because FTS5's external-content mode cannot cleanly
remove already-indexed rows. Add `.strands/private/` to `.gitignore` if you
don't want the sidecar committed.

## Project layout

```
cmd/strands/       # cobra CLI entry points
internal/db/       # schema, open/init, CRUD, FTS search, private store
internal/ids/      # strand id generation
```

## Development

```bash
go build ./...
go test ./...
```

Issue tracking is done with [beads](https://github.com/steveyegge/beads); run
`bd ready` to see open work.

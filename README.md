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

**Recommended (macOS / Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/Pinnacle-Solutions-Group/strands/main/scripts/install.sh | bash
```

The installer downloads the latest prebuilt binary from GitHub releases,
verifies its SHA256 against `checksums.txt`, and installs it to
`/usr/local/bin` if writable, otherwise `$HOME/.local/bin`. Both are on the
default macOS PATH (via `path_helper`) even in non-interactive shells, so the
Claude Code `SessionStart` hook can find `strands` without any profile magic.

**With Go (any platform):**

```bash
go install github.com/Pinnacle-Solutions-Group/strands/cmd/strands@latest
```

Installs to `$GOBIN` (or `$(go env GOPATH)/bin`, default `~/go/bin`). Make
sure that directory is on your `PATH` in the non-interactive shell environment
Claude Code hooks run under, or the SessionStart TOC hook will silently fail.

**From a checkout:**

```bash
go build -o strands ./cmd/strands
```

## Quick start

```bash
# Create .strands/ and the database in the current repo.
# On a TTY, init also offers to install a Claude Code SessionStart hook
# that injects the strand TOC into every new session — see "Claude Code
# session TOC" below. Non-interactive shells default to --limit 0.
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

## Claude Code session TOC

strands can install a `SessionStart` hook into your Claude Code settings so
every new session starts with your strand list injected as in-context
additional context. The TOC acts as a lightweight lookup table: Claude sees
every strand's id, creation time, and topic, and pulls bodies on demand with
`strands show <id>`. It replaces the file-based `toc.md` flow from
[context-shelf](https://github.com/Pinnacle-Solutions-Group/context-shelf).

**Recommended: install once, globally.**

```bash
strands install-hook             # writes to ~/.claude/settings.json (default)
strands install-hook --limit 200 # cap the TOC at 200 entries
```

The global hook is guarded with a `.strands/strands.db` existence check, so it
silently no-ops in any project that hasn't run `strands init`. Run `strands
init` in each repo where you want the TOC — from that point on every new
Claude session there auto-loads the strand list.

**Project-local install**, if you only want the hook in one repo:

```bash
strands install-hook --global=false   # writes to ./.claude/settings.json
```

**From `strands init`.** On a TTY, `strands init` prompts for the `--limit`
value interactively and offers to install the hook at the same time. The
install is project-local from `init` — use `strands install-hook` afterward if
you want the global version instead (or in addition).

- `0` — show all strands (recommended — unbounded like the old toc.md).
- `N` — cap at the N most recent strands if history gets noisy later.
- `s` — skip; you can install the hook later with `strands install-hook`.

Non-interactive init (CI, scripts) defaults to `--limit 0`. Flags:

```bash
strands init --limit 200   # bypass the prompt
strands init --no-hook     # skip hook install entirely
```

`install-hook` is idempotent regardless of scope. It merges into any existing
`settings.json` without touching unrelated hooks or other top-level fields,
and re-running with a different `--limit` replaces the prior strands hook in
place rather than appending a duplicate. If you install both globally and
project-locally, the TOC will print twice per session — pick one.

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
| `strands init` | Create `.strands/` and initialize the database. Non-idempotent — refuses to re-init so accidental runs are visible. On a TTY, also offers to install the Claude Code SessionStart hook. |
| `strands install-hook` | Install or update the Claude Code `SessionStart` hook that injects the strand TOC as in-context additional context. Defaults to global (`~/.claude/settings.json`); pass `--global=false` for project-local. Guarded with a `.strands/strands.db` existence check so it no-ops in non-strands projects. Idempotent — safe to re-run with a new `--limit`. |
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
cmd/strands/          # cobra CLI entry points
internal/db/          # schema, open/init, CRUD, FTS search, private store
internal/ids/         # strand id generation
internal/claudehook/  # Claude Code SessionStart hook installer
```

## Development

```bash
go build ./...
go test ./...
```

Issue tracking is done with [beads](https://github.com/steveyegge/beads); run
`bd ready` to see open work.

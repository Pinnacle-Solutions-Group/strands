# strands — design plan

Personal tool for conversation shelving, designed as a sibling to [beads](https://github.com/steveyegge/beads).
Stores distilled, topic-grouped conversation chunks ("strands") with provenance tags, linked to `bd-` issue IDs.

## Why this exists

Beads tracks the work graph (issues, dependencies, status). It does not track the *narrative* —
the reasoning, dead ends, discoveries, and corrections that produced the work. Context Shelf
(the markdown-based predecessor) solved this with files in `.claude/history/`, but that approach
can't be merged, queried, or linked to issues.

strands moves conversation memory into a real database, so that:

- Multiple agents/sessions can merge their chunks the same way beads merges issues (via Dolt, eventually).
- Chunks can reference `bd-` IDs, tying the narrative directly to the work graph.
- Private content can be filtered at write time rather than by naming convention.
- `strands list` / `strands show` replaces a markdown TOC that rots as it grows.

**Non-goals:** plan lifecycle, task tracking, dependency graphs. Beads already does all of that.

## Architecture sketch

**Sibling tool, separate binary (`strands`), separate repo.** No changes to beads itself.

Open question: **own database, or sidecar table in `.beads/beads.db`?**

- **Own db** (`.strands/strands.db`): clean isolation, independent release cadence, can be used in repos without beads. Tradeoff: foreign-key references to `bd-` IDs become advisory only.
- **Sidecar in beads.db**: single file, stronger referential integrity if we add FKs, merges in lockstep with beads history. Tradeoff: couples us to beads' schema/migrations, and beads may not welcome unknown tables in its db file.

Leaning toward **own db** for the prototype — decouple first, consider merging later if it proves valuable.

## Data model (first draft)

```
sessions
  id              TEXT PRIMARY KEY    -- ULID or timestamp
  started_at      TIMESTAMP
  ended_at        TIMESTAMP NULL
  workdir         TEXT                -- where the session ran
  summary         TEXT NULL           -- one-liner, filled on shelf

strands
  id              TEXT PRIMARY KEY
  session_id      TEXT REFERENCES sessions(id)
  topic           TEXT                -- "Auth middleware refactor"
  body            TEXT                -- distilled markdown content
  created_at      TIMESTAMP

strand_tags                            -- provenance: [read:path], [user], [corrected], etc.
  strand_id       TEXT
  tag_type        TEXT                -- read | user | corrected | inferred | tested | narrative
  tag_value       TEXT NULL           -- e.g. the file path for [read:...]

strand_bead_links                      -- link strands to beads issues
  strand_id       TEXT
  bead_id         TEXT                -- e.g. "bd-42" — advisory reference, not enforced
  relation        TEXT                -- produced | discussed | blocked-on | discovered

private_flags                          -- content the user chose to keep out of main db
  strand_id       TEXT
  reason          TEXT
  -- actual private body stored in .strands/private/<id>.md (gitignored)
```

## CLI surface (first pass)

```
strands init                    # create .strands/ and db
strands shelf                   # distill current conversation into strands (invoked by hook)
strands list [--session <id>]   # list strands, newest first
strands show <id>               # print a strand's body
strands link <strand> <bd-id>   # add a strand ↔ bead link
strands search <query>          # FTS over bodies and topics
strands private <id>            # move a strand to the private store
```

No `strands delete`. Appending only — edits create new revisions. (Open question: do we need revisions, or is immutable-append enough?)

## Open questions

1. **Shelving trigger.** Hook-driven (PreCompact), slash-command-driven (`/strands-shelf`), or both? Context Shelf does both; that probably still makes sense.
2. **Who writes the chunks?** The model does, same as Context Shelf. strands is storage + query, not distillation logic. The distillation prompt lives in a skill/command, not in the binary.
3. **Private content.** Scan at write time (model's job) or at query time (binary's job)? Probably both: model flags on write, binary refuses to print flagged strands without `--include-private`.
4. **Beads linkage enforcement.** Soft references only, or validate `bd-` IDs against an actual beads db when available? Start soft.
5. **Language.** Go, to match beads and keep a single-binary distribution story. (Tentative — Python would be faster to prototype but harder to distribute.)
6. **Name collision.** `strands` is fairly generic — check if there's anything meaningful on crates.io / pypi / go modules before committing to it publicly.

## Rough milestones

- **M0 — schema & `strands init`**: just create the db and tables. Prove the data model.
- **M1 — manual write/read**: `strands show`, a `strands ingest <file.md>` for dumping a hand-written chunk in. No distillation yet.
- **M2 — shelving skill**: port the Context Shelf distillation prompt into a Claude Code skill that writes via `strands ingest`.
- **M3 — linkage**: `strands link`, `strands list --bead bd-42`. This is when the beads integration becomes real.
- **M4 — search**: FTS5 over bodies.
- **M5 — private store**: flag + sidecar file handling.
- **M6 — eject decision**: by this point, do we know whether this wants to stay sibling-shaped, merge into beads, or become a proper Dolt-backed multi-agent tool?

## What this plan deliberately does not cover

- Dolt migration — premature. SQLite first, Dolt if/when multi-agent merge becomes a real need.
- UI / TUI — CLI only for now.
- Syncing across machines — out of scope; it's a local dev tool.
- Anything resembling plan lifecycle or dependency tracking — that's beads' job, and duplicating it is how Context Shelf ended up 30% redundant.

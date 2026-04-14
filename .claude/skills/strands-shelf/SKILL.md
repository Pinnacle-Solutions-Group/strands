---
name: strands-shelf
description: Shelve conversation history to a strands database. Distills the current conversation into topic-grouped chunks with provenance tags and beads links, writing each via 'strands ingest'. Scans for private/sensitive content first and offers to use --private so those chunks never hit the main db.
---

Run the strands shelving process now. This skill replaces the markdown-based `shelf` skill for projects that use the `strands` binary — it produces the same kind of distilled, topic-grouped record, but each chunk becomes a row in `.strands/strands.db` instead of a file in `.claude/history/`.

## Prerequisites

Before doing anything, verify that strands is usable from this directory:

```bash
strands list --limit 1 >/dev/null 2>&1 || echo "need init"
```

If the db does not exist yet, ask the user whether to run `strands init` here. Do not initialize silently — strands creates `.strands/` alongside `.beads/` and the user may not want that in every repo.

## Step 1 — scan for sensitive content

**Before distilling anything**, scan the conversation for content that should not land in the main database. Use the same categories as the global CLAUDE.md shelving instructions: client/company names in a non-public context, financial data, pricing strategy, competitive intelligence, business strategy, personal information about named individuals, legal/compliance discussions, credentials or secrets.

If you find any, tell the user specifically what you flagged and why, and ask:

```
I found potentially sensitive content in this conversation:
- <specific thing> (<category>)
- <specific thing> (<category>)

I can:
1. Ingest sensitive chunks with --private --reason "…" (body goes to
   .strands/private/<id>.md, gitignored, never FTS-indexed)
2. Ingest everything as normal strands
3. Let me review your flagged list before choosing

Which would you like?
```

Wait for their answer before continuing. **Never** put sensitive content into a regular strand — always use `--private --reason "..."` when in doubt.

## Step 2 — distill by topic

Group the conversation since the last shelf (or the whole conversation if this is the first shelf) into **topics**, not a timeline. Same rules as the file-based shelf:

- **Dedup**: same question asked multiple times → store once with the final answer
- **Collapse noise**: rambling, false starts, corrections → keep the conclusion
- **Preserve signal**: decisions, rationale, discoveries, code changes, errors and fixes
- **Structure by topic**: group related exchanges even if they were interleaved in time

For each topic, decide:

- A short **topic** label (will become `--topic`)
- The distilled **body** (markdown)
- Which **provenance tags** apply (become `--tag` flags):
  - `read:<path>` — learned by reading code or a file
  - `user` — the user stated this directly
  - `corrected` — was initially wrong and got corrected during the conversation
  - `inferred` — derived from other facts
  - `tested` — verified by running code or tests
  - `narrative` — the journey of discovery itself matters, keep it as a mini-narrative
- Which **beads issues** the chunk relates to (become `--bead` flags):
  - `bd-N:produced` — the chunk documents work that produced this issue
  - `bd-N:discussed` — the chunk is a conversation about this issue (default if relation unclear)
  - `bd-N:blocked-on` — the work in this chunk is blocked waiting on this issue
  - `bd-N:discovered` — this issue was discovered via the work in this chunk

If the chunk is private per Step 1, also add `--private --reason "..."`.

## Step 3 — write each chunk

For each topic, invoke `strands ingest` via the Bash tool. Pass the body on stdin so you do not have to create temporary files:

```bash
cat <<'BODY' | strands ingest - \
  --topic "Auth middleware refactor" \
  --tag "read:src/middleware/auth.ts" \
  --tag "user" \
  --tag "corrected" \
  --bead "bd-42:produced" \
  --bead "bd-99:discussed"
- Uses JWT tokens for session management [read: src/middleware/auth.ts]
- Switching to HttpOnly cookies [user: legal requirement]
- Originally assumed Redis sessions, corrected by user [corrected]
BODY
```

Notes:

- Use a single `strands ingest` call per topic. Do not split a topic across multiple calls.
- Inline tags like `[read: path]` in the body are fine for human readability — the structured `--tag` flags are what populates `strand_tags` for querying.
- Reuse the same session id across all chunks from one shelf: capture the id from the first ingest output (format: `ingested strand <id> (session <id>)`) and pass `--session <id>` to subsequent calls so they all group together.
- For private chunks, add `--private --reason "<specific reason>"`. The body body still goes on stdin; strands writes it to `.strands/private/<id>.md` instead of the body column.

## Step 4 — report what you wrote

After all chunks are ingested, print a short summary to the user:

```
Shelved N strands to .strands/strands.db:
  - <topic 1> (bd-42, bd-99)
  - <topic 2>
  - [private] <topic 3>  — reason: <reason>

Session id: <session>
Run `strands list` or `strands search <query>` to retrieve.
```

Do not create a markdown TOC file — that is the job of the old `shelf` skill. Strands replaces the TOC with `strands list` / `strands search`.

## Installing this skill globally

This skill lives inside the strands repo at `.claude/skills/strands-shelf/`. To use it from any project, symlink or copy it into `~/.claude/skills/strands-shelf/` so Claude Code picks it up globally.

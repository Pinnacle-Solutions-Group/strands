# Contributing to strands

Thanks for your interest in contributing! This document covers how to report
issues, set up a dev environment, and send pull requests.

## Reporting issues

- **Bugs**: open a [bug report](https://github.com/Pinnacle-Solutions-Group/strands/issues/new?template=bug_report.md).
  Include your OS, `strands --version` (or commit SHA if built from source),
  and the minimum commands needed to reproduce.
- **Features**: open a [feature request](https://github.com/Pinnacle-Solutions-Group/strands/issues/new?template=feature_request.md)
  describing the use case before starting work on a large change — it's the
  fastest way to confirm the idea fits the project's scope.
- **Security**: do **not** open a public issue. See [SECURITY.md](SECURITY.md).

## Development setup

You need Go 1.26 or newer. No CGO toolchain is required — strands uses the
pure-Go `modernc.org/sqlite` driver.

```bash
git clone https://github.com/Pinnacle-Solutions-Group/strands.git
cd strands
go build ./...
go test ./...
```

To run your local build:

```bash
go run ./cmd/strands --help
```

## Project layout

```
cmd/strands/          cobra CLI entry points
internal/db/          schema, open/init, CRUD, FTS search, private store
internal/ids/         strand id generation
internal/claudehook/  Claude Code SessionStart hook installer
```

The SQLite schema is embedded via `go:embed` from `internal/db/schema.sql`.

## Pull request guidelines

1. **Discuss large changes first.** For anything beyond a small bug fix or
   docs tweak, open an issue so we can agree on the approach before you spend
   time on it.
2. **Keep PRs focused.** One logical change per PR. Unrelated refactors
   belong in separate PRs.
3. **Tests.** Add or update tests for any behavior change. `go test ./...`
   must pass.
4. **Formatting.** Run `gofmt -s -w .` before committing. `go vet ./...`
   should be clean.
5. **Commit messages.** Use imperative mood (`add X`, not `added X`).
   Prefix with an area when useful (`db:`, `cli:`, `hook:`).
6. **No breaking changes without discussion.** strands is pre-1.0 but the
   on-disk schema and CLI surface are treated as stable — changes need a
   migration story.

## DCO / sign-off

Contributions are accepted under the project's [MIT license](LICENSE). By
submitting a pull request you agree that your contribution may be distributed
under those terms.

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By
participating you agree to abide by its terms.

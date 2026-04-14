# Red Team Review: qql-go

## Status

The findings from the earlier red-team pass have been addressed in this cleanup round.

## What was fixed

- CLI/process control:
  - command handlers no longer call `os.Exit`
  - process exit is centralized in `cmd/qql/main.go`
  - command errors now flow through typed exit metadata

- CLI stability:
  - rerank prefetch amplification is capped
  - collection readiness failures are more precise and less misleading
  - shared config/client loading paths reduced duplication in command handling

- Config and output:
  - config persistence was simplified to direct JSON I/O
  - missing-config behavior is more consistent
  - output writing is fully injectable instead of partially hardwired to global stdio

- Parser/runtime:
  - lossy hand-rolled numeric parsing was replaced with safer parsing
  - duplicated `USING ...` parsing logic was consolidated
  - search suffix parsing is more consistent and rejects duplicate clauses
  - REPL mixed-case `EXPLAIN` handling was fixed
  - multiline REPL input no longer gets stuck on stray closing delimiters

- Filter conversion:
  - duplicated pointer/value dispatch in filter conversion was collapsed

- Docs and skills:
  - `README.md` is now the canonical public capability surface
  - skill docs were aligned with the actually supported syntax
  - stale capability duplication was reduced
  - tracked generated `__pycache__` artifacts were removed
  - demo scripts no longer hide collection-drop failures
  - the no-op `--execute` flag was removed from the retrieval demo

- Tests and coverage:
  - coverage and edge-case testing were expanded across CLI, config, output, parser, filters, and REPL
  - parser precedence tests now assert deeper structure instead of only broad shape

## Verification

- `go test ./...`
- `go test -cover ./...`
- `go build -o qql.exe ./cmd/qql`
- `qql.exe version --quiet`
- `qql.exe explain --quiet "SHOW COLLECTIONS"`
- `qql.exe explain --quiet --json "SEARCH docs SIMILAR TO 'vector db' LIMIT 5 USING HYBRID RERANK"`

All of the above passed in this pass.

## Remaining limits

- Live Qdrant network paths were not exercised against a real backend in this pass.
- Coverage still has room to grow in `cmd/qql`, `internal/errors`, and parts of `internal/cli/commands`.

# Development Guide

This guide is for maintainers and contributors working on the qql-go codebase.

## Architecture

qql-go is a thin translation layer between a SQL-like query language (QQL) and the Qdrant Go client. Every statement goes through three stages:

```
User input
    |
    v
[ Lexer ]      — tokenizes input into keywords, identifiers, literals
    |
    v
[ Parser ]     — builds a typed AST node (e.g., InsertStmt, SearchStmt)
    |
    v
[ Executor ]   — maps the AST node to Qdrant client calls
    |
    v
[ Output ]     — formats results for humans (--quiet, --json)
```

## Repo Layout

```
cmd/qql-go/                    CLI entrypoint
internal/
  ast/                         AST node definitions (RecommendStmt, SearchStmt, etc.)
  lexer/                       Tokenizer and token kinds
  parser/                      Recursive descent parser
  filters/                     QQL WHERE clause → Qdrant Filter conversion
  cli/commands/                Command handlers (connect, exec, explain, etc.)
  repl/                        Interactive shell
  embedding/                   OpenAI-compatible embedding client
  sparse/                      BM25 sparse vector generation
  script/                      .qql script execution
  dump/                        Collection dump to .qql files
  config/                      Connection config persistence
  output/                      Terminal output formatting
skills/qql-skill/              Public agent skill package
docs/releases/                 Release notes
.github/workflows/             CI and release automation
```

## Inference Modes

qql-go supports three inference modes:

- **cloud** (default) — Uses Qdrant Cloud's server-side inference via `qdrant.Document` objects.
- **local** — Generates dense and sparse vectors client-side using a local OpenAI-compatible embeddings API (e.g., LM Studio, llamafile) and local Qdrant.
- **external** — Same vector-generation path as local, but points at remote services.

The mode is set at `connect` time and stored in `~/.qql/config.json`.

## Local Development Setup

### Prerequisites

- Go 1.24+
- A running Qdrant instance (Docker or native)
- For local mode: an OpenAI-compatible embedding server (e.g., LM Studio, Ollama, llamafile)

### Start Qdrant locally

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

### Connect in cloud mode

```bash
go run ./cmd/qql-go connect --url https://your-cluster.qdrant.io --secret your-api-key
```

### Connect in local mode

```bash
go run ./cmd/qql-go connect \
  --url http://localhost:6334 \
  --inference-mode local \
  --embedding-endpoint http://127.0.0.1:1234/v1/embeddings \
  --embedding-key your-api-key \
  --embedding-model text-embedding-all-minilm-l6-v2-embedding \
  --embedding-dimension 384
```

Note: `--embedding-key` is optional and should be supplied for hosted embedding providers that require bearer auth. `--embedding-dimension` is optional if the endpoint is reachable (auto-probed).

### Test and Build

```bash
go test ./...
go build ./cmd/qql-go
```

### Run the CLI

```bash
go run ./cmd/qql-go version
go run ./cmd/qql-go doctor
go run ./cmd/qql-go exec "SHOW COLLECTIONS"
go run ./cmd/qql-go repl
```

## Testing Strategy

### Unit Tests

All packages have unit tests. Run with:

```bash
go test ./...
```

Key test files:
- `internal/lexer/lexer_test.go` — Tokenization
- `internal/parser/parser_test.go` — Syntax parsing
- `internal/cli/commands/commands_test.go` — Command execution
- `internal/sparse/bm25_test.go` — Sparse vector generation
- `internal/embedding/client_test.go` — Embedding client

### Integration Tests

For end-to-end validation against a live Qdrant instance:

```bash
# Start Qdrant
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant

# Connect
go run ./cmd/qql-go connect --url http://localhost:6334

# Run operations
go run ./cmd/qql-go exec "CREATE COLLECTION test HYBRID"
go run ./cmd/qql-go exec "INSERT INTO COLLECTION test VALUES {'text': 'hello'} USING HYBRID"
go run ./cmd/qql-go exec "SEARCH test SIMILAR TO 'hello' LIMIT 5 USING HYBRID"
go run ./cmd/qql-go exec "RECOMMEND FROM test POSITIVE IDS ('...') LIMIT 5"
```

### What to Test

| Change | Required Tests |
|---|---|
| New QQL syntax | Parser tests + at least one executor test |
| New CLI command | Command handler tests |
| Output format change | JSON output contract tests |
| Inference mode change | Both cloud and local path tests |
| Filter behavior | Filter conversion tests |

## Core Rules

- Keep the implementation aligned with the syntax documented in [README.md](../README.md).
- Prefer surgical changes over broad refactors.
- Add tests for parser and CLI behavior changes.
- Keep output contracts stable, especially `--json` and `--quiet --json`.
- Match existing Go style (gofmt, standard library patterns).
- Do not add speculative syntax or features not implemented end-to-end.

## Vector Generation

### Dense Vectors

Generated via the `internal/embedding` package using an OpenAI-compatible `/v1/embeddings` endpoint. The client validates dimensions strictly.

### Sparse Vectors

Generated via `internal/sparse` using:
- Rust-compatible tokenization (Unicode letters/digits/underscore, length >= 2)
- Length-prefixed FNV-1a hash to avoid prefix collisions
- BM25 weighting with corpus statistics stored in `~/.qql/corpus/<collection>.json`
- Log-TF weighting for queries

Corpus stats are cleaned up on `DROP COLLECTION` and rebuilt if corrupted.

## Versioning

The CLI version is defined in:

- `internal/cli/commands/commands.go` (var `Version`)
- `VERSION` file in repo root

Release notes live in:

- `docs/releases/<version>.md`

Changelog entries live in:

- [CHANGELOG.md](../CHANGELOG.md)

When preparing a release, update all three together.

## CI

CI lives in `.github/workflows/`:

- `ci.yml` — runs `go test ./...` and builds the CLI on Ubuntu, macOS, and Windows
- `release.yml` — tag-driven release automation

## Release Process

For the full release ritual, see [docs/RELEASE.md](RELEASE.md).

Quick reference:

```bash
# Prepare release files
go run docs/dev_tasks.go prepare-release --version 0.2.0

# Run quality gate
go run docs/dev_tasks.go check

# Validate release build
go run docs/dev_tasks.go release-validate

# Tag and push
git tag -a v0.2.0 -m "qql-go v0.2.0"
git push origin main
git push origin v0.2.0
```

## Release Automation

Tag-driven via `.github/workflows/release.yml`:

- Pushing a tag matching `v*` builds release archives
- Archives are published to GitHub Releases for Windows, Linux, and macOS
- A checksum file is attached to the release

Important:

- Do not manually create the GitHub release first.
- Push the commit, then push the tag, and let the workflow publish assets.
- The workflow safely updates existing releases if needed.

## Skills Maintenance

Skills published by this repo live under `skills/`.

Validate local skill discovery:

```bash
npx skills add . --list
```

Install a local skill copy for testing:

```bash
npx skills add . --skill qql-skill --copy
```

Keep skill docs small and point back to [README.md](../README.md) for the canonical feature surface. Update skills whenever:
- New QQL syntax is added
- Inference modes change
- CLI commands are added or removed

## Debugging Tips

### Enable verbose output

```bash
go run ./cmd/qql-go exec --json "SEARCH docs SIMILAR TO 'hello' LIMIT 5"
```

### Check corpus stats

```bash
cat ~/.qql/corpus/<collection>.json
```

### Explain without executing

```bash
go run ./cmd/qql-go explain "SEARCH docs SIMILAR TO 'hello' LIMIT 5 USING HYBRID"
```

### Test embedding endpoint directly

```bash
curl http://127.0.0.1:1234/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"model": "text-embedding-all-minilm-l6-v2-embedding", "input": "test"}'
```

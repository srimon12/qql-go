# Contributing to qql-go

Thank you for contributing to qql-go. This document covers how to get started, what we expect in contributions, and how to submit changes.

## About this project

qql-go is an independent Go port of the original QQL work from [pavanjava/qql](https://github.com/pavanjava/qql). It is a thin translation layer between a SQL-like query language (QQL) and the Qdrant vector database. Contributions should stay aligned with the current Go CLI surface documented in [README.md](README.md).

## Before you start

- **Open an issue or discussion first** for large changes (new QQL syntax, major refactors, new inference modes).
- **Check existing issues** to avoid duplicate work.
- **Keep changes narrow and user-facing.** We prefer surgical PRs over broad refactors.
- **Do not add speculative syntax or features** that are not implemented end-to-end.
- **Treat `README.md` as the public contract** for supported behavior.

## Development setup

### Requirements

- **Go 1.24+**
- **Git**
- **Docker** (for running Qdrant locally during integration testing)
- **GitHub CLI** (only if you work on release automation)
- **Python/uv** (optional, only if you want to run demo scripts under `skills/qql-skill/scripts`)

### Quick start

```bash
# Clone
git clone <repo-url>
cd qql-go

# Verify you can build and test
go test ./...
go build ./cmd/qql-go
```

### Running locally

```bash
# Start Qdrant
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant

# Connect (adjust for your setup)
go run ./cmd/qql-go connect --url http://localhost:6334

# Try commands
go run ./cmd/qql-go version
go run ./cmd/qql-go doctor
go run ./cmd/qql-go exec "SHOW COLLECTIONS"
go run ./cmd/qql-go repl
```

For full local development details (cloud vs local mode, embedding setup, etc.), see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Code style

- Run `gofmt` on all changed Go files.
- Follow standard Go conventions (short variable names, explicit error handling, no unnecessary abstractions).
- Keep packages focused and avoid circular imports.
- Match existing patterns in the codebase.

## Testing

### Required

All PRs must pass:

```bash
go test ./...
```

### What to test

| Change | Required tests |
|---|---|
| New QQL syntax | Parser tests + at least one executor test |
| New CLI command | Command handler tests |
| Output format change | JSON output contract tests (check `--json` and `--quiet --json`) |
| Inference mode change | Tests for both cloud and local paths |
| Filter/WHERE behavior | Filter conversion unit tests |
| Bug fix | A test that would fail without the fix |

### Integration testing

For changes that touch the Qdrant client path, validate against a live instance:

```bash
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
go run ./cmd/qql-go connect --url http://localhost:6334
# Run your new feature end-to-end
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the full integration testing guide.

## What to include in a change

Every PR should include:

1. **Code changes** for the behavior you are adding or fixing.
2. **Tests** for parser, CLI, output, or REPL behavior when applicable.
3. **README updates** if the public surface changed (new commands, new flags, new QQL syntax).
4. **Changelog entry** only when preparing a release or when maintainers ask for it.

## Pull request guidelines

- **Keep PRs small and focused.** One logical change per PR.
- **Write a clear title and description.** Explain the user-visible change.
- **Include example commands** when changing CLI syntax or output.
- **Mention limitations.** If a feature works in cloud mode but not local mode (or vice versa), state it clearly.
- **Reference issues.** Link to any related issue or discussion.
- **Ensure CI passes.** All tests and builds must be green before review.

### PR review process

1. A maintainer will review within a few days.
2. Address feedback with additional commits (do not force-push unless asked).
3. Once approved, a maintainer will merge.

## Contributing to skills

Public skills live under `skills/`.

Before opening a PR that changes skills:

1. Validate discovery locally:

```bash
npx skills add . --list
```

2. Install and test the skill:

```bash
npx skills add . --skill qql-skill --copy
```

3. Ensure skill docs stay small and point back to [README.md](README.md) for the canonical feature surface.

Skill-specific authoring notes live in [docs/SKILLS.md](docs/SKILLS.md).

## Releases

- Do not manually create GitHub releases for normal tagged releases.
- The release workflow is designed to publish assets when a `v*` tag is pushed.
- Full release ritual lives in [docs/RELEASE.md](docs/RELEASE.md).

## Getting help

- Open a **discussion** for questions about the project direction or feature ideas.
- Open an **issue** for bug reports or concrete feature requests.
- Tag `@maintainers` if something is urgent.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

# Development Guide

This guide is for maintainers and contributors working on the QQL-Go codebase and release process.

## Repo layout

Main areas:

- `cmd/qql` entrypoint
- `internal/lexer` tokenization
- `internal/parser` syntax parsing
- `internal/filters` QQL filter conversion
- `internal/cli` and `internal/cli/commands` command handling
- `internal/repl` interactive shell behavior
- `skills/` public agent skills published from this repository
- `.github/workflows/` CI and release automation

## Core rules

- Keep the implementation aligned with the syntax documented in [README.md](../README.md).
- Prefer surgical changes over broad refactors.
- Add tests for parser and CLI behavior changes.
- Keep output contracts stable, especially `--json` and `--quiet --json`.

## Local development

Test and build:

```bash
go test ./...
go build ./cmd/qql
```

Run the CLI:

```bash
go run ./cmd/qql version
go run ./cmd/qql repl
```

Build a local binary:

```bash
go build -o qql.exe ./cmd/qql
```

On non-Windows platforms:

```bash
go build -o qql ./cmd/qql
```

## Versioning

The CLI version string is defined in:

- [internal/cli/commands/commands.go](../internal/cli/commands/commands.go)

Release notes live in:

- `docs/releases/<version>.md`

Changelog entries live in:

- [CHANGELOG.md](../CHANGELOG.md)

When preparing a release, update all three together.

## CI

CI lives in:

- [.github/workflows/ci.yml](../.github/workflows/ci.yml)

It currently:

- runs `go test ./...`
- builds the CLI on Ubuntu, macOS, and Windows

## Release automation

Release automation lives in:

- [.github/workflows/release.yml](../.github/workflows/release.yml)

It is tag-driven:

- pushing a tag matching `v*` builds release archives
- archives are published to GitHub Releases for Windows, Linux, and macOS
- a checksum file is attached to the release

Important:

- Do not manually create the GitHub release first for a tagged release.
- Push the commit, then push the tag, and let the workflow publish the assets.
- The workflow is written to update an existing release safely if it already exists, but the intended path is automation-first.

## Release checklist

1. Update the version in `internal/cli/commands/commands.go`.
2. Update [CHANGELOG.md](../CHANGELOG.md).
3. Add or update `docs/releases/<version>.md`.
4. Run:

```bash
go test ./...
go build ./cmd/qql
```

5. Commit the release prep changes.
6. Tag the release:

```bash
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

7. Verify the GitHub Actions release workflow and release page assets.

## Skills maintenance

Skills published by this repo should live under `skills/`.

Validate local skill discovery with:

```bash
npx skills add . --list
```

Install a local skill copy for testing:

```bash
npx skills add . --skill qql-skill --copy
```

Keep skill docs small and point back to [README.md](../README.md) for the canonical feature surface.

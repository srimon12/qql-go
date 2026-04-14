# Contributing

Thanks for contributing to qql-go.

This project is an independent Go port of the original QQL work from [pavanjava/qql](https://github.com/pavanjava/qql). Contributions should stay aligned with the current Go CLI surface documented in [README.md](README.md).

## Before you start

- Open an issue or discussion first for large changes.
- Keep changes narrow and user-facing.
- Do not add speculative syntax or features that are not implemented end-to-end.
- Treat `README.md` as the public contract for supported behavior.

## Development setup

Requirements:

- Go 1.24+
- GitHub CLI if you work on release automation
- Optional: Python/uv if you want to run the demo scripts under `skills/qql-skill/scripts`

Clone and test:

```bash
go test ./...
go build ./cmd/qql-go
```

Run the CLI locally:

```bash
go run ./cmd/qql-go version
go run ./cmd/qql-go doctor
go run ./cmd/qql-go exec "SHOW COLLECTIONS"
```

## What to include in a change

- Code changes for the behavior you are adding or fixing
- Tests for parser, CLI, output, or REPL behavior when applicable
- README updates if the public surface changed
- Changelog entry only when preparing a release or when maintainers ask for it

## Pull request guidelines

- Keep PRs small and focused.
- Explain the user-visible change clearly.
- Mention any Qdrant Cloud versus self-hosted limitations that still apply.
- Include example commands when changing CLI syntax or output.

## Skills

Public skills live under `skills/`.

Before opening a PR that changes skills, validate discovery locally:

```bash
npx skills add . --list
```

Skill-specific authoring notes live in [docs/SKILLS.md](docs/SKILLS.md).

## Releases

- Do not manually create GitHub releases for normal tagged releases.
- The release workflow is designed to publish assets when a `v*` tag is pushed.
- Maintainer release steps live in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

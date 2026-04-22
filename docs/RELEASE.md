# Release Guide

This is the release ritual for `qql-go`.

## Release source of truth

Before tagging, these files must agree on the release version:

- `VERSION`
- `internal/cli/commands/commands.go` (var `Version`)
- `docs/releases/<version>.md`
- `CHANGELOG.md`

The CLI binary reports the version from `commands.go`.

## Prepare a release

Run from the repository root:

```bash
go run developer_guide/dev_tasks.go prepare-release --version 0.2.0
```

That updates:

- `VERSION`
- `internal/cli/commands/commands.go`
- `docs/releases/0.2.0.md` if it does not already exist
- `CHANGELOG.md` if the version entry does not already exist

Then replace any scaffold text with the real release notes.

## Verify before tag

Run the local quality gate:

```bash
go run developer_guide/dev_tasks.go check
```

That runs:

1. Version sync check (VERSION == commands.go)
2. `gofmt --check` on all Go files
3. `go vet ./...`
4. `go test ./...`
5. `go build ./cmd/qql-go`

Run the local release validator:

```bash
go run developer_guide/dev_tasks.go release-validate
```

That validator runs the full quality gate, then builds a release binary and checks:

- the binary version matches the checked-in version
- `--version` flag reports the correct version on the current platform

## Commit and tag

After the release-prep branch is ready and CI is green:

```bash
git checkout main
git pull
git tag -a v0.2.0 -m "qql-go v0.2.0"
git push origin main
git push origin v0.2.0
```

## What the release workflow does

The tag-triggered GitHub Actions workflow:

- verifies `VERSION` matches the tag
- verifies `commands.go` version matches the tag
- builds native release bundles for all platforms
- verifies the packaged binary responds to `--version`
- uploads release assets and checksums
- publishes or updates the GitHub Release using `docs/releases/<version>.md`

## Current release targets

Right now the release workflow publishes:

- Linux `amd64`
- Linux `arm64`
- Windows `amd64`
- macOS `arm64`

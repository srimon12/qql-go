# Changelog

All notable user-facing changes to this repository will be documented in this file.

The format is inspired by Keep a Changelog and uses calendar dates for repo releases.

## [Unreleased]

- No unreleased changes yet.

## [0.1.1] - 2026-04-14

### Changed

- Renamed the shipped CLI binary and release artifacts from `qql` to `qql-go`.
- Updated CLI help text, docs, skill references, and helper scripts to use `qql-go`.
- Made GitHub release publishing idempotent so reruns update assets instead of failing when a release already exists.

### Notes

- This is a maintenance follow-up to `0.1.0` focused on packaging, naming, and release automation polish.

## [0.1.0] - 2026-04-14

### Added

- Standalone Go CLI for Qdrant with `connect`, `disconnect`, `exec`, `explain`, `doctor`, `repl`, and `version` commands.
- SQL-like QQL support for collection management, inserts, search, explain plans, and deletes.
- Structured JSON output mode for script and agent workflows.
- Public `skills/qql-skill` package for agent installation through the `skills` CLI.
- Basic GitHub Actions CI that runs tests and verifies the CLI builds on push and pull request.
- Tagged release automation for publishing prebuilt binaries to GitHub Releases.
- Open-source repo docs for release notes, changelog tracking, and skill authoring.

### Notes

- In the current Go build, text `INSERT`, text `SEARCH ... SIMILAR TO ...`, `USING HYBRID`, and `RERANK` depend on Qdrant Cloud inference paths.
- Self-hosted or local Qdrant is currently best suited to non-inference operations such as collection and payload index management.

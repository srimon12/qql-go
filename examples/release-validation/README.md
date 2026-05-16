# Demo: Retrieval Regression Validation

This example is the real CI story.

It validates retrieval behavior against an existing Qdrant collection that already holds your production or staging dataset.

## Problem

You changed something around retrieval:

- embedding model
- chunking
- sparse or hybrid settings
- payload schema
- quantization
- rerank path

You need a fast regression job.

You do not want CI to:

- create a fresh Qdrant
- reinsert a large corpus
- recompute embeddings for 1M points

## What this demo does

- connects to an existing Qdrant
- optionally configures an external embeddings endpoint for query-time text embedding
- reads a checked-in regression suite
- runs `SHOW`, `SEARCH`, and `EXPLAIN` checks
- asserts result IDs, result counts, grouping, and plan text from JSON output
- writes machine-readable artifacts for diffing and incident triage

## Files

- `regression-suite.json`
  Edit this to match your real collection, queries, and expected IDs.
- `run-demo.sh`
  Self-contained Bash runner for CI and local verification.
- `run-demo.ps1`
  Self-contained PowerShell runner.
- `validate-artifacts.sh`
  Bash validator for the JSON artifacts.
- `validate-artifacts.ps1`
  PowerShell validator.
- `github-actions.yml`
  Workflow template for GitHub Actions using an existing remote Qdrant.

## How to run it

If you already have a saved `qql-go` connection that points at the right cluster:

```bash
bash examples/release-validation/run-demo.sh
```

If CI should connect explicitly:

```bash
export QDRANT_URL="https://<cluster>.qdrant.io"
export QDRANT_API_KEY="<api-key>"
export EMBEDDING_ENDPOINT="https://<embeddings-endpoint>/v1/embeddings"
export EMBEDDING_API_KEY="<embedding-api-key>"
export EMBEDDING_MODEL="text-embedding-3-small"

bash examples/release-validation/run-demo.sh
```

`EMBEDDING_ENDPOINT` is only needed when your regression queries use text `SEARCH ... SIMILAR TO ...` and the saved connection does not already point at an inference-enabled setup.

Install the binary the same way a user would:

```bash
INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/srimon12/qql-go/main/install.sh | sh
```

## Artifacts it writes

- `artifacts/00-connect.json` when `QDRANT_URL` is provided
- `artifacts/01-doctor.json`
- `artifacts/02-inspect.json`
- one JSON artifact per suite check

## CI Notes

- The active repo workflow now lives at `.github/workflows/retrieval-regression.yml`.
- `github-actions.yml` in this folder is the copyable example version of the same workflow.
- The GitHub Actions example installs `qql-go` via `install.sh` instead of compiling from source.
- Current `ubuntu-latest` hosted runners already include `jq 1.7`, so the example workflow does not install it separately.
- If you want to validate an unreleased branch of `qql-go` itself, that is a different workflow from the consumer-facing regression example here.

## Why this is the primary example

This is the operational workflow people actually recognize:

- connect to the real cluster
- run read-only retrieval checks
- fail fast on ranking regressions
- avoid reseeding and re-embedding huge datasets in CI

If you want disposable smoke coverage instead of a real regression suite, write a tiny checked-in collection setup and a few `qql-go exec --quiet --json` assertions around it. Keep that separate from this workflow.

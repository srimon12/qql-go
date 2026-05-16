# Demo: Retrieval Release Validation

This is the strongest `qql-go` demo for positioning.

## Problem

A team changes retrieval infrastructure:

- embedding model
- sparse or hybrid settings
- payload indexes
- collection quantization
- seeded support content

They need a repeatable validation workflow that runs from a single binary.

## What this demo proves

- provision a retrieval collection from versioned QQL
- seed a realistic support corpus
- inspect collection topology
- compare hybrid, exact, sparse, and grouped retrieval
- capture explain-plan and query results as JSON artifacts
- dump the final collection to a portable backup script

## Run it

### Windows (PowerShell)
```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File examples/release-validation/run-demo.ps1
```

### Linux / macOS (Bash)
```bash
./examples/release-validation/run-demo.sh
```

## Artifacts it writes

- `artifacts/01-doctor.json`
- `artifacts/02-provision.json`
- `artifacts/03-seed.json`
- `artifacts/04-show-collection.json`
- `artifacts/05-explain-hybrid.json`
- `artifacts/06-validate.json`
- `artifacts/07-search-refund-hybrid.json`
- `artifacts/08-search-refund-exact.json`
- `artifacts/09-search-billing-sparse.json`
- `artifacts/10-search-security-grouped.json`
- `artifacts/11-dump.json`
- `artifacts/release-validation-backup.qql`

## Demo flow

1. Provision `release_validation_docs`
2. Insert a support knowledge base
3. Explain a hybrid retrieval query before execution
4. Replay release validation searches
5. Save JSON outputs for CI, screenshots, and docs
6. Export a backup script for reproducibility

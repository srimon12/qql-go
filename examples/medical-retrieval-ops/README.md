# Demo: Medical Retrieval Operations

This demo is the vertical showcase.

## Problem

A team needs to prove that `qql-go` is useful for a real retrieval corpus, not just toy support snippets.

They want one reproducible workflow that:

- provisions a collection with tenant-aware indexing
- seeds clinical notes
- runs hybrid, sparse, filtered, grouped, and recommend queries
- captures machine-readable outputs for review

## What this demo proves

- hybrid collection creation with quantization
- tenant-aware payload indexing
- text indexing for diagnosis fields
- grouped retrieval by specialty
- filtered retrieval for active high-priority cases
- recommend-by-example from a known patient case
- portable dump generation after validation

## Run it

### Windows (PowerShell)
```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File examples/medical-retrieval-ops/run-demo.ps1
```

### Linux / macOS (Bash)
```bash
./examples/medical-retrieval-ops/run-demo.sh
```

## Artifacts it writes

- `artifacts/01-doctor.json`
- `artifacts/02-provision.json`
- `artifacts/03-seed.json`
- `artifacts/04-show-collection.json`
- `artifacts/05-search-hybrid-stroke.json`
- `artifacts/06-search-sparse-stroke.json`
- `artifacts/07-search-filtered-high-priority.json`
- `artifacts/08-search-grouped-specialty.json`
- `artifacts/09-recommend-from-stroke.json`
- `artifacts/10-dump.json`
- `artifacts/medical-retrieval-backup.qql`

## Demo flow

1. Create `medical_retrieval_ops`
2. Seed a clinical retrieval corpus
3. Run retrieval across multiple modes
4. Group results by specialty
5. Recommend similar records from a known stroke case
6. Export the collection as a portable script

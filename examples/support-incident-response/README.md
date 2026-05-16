# Demo: Support Incident Response

This demo shows `qql-go` as a support and incident-response tool, not just a query syntax layer.

## Problem

A customer says:

> Search stopped finding the right incident or runbook.

The operator needs a terminal-native workflow to inspect the collection, page through suspicious data, compare retrieval modes, and update the incident record without opening a notebook.

## What this demo proves

- seed a live-like incident collection
- inspect collection health and payload schema
- fetch an exact incident by ID
- page through filtered incidents
- compare dense, hybrid, exact, and sparse retrieval
- recommend similar incidents from a known example
- update incident ownership and status
- save all key steps as JSON artifacts

## Run it

### Windows (PowerShell)
```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File examples/support-incident-response/run-demo.ps1
```

### Linux / macOS (Bash)
```bash
./examples/support-incident-response/run-demo.sh
```

## Artifacts it writes

- `artifacts/01-doctor.json`
- `artifacts/02-seed.json`
- `artifacts/03-show-collection.json`
- `artifacts/04-select-incident.json`
- `artifacts/05-scroll-high-priority.json`
- `artifacts/06-explain-filtered-hybrid.json`
- `artifacts/07-search-hybrid.json`
- `artifacts/08-search-exact.json`
- `artifacts/09-search-sparse.json`
- `artifacts/10-recommend-similar.json`
- `artifacts/11-update-owner.json`
- `artifacts/12-select-updated-incident.json`

## Demo flow

1. Create `support_incident_response`
2. Seed realistic support incidents
3. Inspect the collection and one known incident
4. Walk the high-priority slice
5. Compare retrieval behaviors for the same problem statement
6. Recommend similar incidents from a known ID
7. Update ownership to show operational follow-through

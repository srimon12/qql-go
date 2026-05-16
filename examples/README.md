# Examples

`qql-go` needs to be shown through workflows people already recognize.

The canonical demos in this folder are now the **demo folders**, not the loose `.qql` files.

Use the folders when you want an end-to-end showcase with:

- ordered QQL steps
- a runnable PowerShell driver
- JSON artifacts you can screenshot, diff, or feed into docs
- a clear operational narrative instead of a raw syntax sample

## Before You Run Them

Connect `qql-go` to a Qdrant instance first:

```bash
qql-go connect --url https://<cluster>.qdrant.io --secret <api-key>
```

Or use local/self-hosted Qdrant for management-only flows:

```bash
qql-go connect --url http://localhost:6334
```

Some examples use text `INSERT` and `SEARCH ... SIMILAR TO ...`, so they need either:

- Qdrant Cloud inference
- local/external mode with an OpenAI-compatible embeddings endpoint

## Canonical Demo Folders

| Folder | Workflow it demonstrates | Run it with (PS / Bash) |
|---|---|---|
| `release-validation/` | Release validation for retrieval changes: provision, seed, explain, validate, dump | `examples/release-validation/run-demo.[ps1|sh]` |
| `support-incident-response/` | Support and incident workflow: inspect, scroll, compare retrieval modes, recommend, update ownership | `examples/support-incident-response/run-demo.[ps1|sh]` |
| `medical-retrieval-ops/` | Vertical demo: tenant-aware clinical retrieval with grouped and filtered search plus dump | `examples/medical-retrieval-ops/run-demo.[ps1|sh]` |

These are the examples to showcase publicly.

## Single-File Building Blocks

| File | What it demonstrates | Run it with |
|---|---|---|
| `01-bootstrap-docs.qql` | Create a hybrid collection with payload-aware HNSW, add tenant-aware and text payload indexes, bulk-insert seed documents, inspect schema | `qql-go execute examples/01-bootstrap-docs.qql` |
| `02-ci-smoke-test.qql` | Disposable end-to-end smoke test for create, insert, search, recommend, delete, update, and drop | `qql-go execute --quiet --json examples/02-ci-smoke-test.qql` |
| `03-retrieval-debugging.qql` | Compare dense, dense+MMR, hybrid, sparse, exact, filtered, and ACORN-assisted retrieval flows | `qql-go execute examples/03-retrieval-debugging.qql` |
| `04-support-diagnostics.qql` | Show collection health, exact point lookup, filtered scroll, and incident recommendations | `qql-go execute examples/04-support-diagnostics.qql` |
| `05-retention-cleanup.qql` | Review archived data, delete by filter, and verify retention cleanup | `qql-go execute examples/05-retention-cleanup.qql` |
| `06-medical-records.qql` | Real-world healthcare search demo with hybrid retrieval and metadata filters | `qql-go execute examples/06-medical-records.qql` |

Use the single-file scripts when you want a smaller isolated query pattern.

## Why The Folder Demos Matter

- they feel like real ops workflows, not parser tests
- they generate reusable JSON artifacts
- they make the standalone binary value obvious
- they are easier to turn into docs, screenshots, or short demo videos

## Automation Patterns

Use these examples as building blocks for scripts, CI jobs, and agents:

```bash
qql-go doctor --quiet --json
qql-go exec --quiet --json "SHOW COLLECTION docs"
qql-go explain --quiet --json "SEARCH docs SIMILAR TO 'vector database latency' LIMIT 5 USING HYBRID"
qql-go execute --quiet --json examples/02-ci-smoke-test.qql
qql-go dump --quiet --json docs docs-backup.qql
```

## Which Demo Matches Which Job

- Release validation around retrieval changes: `release-validation/`
- Support and on-call investigation: `support-incident-response/`
- A domain-specific, non-toy retrieval showcase: `medical-retrieval-ops/`
- Smaller isolated syntax patterns: the loose `.qql` files

## Boundaries

These are operational examples, not application architecture templates. Use the Qdrant SDK in app code; use `qql-go` when you want deterministic, reviewable database operations.

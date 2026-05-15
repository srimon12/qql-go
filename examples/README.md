# Examples

These examples present `qql-go` as an operational interface for Qdrant: bootstrap collections, run CI smoke checks, debug retrieval, inspect live data, and clean up collections with repeatable `.qql` scripts.

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

## Example Scripts

| File | What it demonstrates | Run it with |
|---|---|---|
| `01-bootstrap-docs.qql` | Create a hybrid collection, add payload indexes, bulk-insert seed documents, inspect schema | `qql-go execute examples/01-bootstrap-docs.qql` |
| `02-ci-smoke-test.qql` | Disposable end-to-end smoke test for create, insert, search, recommend, delete, update, and drop | `qql-go execute --quiet --json examples/02-ci-smoke-test.qql` |
| `03-retrieval-debugging.qql` | Compare dense, hybrid, sparse, exact, filtered, and ACORN-assisted retrieval flows | `qql-go execute examples/03-retrieval-debugging.qql` |
| `04-support-diagnostics.qql` | Show collection health, exact point lookup, filtered scroll, and incident recommendations | `qql-go execute examples/04-support-diagnostics.qql` |
| `05-retention-cleanup.qql` | Review archived data, delete by filter, and verify retention cleanup | `qql-go execute examples/05-retention-cleanup.qql` |
| `06-medical-records.qql` | Real-world healthcare search demo with hybrid retrieval and metadata filters | `qql-go execute examples/06-medical-records.qql` |

## Automation Patterns

Use these examples as building blocks for scripts, CI jobs, and agents:

```bash
qql-go doctor --quiet --json
qql-go exec --quiet --json "SHOW COLLECTION docs"
qql-go explain --quiet --json "SEARCH docs SIMILAR TO 'vector database latency' LIMIT 5 USING HYBRID"
qql-go execute --quiet --json examples/02-ci-smoke-test.qql
qql-go dump --quiet --json docs docs-backup.qql
```

## Which Example Matches Which Job

- Bootstrap a collection and seed example data: `01-bootstrap-docs.qql`
- Prove a cluster is healthy in CI: `02-ci-smoke-test.qql`
- Debug why retrieval quality changed: `03-retrieval-debugging.qql`
- Hand a support engineer a reproducible inspection flow: `04-support-diagnostics.qql`
- Automate retention cleanup and post-delete checks: `05-retention-cleanup.qql`
- Demo a real vertical use case instead of synthetic lorem ipsum: `06-medical-records.qql`

## Boundaries

These are operational examples, not application architecture templates. Use the Qdrant SDK in app code; use `qql-go` when you want deterministic, reviewable database operations.

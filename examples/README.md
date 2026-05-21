# Examples

These examples are runnable retrieval workflows.

Each one ships as a folder with:

- a Bash runner
- a PowerShell runner
- checked-in QQL used by the workflow
- JSON artifacts you can inspect, diff, or attach to CI

## Before You Run Them

Install `qql-go`, then connect it to Qdrant:

```bash
qql-go connect --url https://<cluster>.qdrant.io --secret <api-key>
```

If you use local or self-hosted Qdrant:

```bash
qql-go connect --url http://localhost:6333
```

Examples that use text `INSERT` or `SEARCH ... SIMILAR TO ...` also need embeddings through either:

- Qdrant Cloud inference
- `local` or `external` inference mode with an OpenAI-compatible embeddings endpoint

## Start Here

### `release-validation/`

Use this when you want retrieval regression checks against an existing collection.

It runs `SHOW`, `EXPLAIN`, and `SEARCH` checks against the dataset you already have, validates the JSON results, and fits naturally into CI without reseeding the corpus. The checked-in suite also exercises the current search syntax for offset windows, score thresholds, and lookup-from plans.

Run:

```bash
bash examples/release-validation/run-demo.sh
```

### `retrieval-debug-runbook/`

Use this when someone says search stopped working and you need to investigate quickly.

It provisions a small runbook corpus, compares hybrid, exact, and sparse retrieval, reruns the issue with filters, inspects the expected document, and saves the artifacts you would want in a support or on-call report.

Run:

```bash
bash examples/retrieval-debug-runbook/run-demo.sh
```

### `medical-retrieval-ops/`

Use this when you want a full end-to-end benchmark and showcase.

It downloads the full `ChatMED-Project/RAGCare-QA` dataset from Hugging Face, builds a QQL corpus, loads it into Qdrant, compares dense, sparse, hybrid RRF, hybrid DBSF, and exact retrieval, then writes benchmark results for `hit@1` and `hit@5`.

Run:

```bash
bash examples/medical-retrieval-ops/run-demo.sh
```

## Which One To Show First

- `release-validation/` if you want the clearest CI and ops story
- `retrieval-debug-runbook/` if you want the fastest investigation story
- `medical-retrieval-ops/` if you want the broadest feature and benchmark story

## What The Artifacts Are For

Each workflow writes JSON artifacts under its own `artifacts/` directory.

Use them to:

- diff retrieval behavior between runs
- attach evidence to a CI failure
- inspect explain plans and result IDs
- feed screenshots, docs, or agent reports

## Boundaries

These examples show how to operate and validate retrieval systems around an app.

Use the Qdrant SDK in application code. Use `qql-go` when you want deterministic, reviewable retrieval operations.

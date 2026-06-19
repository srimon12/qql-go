# Examples

Runnable retrieval workflows showcasing `qql-go` capabilities.

## Before You Run

Install `qql-go`, then connect:

```bash
# Cloud
qql-go connect --url https://<cluster>.qdrant.io --secret <api-key>

# Local (needs embedding endpoint for text operations)
qql-go connect --url http://localhost:6334 --inference-mode local \
  --embedding-endpoint http://127.0.0.1:1234/v1/embeddings \
  --embedding-model text-embedding-all-minilm-l6-v2-embedding
```

## Examples

### `pdf-retrieval/` — Multivector PDF retrieval with ColBERT

Two-stage retrieval with ColPali/ColQwen-style multivectors: mean-pooled vectors for fast first-stage, original multivectors for accurate reranking.

**Best for:** PDF retrieval at scale, ColBERT/ColPali workloads.

```bash
bash examples/pdf-retrieval/run-demo.sh
```

### `medical-showcase/` — Full QQL feature showcase

Minimal Python script that demonstrates every QQL feature against 12 medical records: hybrid search, filters, grouped retrieval, recommend, context, discover, prefetch DAGs, parameterized RRF, mutations, and operations.

**Best for:** seeing everything QQL can do in one run.

```bash
uv run examples/medical-showcase/main.py --execute
```

### `release-validation/` — CI retrieval regression checks

Runs `SHOW`, `EXPLAIN`, and `QUERY` checks against an existing collection. Validates JSON results. Fits into CI without reseeding.

**Best for:** CI pipelines, smoke tests, release gates.

```bash
bash examples/release-validation/run-demo.sh
```

### `retrieval-debug-runbook/` — Search incident investigation

Provisions a small corpus, compares hybrid/exact/sparse retrieval, inspects expected documents, and saves artifacts for support reports.

**Best for:** on-call debugging, support investigations.

```bash
bash examples/retrieval-debug-runbook/run-demo.sh
```

### `medical-retrieval-ops/` — Full benchmark with HuggingFace corpus

Downloads the `RAGCare-QA` dataset, builds a QQL corpus, compares retrieval modes, and writes `hit@1`/`hit@5` benchmark results.

**Best for:** retrieval quality evaluation, model comparison.

```bash
bash examples/medical-retrieval-ops/run-demo.sh
```

## Which To Show First

| Audience | Example |
|----------|---------|
| Developer evaluating QQL | `medical-showcase/` |
| PDF / ColBERT retrieval | `pdf-retrieval/` |
| CI/ops engineer | `release-validation/` |
| On-call / support | `retrieval-debug-runbook/` |
| ML / retrieval engineer | `medical-retrieval-ops/` |

## Artifacts

Each workflow writes JSON artifacts to its `artifacts/` directory.

Use them to: diff retrieval between runs, attach to CI failures, inspect explain plans, feed agent reports.

## Boundaries

Use `qql-go` for deterministic, reviewable retrieval operations. Use the Qdrant SDK for application code.

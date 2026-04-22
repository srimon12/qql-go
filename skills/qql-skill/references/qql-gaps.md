# QQL Gaps

Use this file when a request sounds reasonable in Qdrant terms but is still outside the current QQL surface.

## Not Supported Yet

- local/external rerank (`RERANK` is cloud-only)
- discovery API
- MMR or diversity controls
- score boosting
- relevance feedback
- multi-stage retrieval beyond the built-in hybrid and rerank paths
- select by id
- update or upsert by explicit id (you can `INSERT` with explicit `id` to overwrite)
- scroll or pagination
- collection diagnostics beyond `doctor`
- collection-level HNSW or quantization config
- on-disk vector or payload toggles
- `CREATE COLLECTION` with custom vector sizes or distance metrics

## What To Say

Prefer plain language:

- `QQL does not support this yet.`
- `This needs raw Qdrant SDK usage or a QQL extension.`
- `The closest supported QQL form is ...`

## Practical Fallbacks

- Need exact baseline: use `EXACT`
- Need recall tuning: use `WITH { hnsw_ef: ... }`
- Need keyword plus semantic retrieval: use `USING HYBRID`
- Need better ordering: use `RERANK` (cloud only)
- Need filtering: create an index first, then use `WHERE`
- Need a runnable prototype: stay inside `CREATE`, `CREATE INDEX`, `INSERT`, `SEARCH`, `DELETE`, `RECOMMEND`
- Need batch insert: use `INSERT BULK`
- Need script round-trip: use `qql-go execute` and `qql-go dump`
- Need local inference without cloud: use `qql-go connect --inference-mode local`

## Reminder

Do not hide missing features behind made-up syntax. If the current CLI cannot parse and execute it, it is outside this skill.

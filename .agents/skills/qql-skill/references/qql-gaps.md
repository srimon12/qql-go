# QQL Gaps

Use this file when a request sounds reasonable in Qdrant terms but is still outside the current QQL surface.

## Not Supported Yet

- self-hosted/local text inference pipeline for `INSERT` and `SEARCH ... SIMILAR TO ...`
- self-hosted/local dense+sparse generation for `USING HYBRID` without cloud inference
- recommend API
- discovery API
- MMR or diversity controls
- score boosting
- relevance feedback
- multi-stage retrieval beyond the built-in hybrid and rerank paths
- select by id
- update or upsert by explicit id
- batch insert
- scroll or pagination
- collection diagnostics
- collection-level HNSW or quantization config
- on-disk vector or payload toggles

## Supported Recently

Do not call these gaps anymore:

- `CREATE INDEX ON COLLECTION ... FOR ... TYPE ...`
- `DELETE FROM ... WHERE <field> = '<value>'`
- `EXPLAIN <statement>`
- `RERANK` on the current Qdrant Cloud path

## What To Say

Prefer plain language:

- `QQL does not support this yet.`
- `This needs raw Qdrant SDK usage or a QQL extension.`
- `The closest supported QQL form is ...`

## Practical Fallbacks

- Need exact baseline: use `EXACT`
- Need recall tuning: use `WITH { hnsw_ef: ... }`
- Need keyword plus semantic retrieval: use `USING HYBRID`
- Need better ordering: use `RERANK`
- Need filtering: create an index first, then use `WHERE`
- Need a runnable prototype: stay inside `CREATE`, `CREATE INDEX`, `INSERT`, `SEARCH`, `DELETE`

## Reminder

Do not hide missing features behind made-up syntax. If the current CLI cannot parse and execute it, it is outside this skill.

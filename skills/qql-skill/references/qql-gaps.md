# QQL Gaps

Use this file when a request sounds reasonable in Qdrant terms but is still outside the current QQL surface.

## Not Supported Yet

- local/external rerank (`RERANK` is cloud-only)
- formula / score boosting (payload-aware score shaping, geo decay, conditional scoring)
- relevance feedback query
- ORDER BY (non-similarity sorting by payload field)
- SAMPLE RANDOM (random point sampling)
- WithPayload / WithVectors selectors (control what's returned in results)
- per-prefetch filter/score threshold via manual prefetch DAGs (partially supported — `PREFETCH` block supports per-prefetch `WHERE` and `SCORE THRESHOLD`)
- offset-style pagination for grouped search
- MMR for `USING SPARSE` or `RECOMMEND`
- custom vector on-disk toggles
- ReadConsistency / ShardKeySelector / Timeout controls
- Go programmatic API (`qql.Parse()` + `qql.Execute()`)
- batch query / mutation surfaces

## What To Say

Prefer plain language:

- `QQL does not support this yet.`
- `This needs raw Qdrant SDK usage or a QQL extension.`
- `The closest supported QQL form is ...`

## Practical Fallbacks

- Need exact baseline: use `EXACT`
- Need a single point by exact ID: use `SELECT * FROM <collection> WHERE id = ...`
- Need to browse or export points page by page: use `SCROLL FROM <collection> ... LIMIT <n>`
- Need recall tuning: use `WITH (hnsw_ef = ...)`
- Need flat search pagination: use `QUERY ... LIMIT <n> OFFSET <n>`
- Need low-score filtering: use `QUERY ... SCORE THRESHOLD <float|int>`
- Need cross-collection lookup: use `QUERY ... LOOKUP FROM <collection> [VECTOR '<name>']`
- Need keyword plus semantic retrieval: use `USING HYBRID`
- Need parameterized RRF tuning: use `WITH (rrf_k = <n>, rrf_weights = [...])`
- Need multi-stage retrieval with per-prefetch filters: use `WITH <name> AS (...) ... PREFETCH (name) FUSION RRF`
- Need hybrid DBSF fusion: use `USING HYBRID FUSION DBSF`
- Need better ordering: use `RERANK` (cloud only)
- Need filtering: create an index first, then use `WHERE`
- Need grouped top results by field: use `QUERY ... GROUP BY <field> [GROUP_SIZE <n>]`
- Need to patch metadata in place: use `UPDATE <collection> SET PAYLOAD = {...} WHERE ...`
- Need to replace a stored vector: use `UPDATE <collection> SET VECTOR = [...] WHERE id = ...`
- Need a runnable prototype: stay inside `CREATE`, `CREATE INDEX`, `INSERT`, `QUERY`, `DELETE`
- Need batch insert: use comma-separated `INSERT INTO <name> VALUES {...}, {...}`
- Need script round-trip: use `qql-go execute` and `qql-go dump [--batch-size N]`
- Need local inference without cloud: use `qql-go connect --inference-mode local`
- Need score boosting or formula: use raw Qdrant SDK

## Reminder

Do not hide missing features behind made-up syntax. If the current CLI cannot parse and execute it, it is outside this skill.

# Recommend API Upgrade Spec

## Target

Upgrade RECOMMEND in both `D:\Sativa\qql` (Python) and `D:\Sativa\qql-go` (Go) to cover the full Qdrant Recommend API surface.

## Current State (Both Projects)

Both support:
- `POSITIVE IDS (<id>, ...)`
- `[NEGATIVE IDS (<id>, ...)]`
- `[STRATEGY '<strategy>']` — average_vector, best_score, sum_scores
- `LIMIT <n>`
- `[WHERE <filter>]`
- Seed ID exclusion via HasIdCondition in must_not

## Gaps to Close (Both Projects)

### 1. OFFSET
Qdrant `QueryPoints` supports `Offset` for pagination.

Syntax: `[OFFSET <n>]` after LIMIT

Example:
```sql
RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 OFFSET 5
```

### 2. SCORE THRESHOLD
Qdrant `QueryPoints` supports `ScoreThreshold` to filter low-scoring results.

Syntax: `[SCORE THRESHOLD <f>]` (float)

Example:
```sql
RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 SCORE THRESHOLD 0.5
```

### 3. WITH clause (search params parity)
SEARCH supports `WITH { exact: true, hnsw_ef: 128 }`. RECOMMEND should too.

Syntax: `[WITH { exact: true, hnsw_ef: <n> }]`

Example:
```sql
RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 WITH { exact: true }
```

### 4. LOOKUP FROM (cross-collection recommend)
Qdrant `QueryPoints` supports `LookupFrom` (LookupLocation) for looking up positive/negative IDs from a different collection.

Syntax: `[LOOKUP FROM <collection> [VECTOR '<name>']]`

Example:
```sql
RECOMMEND FROM target_collection POSITIVE IDS ('a') LIMIT 5 LOOKUP FROM source_collection VECTOR 'dense'
```

If `VECTOR` is omitted, uses the default vector in the source collection.

### 5. USING <vector_name> (target collection vector)
Qdrant `QueryPoints` supports `Using` to specify which named vector in the TARGET collection to search.

Currently hardcoded to "dense" in Go. Should be overridable.

Syntax: `[USING '<vector_name>']`

Example:
```sql
RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 5 USING 'sparse'
```

### 6. LOOKUP FROM + USING together
Cross-collection recommend with specific vectors on both ends:

```sql
RECOMMEND FROM target_collection
POSITIVE IDS ('a')
LIMIT 5
LOOKUP FROM source_collection VECTOR 'dense'
USING 'sparse'
```

## What NOT to add

- Raw vector positive/negative inputs (too niche, complex syntax)
- with_payload / with_vectors (response shaping is a CLI concern, not QQL query semantics)
- ShardKeySelector (too advanced for QQL scope)

## Order of tokens in parser

```
RECOMMEND FROM <collection>
POSITIVE IDS (<id>, ...)
[NEGATIVE IDS (<id>, ...)]
[STRATEGY '<strategy>']
[LOOKUP FROM <collection> [VECTOR '<name>']]
[USING '<vector_name>']
LIMIT <n>
[OFFSET <n>]
[SCORE THRESHOLD <f>]
[WHERE <filter>]
[WITH { exact: true, hnsw_ef: <n> }]
```

## Files to modify

### Python (D:\Sativa\qql)

1. `src/qql/ast_nodes.py` — Add fields to `RecommendStmt`
2. `src/qql/lexer.py` — Add token kinds: OFFSET, SCORE, THRESHOLD, LOOKUP
3. `src/qql/parser.py` — Parse new clauses
4. `src/qql/executor.py` — Wire to Qdrant client
5. `tests/test_parser.py` — Parser tests
6. `tests/test_executor.py` — Executor tests
7. Update skill docs if applicable

### Go (D:\Sativa\qql-go)

1. `internal/ast/nodes.go` — Add fields to `RecommendStmt`
2. `internal/lexer/tokenkind.go` — Add token kinds
3. `internal/lexer/lexer.go` — Tokenize new keywords
4. `internal/parser/parser.go` — Parse new clauses
5. `internal/cli/commands/commands.go` — Wire to Qdrant client
6. `internal/parser/parser_test.go` — Parser tests
7. `internal/cli/commands/commands_test.go` — Executor tests
8. Update skill docs

## Important Notes

- Both implementations must stay in sync (same QQL syntax)
- `LOOKUP FROM` is only meaningful if positive/negative IDs exist in a different collection
- `USING` on recommend is about the target collection's vector, not the lookup collection's vector
- The `WITH` clause parser already exists for SEARCH; reuse it for RECOMMEND
- Tests must cover: offset, score_threshold, with clause, lookup_from, using
- DO NOT commit changes. Just edit files and ensure tests pass.

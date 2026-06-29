# QQL Filter Reference

Filter expressions in `WHERE` clauses for QUERY, SCROLL, UPDATE, and DELETE statements.

## Comparison operators

```sql
WHERE field = 'value'          -- equality
WHERE field != 'value'         -- inequality
WHERE field > 10               -- greater than
WHERE field >= 10              -- greater than or equal
WHERE field < 100              -- less than
WHERE field <= 100             -- less than or equal
WHERE field = 3.14             -- float equality
WHERE field = true             -- boolean equality
```

## Range

```sql
WHERE field BETWEEN 10 AND 100
WHERE year BETWEEN 2020 AND 2026
WHERE score BETWEEN 0.5 AND 1.0
```

## Set membership

```sql
WHERE status IN ('active', 'pending', 'reviewed')
WHERE status NOT IN ('deleted', 'archived')
WHERE priority IN ('high', 'medium')
WHERE year IN (2024, 2025, 2026)
```

## Null and empty checks

```sql
WHERE field IS NULL
WHERE field IS NOT NULL
WHERE field IS EMPTY
WHERE field IS NOT EMPTY
```

## Text matching

```sql
WHERE content MATCH 'hello world'           -- full-text match
WHERE content MATCH ANY 'hello world'       -- match any term
WHERE content MATCH PHRASE 'hello world'    -- exact phrase match
```

## Logical operators

```sql
WHERE a = 1 AND b = 2
WHERE a = 1 OR b = 2
WHERE NOT a = 1
WHERE (a = 1 OR b = 2) AND c = 3
WHERE (team = 'search' OR team = 'infra') AND severity >= 3
```

## Nested object filters

Filters on nested array fields using `NESTED('path', filter)`:

```sql
-- Filter by nested array field
WHERE NESTED('reviews', rating > 4)

-- Nested filter with AND
WHERE NESTED('overwritten_in', by = 'root' AND seq <= 2)

-- Negated nested filter
WHERE NOT NESTED('overwritten_in', by = 'root')

-- Combined with top-level filters
WHERE branch = 'root' AND NOT NESTED('overwritten_in', by = 'root' AND seq <= 2)

-- Nested filter in SCROLL
SCROLL FROM content WHERE NESTED('tags', name = 'important') LIMIT 20
```

`NESTED` scopes a filter to a nested/array object path. The first argument is the field path (string), the second is any valid filter expression. Maps to Qdrant's `NestedCondition`.

## Operator precedence

1. Comparison operators, `BETWEEN`, `IN`, `IS`, `MATCH`
2. `NOT`
3. `AND`
4. `OR`

Parentheses override precedence.

## Examples

```sql
-- Filter by status and year
QUERY 'search' FROM docs LIMIT 10 WHERE status = 'published' AND year >= 2024

-- Filter with set membership
QUERY 'retrieval' FROM docs LIMIT 10 WHERE category IN ('ml', 'nlp', 'cv')

-- Filter with range
QUERY 'articles' FROM docs LIMIT 10 WHERE score BETWEEN 0.8 AND 1.0

-- Filter with text matching
QUERY 'search' FROM docs LIMIT 10 WHERE title MATCH PHRASE 'vector database'

-- Complex filter
QUERY 'emergency' FROM docs LIMIT 10
  WHERE (specialty = 'neurology' OR specialty = 'cardiology')
    AND priority = 'high'
    AND status != 'discharged'

-- Null check
QUERY 'records' FROM docs LIMIT 10 WHERE diagnosis IS NOT NULL

-- Filter in SCROLL
SCROLL FROM docs WHERE topic = 'search' AND year >= 2024 LIMIT 20

-- Filter in DELETE
DELETE FROM docs WHERE status = 'archived'

-- Filter in UPDATE
UPDATE docs SET PAYLOAD = {'status': 'reviewed'} WHERE status = 'pending'

-- Nested filter (branch-aware search)
QUERY 'pricing' FROM content LIMIT 5
  WHERE branch = 'root'
    AND NOT NESTED('overwritten_in', by = 'root' AND seq <= 2)
```

## Per-prefetch filters

Filters can be applied to individual CTE prefetches:

```sql
WITH
  dense AS (QUERY 'search' USING dense LIMIT 200),
  sparse AS (QUERY 'search' USING sparse LIMIT 300)
QUERY 'search' FROM docs LIMIT 10
  PREFETCH (
    dense WHERE category = 'tech' SCORE THRESHOLD 0.6,
    sparse WHERE priority = 'high' SCORE THRESHOLD 0.3
  )
  FUSION RRF
```

## Payload indexes

For efficient filtering, create payload indexes:

```sql
CREATE INDEX ON docs FOR status TYPE keyword
CREATE INDEX ON docs FOR year TYPE integer
CREATE INDEX ON docs FOR score TYPE float
CREATE INDEX ON docs FOR tags TYPE keyword WITH (is_tenant = true)
CREATE INDEX ON docs FOR content TYPE text WITH (tokenizer = 'word', lowercase = true)
```

Without an index, Qdrant performs a full scan. With heavy filtering, always create indexes first.

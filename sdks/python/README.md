# QQL Python SDK

Python client for the QQL Gateway. String in, dict out.

## Install

```bash
pip install qql
# or with uv
uv add qql
```

## Quick Start

```python
from qql import QQLClient

client = QQLClient("http://localhost:50051")

# Simple search
res = client.exec("QUERY 'emergency triage' FROM docs LIMIT 5 USING HYBRID")
print(res.data)  # parsed JSON dict

# Explain without executing
plan = client.explain("QUERY 'test' FROM docs LIMIT 5")
print(plan)
```

## API

### `exec(query: str) -> Result`

Execute a single QQL query.

```python
res = client.exec("QUERY 'stroke' FROM medical LIMIT 5")
# res.ok, res.operation, res.message, res.data
```

### `exec_batch(queries: list[str], stop_on_error=False) -> list[Result]`

Execute multiple queries. Supports mixed statement types (INSERT, CREATE, QUERY, etc.). For pure QUERY batches, the gateway automatically uses Qdrant's native `QueryBatch` API for a single round-trip.

```python
results = client.exec_batch([
    "INSERT INTO docs VALUES {'text': 'hello'}",
    "QUERY 'hello' FROM docs LIMIT 5",
], stop_on_error=True)
```

### `explain(query: str) -> str`

Return the execution plan without running.

```python
plan = client.explain("QUERY 'test' FROM docs LIMIT 5 BOOST (score * 2.0)")
```

### `health() -> HealthStatus`

Check gateway and Qdrant connection.

```python
status = client.health()
print(status.version, status.qdrant_connected)
```

### `raw`

Access the generated Connect RPC client for direct protobuf usage.

```python
from qql.gen.qql_pb2 import ExecRequest

resp = client.raw.exec(ExecRequest(query="QUERY 'test' FROM docs LIMIT 5"))
```

## Context Manager

```python
with QQLClient("http://localhost:50051") as client:
    res = client.exec("SHOW COLLECTIONS")
```

## Production Examples

### Hybrid Search with Score Boosting

```python
res = client.exec("""
WITH
  dense AS (QUERY 'kubernetes deployment' USING dense LIMIT 200 WHERE priority = 'high'),
  sparse AS (QUERY 'kubernetes deployment' USING sparse LIMIT 300)
QUERY 'kubernetes deployment' FROM incidents LIMIT 10
  PREFETCH (dense SCORE THRESHOLD 0.6, sparse SCORE THRESHOLD 0.3)
  FUSION RRF
  WITH (rrf_k = 20, rrf_weights = [0.6, 0.4])
  BOOST (CASE WHEN priority = 'critical' THEN score * 2.0 ELSE score END)
""")
```

### Batch Ingest + Search

```python
results = client.exec_batch([
    "CREATE COLLECTION docs HYBRID WITH HNSW (m = 32) WITH QUANTIZATION (type = 'turbo', bits = 2)",
    "CREATE INDEX ON COLLECTION docs FOR category TYPE keyword",
    "INSERT INTO docs VALUES {'text': 'first doc', 'category': 'tech'} USING HYBRID",
    "QUERY 'first doc' FROM docs LIMIT 5",
], stop_on_error=True)

for r in results:
    print(r.ok, r.message)
```

### Random Sampling for Dashboards

```python
res = client.exec("QUERY SAMPLE FROM docs LIMIT 20 WHERE status = 'active'")
for hit in res.data:
    print(hit["id"], hit["score"])
```

### Paginated Browse by Field

```python
page1 = client.exec("QUERY ORDER BY created_at DESC FROM docs LIMIT 20")
page2 = client.exec("QUERY ORDER BY created_at DESC FROM docs LIMIT 20 OFFSET 20")
```

### Geo-Distance Decay

```python
res = client.exec("""
QUERY 'restaurant' FROM places LIMIT 10
  BOOST (score * GAUSS_DECAY(GEO_DISTANCE(48.8566, 2.3522, location), 0, 5000, 0.5))
""")
```

### Conditional Scoring

```python
res = client.exec("""
QUERY 'patient treatment' FROM medical LIMIT 10
  BOOST (CASE WHEN priority = 'high' THEN score * 2.0
              WHEN status = 'critical' THEN score * 1.5
              ELSE score END)
""")
```

## Error Handling

Failed queries return a `Result` with `ok=False`:

```python
res = client.exec("QUERY 'test' FROM nonexistent LIMIT 5")
if not res.ok:
    print(res.message)  # error description
```

## Types

```python
from qql import Result, HealthStatus

# Result
res.ok: bool
res.operation: str
res.message: str
res.data: Any  # parsed JSON or None

# HealthStatus
status.version: str
status.qdrant_connected: bool
status.qdrant_status: str
```

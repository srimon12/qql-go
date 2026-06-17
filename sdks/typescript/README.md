# QQL TypeScript SDK

TypeScript client for the QQL Gateway. String in, object out.

## Install

```bash
npm install qql-client
# or
pnpm add qql-client
```

## Quick Start

```typescript
import { QQLClient } from "qql-client";

const client = new QQLClient("http://localhost:50051");

// Simple search
const res = await client.exec("QUERY 'emergency triage' FROM docs LIMIT 5 USING HYBRID");
console.log(res.data); // parsed JSON object

// Explain without executing
const plan = await client.explain("QUERY 'test' FROM docs LIMIT 5");
console.log(plan);
```

## API

### `exec(query: string): Promise<Result>`

Execute a single QQL query.

```typescript
const res = await client.exec("QUERY 'stroke' FROM medical LIMIT 5");
// res.ok, res.operation, res.message, res.data
```

### `execBatch(queries: string[], stopOnError?: boolean): Promise<Result[]>`

Execute multiple queries in one round-trip.

```typescript
const results = await client.execBatch([
  "INSERT INTO docs VALUES {'text': 'hello'}",
  "QUERY 'hello' FROM docs LIMIT 5",
], true);
```

### `explain(query: string): Promise<string>`

Return the execution plan without running.

```typescript
const plan = await client.explain("QUERY 'test' FROM docs LIMIT 5 BOOST (score * 2.0)");
```

### `health(): Promise<HealthStatus>`

Check gateway and Qdrant connection.

```typescript
const status = await client.health();
console.log(status.version, status.qdrantConnected);
```

### `raw`

Access the generated Connect RPC client for direct protobuf usage.

```typescript
import { ExecRequestSchema } from "qql-client/gen/qql_pb";

const resp = await client.raw.exec({ query: "QUERY 'test' FROM docs LIMIT 5" });
```

## Production Examples

### Hybrid Search with Score Boosting

```typescript
const res = await client.exec(`
WITH
  dense AS (QUERY 'kubernetes deployment' USING dense LIMIT 200 WHERE priority = 'high'),
  sparse AS (QUERY 'kubernetes deployment' USING sparse LIMIT 300)
QUERY 'kubernetes deployment' FROM incidents LIMIT 10
  PREFETCH (dense SCORE THRESHOLD 0.6, sparse SCORE THRESHOLD 0.3)
  FUSION RRF
  WITH (rrf_k = 20, rrf_weights = [0.6, 0.4])
  BOOST (CASE WHEN priority = 'critical' THEN score * 2.0 ELSE score END)
`);
```

### Batch Ingest + Search

```typescript
const results = await client.execBatch([
  "CREATE COLLECTION docs HYBRID WITH HNSW (m = 32) WITH QUANTIZATION (type = 'turbo', bits = 2)",
  "CREATE INDEX ON COLLECTION docs FOR category TYPE keyword",
  "INSERT INTO docs VALUES {'text': 'first doc', 'category': 'tech'} USING HYBRID",
  "QUERY 'first doc' FROM docs LIMIT 5",
], true);

for (const r of results) {
  console.log(r.ok, r.message);
}
```

### Random Sampling for Dashboards

```typescript
const res = await client.exec("QUERY SAMPLE FROM docs LIMIT 20 WHERE status = 'active'");
for (const hit of res.data as any[]) {
  console.log(hit.id, hit.score);
}
```

### Paginated Browse by Field

```typescript
const page1 = await client.exec("QUERY ORDER BY created_at DESC FROM docs LIMIT 20");
const page2 = await client.exec("QUERY ORDER BY created_at DESC FROM docs LIMIT 20 OFFSET 20");
```

### Geo-Distance Decay

```typescript
const res = await client.exec(`
QUERY 'restaurant' FROM places LIMIT 10
  BOOST (score * GAUSS_DECAY(GEO_DISTANCE(48.8566, 2.3522, location), 0, 5000, 0.5))
`);
```

### Conditional Scoring

```typescript
const res = await client.exec(`
QUERY 'patient treatment' FROM medical LIMIT 10
  BOOST (CASE WHEN priority = 'high' THEN score * 2.0
              WHEN status = 'critical' THEN score * 1.5
              ELSE score END)
`);
```

## Error Handling

Failed queries return a `Result` with `ok: false`:

```typescript
const res = await client.exec("QUERY 'test' FROM nonexistent LIMIT 5");
if (!res.ok) {
  console.error(res.message);
}
```

## Types

```typescript
import type { Result, HealthStatus } from "qql-client";

// Result
res.ok: boolean;
res.operation: string;
res.message: string;
res.data: unknown | null;

// HealthStatus
status.version: string;
status.qdrantConnected: boolean;
status.qdrantStatus: string;
```

## Power Users

Import generated protobuf types directly:

```typescript
import { QQL, ExecRequestSchema, ExecResponseSchema } from "qql-client/gen/qql_pb";
```

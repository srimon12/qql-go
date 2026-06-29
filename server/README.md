# server — QQL Gateway

The `server` package implements the QQL Connect RPC gateway. It accepts QQL queries over HTTP, parses them, enforces policy, executes them against Qdrant, and returns structured responses.

## What it is

A single-binary gateway that sits between your application and Qdrant. It speaks Connect RPC (gRPC + gRPC-Web + HTTP/1.1 JSON), so any language can call it — curl, Python, TypeScript, Go, or an LLM agent.

```
┌──────────┐       Connect RPC        ┌───────────┐       gRPC        ┌──────────┐
│  Client  │  ──────────────────────▶  │  Gateway  │  ──────────────▶  │  Qdrant  │
│ (any)    │  HTTP POST / JSON / gRPC  │  (this)   │  go-client       │          │
└──────────┘                           └───────────┘                   └──────────┘
```

Without gateway flags, it's a transparent QQL proxy. With flags, it becomes a policy enforcement point with auth, tenant isolation, and audit.

## RPC Service

Defined in `proto/qql.proto`, implemented in `handler.go`:

| RPC | Purpose |
|-----|---------|
| `Exec` | Parse and execute a single QQL statement |
| `ExecBatch` | Execute multiple statements in one round-trip |
| `Explain` | Return the execution plan without running the query |
| `Health` | Gateway + Qdrant connection status |
| `Convert` | Translate Qdrant REST JSON into QQL statements |

### Exec

```bash
curl -X POST http://localhost:50051/qql.QQL/Exec \
  -H "Content-Type: application/json" \
  -d '{"query": "QUERY \"search\" FROM docs LIMIT 5 USING HYBRID"}'
```

Returns `ExecResponse` with `ok`, `operation`, `message`, and `data` (JSON bytes).

### ExecBatch

```bash
curl -X POST http://localhost:50051/qql.QQL/ExecBatch \
  -H "Content-Type: application/json" \
  -d '{"queries": [{"query": "QUERY \"a\" FROM docs LIMIT 5"}, {"query": "QUERY \"b\" FROM docs LIMIT 5"}], "stop_on_error": true}'
```

When all queries are pure `QUERY` statements, the gateway auto-detects and routes them through Qdrant's native `QueryBatch` API for a single round-trip. Mixed statements (INSERT + QUERY) execute sequentially.

### Explain

```bash
curl -X POST http://localhost:50051/qql.QQL/Explain \
  -H "Content-Type: application/json" \
  -d '{"query": "QUERY \"search\" FROM docs LIMIT 5 USING HYBRID RERANK"}'
```

Returns the parsed plan without executing. Useful for debugging query structure.

### Health

```bash
curl http://localhost:50051/health
# {"ok":true,"version":"dev"}
```

The RPC health endpoint also reports Qdrant connectivity and collection count.

### Convert

```bash
curl -X POST http://localhost:50051/qql.QQL/Convert \
  -H "Content-Type: application/json" \
  -d '{"json_payload": "{\"points\":[{\"id\":1,\"payload\":{\"text\":\"hello\"}}]}"}'
```

Translates Qdrant REST API JSON into equivalent QQL statements. Supports create collection, create index, upsert, search, query, recommend, scroll, get, delete.

## Architecture

```
Request
  │
  ▼
┌─────────────────────────────────────────────────┐
│  Interceptor Chain                              │
│                                                 │
│  1. JWT Validation   (jwks.go)                  │
│     - Fetch & cache JWKS keys                   │
│     - Validate token signature, expiry, issuer  │
│     - Extract claims into context               │
│                                                 │
│  2. Policy Evaluation (policy.go)               │
│     - Match claims against YAML rules           │
│     - Determine: allowed ops, collections,      │
│       filter injection, limit caps              │
│                                                 │
│  3. Audit Meta Injection                        │
│     - Create empty AuditMeta in context         │
│     - Handler fills it with AST details         │
│     - Interceptor logs it after execution       │
│                                                 │
│  4. Handler Execution (handler.go)              │
│     - Parse QQL → AST                           │
│     - Enforce operation + collection rules      │
│     - Inject tenant filter into AST             │
│     - Enforce LIMIT cap                         │
│     - Execute against Qdrant                    │
│     - Return response                           │
│                                                 │
│  5. Audit Logging                               │
│     - Build structured JSON entry               │
│     - Write to file or stderr                   │
└─────────────────────────────────────────────────┘
```

When no gateway flags are set, the interceptor chain falls back to a simple `loggingInterceptor` that prints `procedure status duration` to stderr.

## Gateway Features

### JWT Authentication

Any IdP that exposes a JWKS endpoint works: Auth0, Okta, Keycloak, Firebase, Azure AD, Cognito, Descope, SuperTokens, or a custom server.

```bash
qql-go serve \
  --jwks-url https://idp.example.com/.well-known/jwks.json \
  --jwt-issuer https://idp.example.com \
  --jwt-audience my-app \
  --role-claim role \
  --tenant-claim org_id
```

The gateway fetches and caches JWKS keys (default 10min TTL), validates the JWT signature on every request, and extracts claims into the request context. Works with RSA, EC, and Ed25519 keys.

Token goes in the `Authorization` header:

```
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
```

### Policy Engine

A YAML file defines who can do what. Rules are evaluated top-to-bottom; first match wins. Unmatched requests are denied.

**Single filter injection:**

```yaml
rules:
  - match:
      claims:
        role: reader
    allow: [QUERY, SCROLL, SELECT, SHOW, EXPLAIN]
    inject:
      where:
        field: tenant_id        # Qdrant payload field
        from_claim: org_id      # JWT claim to read value from
        op: "="
    limits:
      max_limit: 50
```

**Multi-filter injection** (AND logic — all filters applied):

```yaml
  - match:
      claims:
        role: reader
    allow: [QUERY, SCROLL, SELECT, SHOW, EXPLAIN]
    inject:
      filters:
        - field: org              # tenant isolation
          from_claim: org_id
          op: "="
        - field: team             # department scoping (user's team + org-wide)
          from_claim: department
          op: "in"
        - field: access           # exclude confidential
          value: "confidential"
          op: "!="
    limits:
      max_limit: 50
```

The `filters:` list lets you inject multiple WHERE conditions. Each filter resolves independently — `from_claim` reads from JWT, `value` is static. All filters are AND'd together.

**What a rule controls:**

| Field | What it does |
|-------|-------------|
| `match.claims` | JWT claim values to match (AND logic) |
| `match.authenticated` | `true` = any valid token, `false` = no token |
| `allow` | Permitted operation types |
| `deny` | Blocked operation types (takes precedence over allow) |
| `collections` | Glob-pattern allowlist for collection names |
| `inject.where` | Single filter injection (legacy) |
| `inject.filters` | Multiple filter injection (AND logic) |
| `inject.where.field` | Payload field to filter on |
| `inject.where.from_claim` | JWT claim to read the filter value from |
| `inject.where.value` | Static filter value (mutually exclusive with from_claim) |
| `inject.where.op` | Comparison operator: `=`, `!=`, `in`, `not_in` |
| `limits.max_limit` | Cap the LIMIT value in queries |

### AST Injection

This is the key differentiator from a generic HTTP proxy. The gateway parses the QQL into an AST, then rewrites it before execution.

When alice (role=reader, org_id=acme-corp) sends:

```sql
QUERY 'company' FROM docs LIMIT 500
```

The gateway rewrites it to:

```sql
QUERY 'company' FROM docs LIMIT 50 WHERE tenant_id = 'acme-corp'
```

The filter is injected at the AST level — the user never writes it, can't bypass it, and Qdrant only scores documents matching the filter. The LIMIT is capped by policy.

For CTE-based queries, injection is recursive — the filter and limit cap apply to every CTE body, not just the top-level query. This prevents cross-tenant data leaking through prefetch subqueries.

### Audit Logging

Every request gets a structured JSON line:

```json
{"ts":"2026-06-18T10:00:00Z","subject":"usr_alice","email":"alice@example.com","tenant_id":"acme-corp","roles":["reader"],"operation":"EXEC","collection":"docs","query":"QUERY 'company' FROM docs LIMIT 5","status":"ok","latency_ms":12,"allowed":true}
```

Denied requests include the reason:

```json
{"ts":"2026-06-18T10:00:01Z","operation":"EXEC","status":"denied","denied":"true","denied_reason":"operation DELETE not permitted for current token","latency_ms":1}
```

Write to a file with `--audit-file audit.jsonl`, or omit to write to stderr.

## CLI Flags

### Core

| Flag | Default | Description |
|------|---------|-------------|
| `--listen` | `:50051` | Address to listen on |
| `--qdrant-url` | `http://localhost:6334` | Qdrant gRPC endpoint |
| `--api-key` | | Qdrant API key |

### Embedding

| Flag | Default | Description |
|------|---------|-------------|
| `--inference-mode` | `local` | `cloud`, `external`, or `local` |
| `--embedding-endpoint` | | Embedding server URL (required for local/external) |
| `--embedding-model` | `sentence-transformers/all-minilm-l6-v2` | Model name |
| `--embedding-dimension` | `384` | Vector dimension |

### Auth

| Flag | Default | Description |
|------|---------|-------------|
| `--jwks-url` | | JWKS endpoint URL |
| `--jwt-issuer` | | Expected `iss` claim |
| `--jwt-audience` | | Expected `aud` claim |
| `--jwks-cache-ttl` | `10m` | Key cache TTL |
| `--role-claim` | `role` | JWT claim path for roles |
| `--tenant-claim` | `tenant_id` | JWT claim path for tenant |

### Policy

| Flag | Default | Description |
|------|---------|-------------|
| `--policy-file` | | Path to YAML policy file |
| `--policy-reload` | `false` | Watch policy file for changes and reload automatically (zero downtime) |

### Rate Limiting

| Flag | Default | Description |
|------|---------|-------------|
| `--rate-limit` | `0` | Max requests per second per user (0 = unlimited) |
| `--rate-limit-capacity` | `20` | Max burst size per user |
| `--anon-rate-limit` | `0` | Max requests per second for anonymous (pre-auth) clients (0 = disabled) |
| `--anon-rate-limit-capacity` | `10` | Max burst size for anonymous clients |

Uses a token bucket per JWT subject (or client IP for anonymous). When the bucket is empty, requests get `429 Resource Exhausted` with a `Retry-After` header. Stale buckets are cleaned up every 5 minutes. Anonymous rate limiting runs before auth to prevent resource exhaustion from invalid-token floods.

### Query Complexity Guards

| Flag | Default | Description |
|------|---------|-------------|
| `--max-filter-depth` | `0` | Maximum nesting depth for filter expressions (0 = unlimited) |
| `--max-or-operands` | `0` | Maximum number of OR operands in a filter (0 = unlimited) |
| `--max-prefetch-depth` | `0` | Maximum nesting depth for CTE prefetch subqueries (0 = unlimited) |

These guards prevent resource exhaustion from deeply nested filters or excessive CTEs. Requests exceeding limits get `429 Resource Exhausted` with a descriptive message.

### CORS

| Flag | Default | Description |
|------|---------|-------------|
| `--allowed-origins` | `*` | Comma-separated list of allowed CORS origins, or `*` for all |

When set to specific origins, the gateway checks the request `Origin` header and only echoes it back when it matches. Responses include `Vary: Origin` when varying by origin.

### Templates

| Flag | Default | Description |
|------|---------|-------------|
| `--templates` | | Path to YAML query template file |

Templates let agents invoke named operations instead of writing raw QQL. Variables use `{name}` syntax, JWT claims are available as `{claims.<field>}`.

```yaml
templates:
  search_docs:
    description: "Search documents"
    query: "QUERY '{query}' FROM docs LIMIT {limit} USING HYBRID"
  tenant_scroll:
    description: "Scroll caller's tenant"
    query: "SCROLL FROM docs LIMIT {limit}"
    require_claims: [org_id]
```

### Audit

| Flag | Default | Description |
|------|---------|-------------|
| `--audit` | `false` | Enable structured audit logging |
| `--audit-file` | | Output file (default: stderr) |

## Usage

### Basic gateway (no auth)

```bash
qql-go serve --qdrant-url http://localhost:6334
```

Transparent QQL proxy. Any query goes through.

### With JWT auth

```bash
qql-go serve \
  --qdrant-url http://localhost:6334 \
  --jwks-url https://idp.example.com/.well-known/jwks.json \
  --tenant-claim org_id \
  --role-claim role
```

Validates tokens but doesn't enforce policy — all authenticated users can do everything.

### Full gateway (auth + policy + audit)

```bash
qql-go serve \
  --qdrant-url http://localhost:6334 \
  --jwks-url http://localhost:8081/.well-known/jwks.json \
  --jwt-issuer qql-demo-auth \
  --tenant-claim org_id \
  --role-claim role \
  --policy-file policies.yaml \
  --audit \
  --audit-file audit.jsonl \
  --inference-mode local \
  --embedding-endpoint http://127.0.0.1:1234/v1/embeddings \
  --embedding-model text-embedding-all-minilm-l6-v2-embedding \
  --embedding-dimension 384
```

### From Go

```go
import "github.com/srimon12/qql-go/server"

server.Run(server.Config{
    ListenAddr:   ":50051",
    QdrantURL:    "http://localhost:6334",
    QQLConfig:    &config.Config{InferenceMode: "cloud"},
    Gateway: &server.GatewayConfig{
        JWTValidator: server.NewJWTValidator(server.JWKSConfig{JWKSURL: "..."}),
        PolicyEngine: pe,
        Audit:        server.NewAuditLogger(nil, true),
    },
})
```

## File Map

| File | Purpose |
|------|---------|
| `server.go` | `Run()` — starts HTTP server, wires handler + interceptors |
| `handler.go` | `Handler` — implements `Exec`, `ExecBatch`, `Explain`, `Health`, `Convert` |
| `interceptor.go` | `loggingInterceptor` (fallback) + `chainInterceptors` (auth → policy → audit) |
| `jwt.go` | JWKS fetch/cache, JWT validation, claim extraction |
| `jwt_keys.go` | RSA/EC/Ed25519 key conversion from JWK format |
| `policy.go` | YAML policy loader, rule matching, `EvaluatedPolicy` |
| `inject.go` | `ASTInjector` — tenant filter injection, limit cap, collection scoping, CTE recursion |
| `audit.go` | `AuditLogger` — structured JSON lines, `AuditMeta` context pattern |
| `reload.go` | `PolicyReloader` — fsnotify watcher, atomic policy swap on file change |
| `ratelimit.go` | `RateLimiter` — per-key token bucket with cleanup |
| `templates.go` | `TemplateEngine` — named query templates with variable substitution |
| `cmd.go` | `NewServeCmd()` — cobra command with all flags |
| `gateway_test.go` | 28 tests: policy engine, AST injection, claim extraction, glob matching, rate limiter, templates |

## Testing

```bash
go test ./server/... -v
```

Covers: policy rule matching (admin, reader, nil claims, deny overrides, rule order), AST injection (tenant filter, merge with existing, limit cap, collection scoping, operation deny, static value), claim extraction (simple, nested with `#` separator, slice claims), glob matching, rate limiter (disabled, allow/block, refill, retry-after), and template engine (resolve, claim substitution, not found, required claims, param extraction).

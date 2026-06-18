# Gateway Demo — Multi-Tenant Retrieval with JWT + Policy Enforcement

Same Qdrant cluster. Same query. Different users. Different results.

## What's here

```
server-gateway/
├── 01-seed.qql         # QQL seed script (create collection, indexes, insert docs)
├── auth_server.py      # FastAPI mini IdP — login, JWKS, custom tokens
├── run.py              # Demo runner — seeds, starts gateway, runs scenarios
├── policies.yaml       # Gateway policy rules
├── pyproject.toml      # uv project
└── README.md
```

## Architecture

![Architecture Diagram](./architecture.png)

The core magic happens at the AST Parser & Injector level. Instead of filtering results *after* they return from the database, the gateway rewrites the query structure directly in memory (recursively handling CTEs and limits) before generating the physical `QueryBatch` payload for Qdrant.

## Prerequisites

- **Qdrant** running (default `http://localhost:6334` gRPC)
- **Go 1.24+** (to build the gateway binary)
- **Python 3.10+** with [uv](https://docs.astral.sh/uv/)
- **Embedding server** on `http://127.0.0.1:1234` (e.g. LM Studio, Ollama, FastEmbed)

## Quick start

```bash
# 1. Start Qdrant (if not running)
docker run -d --name qdrant -p 6334:6333 qdrant/qdrant:latest

# 2. Run the demo (builds, seeds, starts services, runs scenarios, cleans up)
cd examples/server-gateway
uv sync
uv run run.py
```

The runner does everything: builds the binary, seeds data via `qql-go execute 01-seed.qql`, starts the auth server and gateway, runs all scenarios, shows the audit trail, and cleans up.

## What the demo shows

| Step | Scenario | Result |
|------|----------|--------|
| 1 | Authenticate 5 users | Different roles, tenants, departments |
| 2 | Same SCROLL, different tenants | alice→9 acme docs, carol→6 globex docs |
| 3 | Admin bypasses tenant filter | bob sees all |
| 4 | DELETE blocked for reader | DENIED |
| 5 | LIMIT 500 capped to 50 | Policy enforced |
| 6 | Agent restricted to public docs | 6 public docs only |
| 7 | No token → rejected | DENIED |
| 8 | Audit trail | 9 structured JSON entries |

## Flags

```bash
uv run run.py --qdrant-url http://10.0.0.5:6334
uv run run.py --embedding-url http://localhost:11434/v1/embeddings
uv run run.py --no-auth --no-gateway   # skip starting services
```

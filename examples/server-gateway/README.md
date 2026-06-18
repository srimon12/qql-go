# Gateway Demo — Multi-Tenant Retrieval with JWT + Policy Enforcement

Same Qdrant cluster. Same query. Different users. Different results.

## What's here

```
server-gateway/
├── ui/
│   └── app.py              # Streamlit dashboard
├── 01-seed.qql             # QQL seed script
├── auth_server.py          # FastAPI mini IdP
├── run.py                  # CLI demo runner
├── policies.yaml           # Gateway policy rules
├── templates.yaml          # Query templates
├── start_ui.sh             # One-command launcher (all services + dashboard)
├── pyproject.toml          # uv project
└── README.md
```

## Prerequisites

- **Qdrant** running (default `http://localhost:6334` gRPC)
- **Go 1.24+** (to build the gateway binary)
- **Python 3.10+** with [uv](https://docs.astral.sh/uv/)
- **Embedding server** on `http://127.0.0.1:1234`

## Quick start — Dashboard

```bash
# 1. Start Qdrant
docker run -d --name qdrant -p 6334:6333 qdrant/qdrant:latest

# 2. Launch everything (auth, gateway, dashboard)
cd examples/server-gateway
uv sync
./start_ui.sh

# 3. Open http://localhost:8501
```

## Quick start — CLI demo

```bash
uv run run.py
```

## Dashboard tabs

| Tab | What it does |
|-----|-------------|
| **Query** | Execute QQL with policy enforcement, see results |
| **Explain** | View query plans without execution |
| **Tenant Isolation** | Same query, 3 users, side-by-side results |
| **Templates** | Invoke named operations with variable substitution |
| **Policy Editor** | View/edit policies.yaml live |
| **Audit Log** | Structured audit entries with filtering |
| **Convert** | REST JSON → QQL converter |

## Flags

```bash
uv run run.py --qdrant-url http://10.0.0.5:6334
uv run run.py --embedding-url http://localhost:11434/v1/embeddings
uv run run.py --no-auth --no-gateway
```

# Gateway Demo — Multi-Org Policy Enforcement for Qdrant

4 organizations. 15 teams. 64 documents. Same Qdrant cluster. Different users see different data.

## What's here

```
server-gateway/
├── ui/
│   ├── app.py              # Streamlit dashboard (entry point)
│   ├── utils.py            # Shared helpers
│   ├── tab_query.py        # Execute QQL with policy enforcement
│   ├── tab_explain.py      # Query plans
│   ├── tab_tenant.py       # Side-by-side tenant isolation demo
│   ├── tab_templates.py    # Named operations with variable substitution
│   ├── tab_policy.py       # View/edit policies.yaml live
│   ├── tab_audit.py        # Structured audit log
│   └── tab_convert.py      # 110 Qdrant REST JSON examples → QQL
├── seed.py                 # Seed Qdrant with 64 multi-org documents
├── auth_server.py          # FastAPI mini IdP (30+ users, 4 orgs)
├── run.py                  # CLI demo runner (no UI)
├── start_ui.sh             # One-command launcher (auth + gateway + dashboard)
├── policies.yaml           # Gateway policy rules
├── templates.yaml          # Query templates for agents
├── pyproject.toml          # uv project
└── README.md
```

## Dataset

| Org | Industry | Teams | Users | Docs |
|-----|----------|-------|-------|------|
| ACME Corp | Cloud Infrastructure | engineering, finance, security, all | 7 | 14 |
| Globex Corp | Digital Banking | engineering, finance, compliance, product, all | 6 | 15 |
| Initech Inc | Healthcare Devices | engineering, clinical, compliance, rd, all | 6 | 15 |
| Umbrella Corp | Industrial IoT | engineering, supply_chain, quality, operations, all | 6 | 15 |
| — | Public docs | — | — | 5 |

**64 documents** with realistic content: architecture docs, financial reports, security audits, clinical trials, compliance reports, supply chain data, quality policies, product roadmaps, HR policies, public documentation.

## Access model

| Role | Sees | Injected filter |
|------|------|-----------------|
| **Reader** | Own team + org-wide docs, no confidential | `org = X AND team IN (dept, all) AND access != confidential` |
| **Manager** | Everything in org (including confidential) | `org = X` |
| **Admin** | Everything in org (full CRUD) | `org = X` |
| **Agent** | Public docs only, any org | `access = public` |

## Prerequisites

- **Qdrant** running (default `http://localhost:6334` gRPC, `:6333` REST)
- **Go 1.24+**
- **Python 3.10+** with [uv](https://docs.astral.sh/uv/)
- **Embedding server** on `http://127.0.0.1:1234` (LM Studio, Ollama, FastEmbed)

## Quick start — Dashboard

```bash
# 1. Start Qdrant
docker run -d --name qdrant -p 6334:6333 qdrant/qdrant:latest

# 2. Launch everything
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
| **Query** | Execute QQL, see full payload in table, policy denial feedback |
| **Explain** | View query plans without execution |
| **Tenant Isolation** | Pick 2-5 users, run same query, see side-by-side results |
| **Templates** | Invoke named operations with variable substitution |
| **Policy Editor** | View/edit policies.yaml, parsed rule summary, save with validation |
| **Audit Log** | Structured JSONL with filtering (allowed/denied/errors) |
| **Convert** | 110 Qdrant REST JSON examples in 15 categories, convert to QQL, execute |

## Gateway features

| Feature | Flag | Description |
|---------|------|-------------|
| JWT auth | `--jwks-url` | Any IdP via JWKS (Auth0, Okta, Keycloak, etc.) |
| Policy engine | `--policy-file` | YAML rules: operation control, collection scoping, filter injection |
| Policy hot-reload | `--policy-reload` | Watch file for changes, atomic swap |
| Rate limiting | `--rate-limit 10` | Per-user token bucket, 429 with Retry-After |
| Anonymous rate limiting | `--anon-rate-limit 5` | Pre-auth rate limiter by client IP, prevents invalid-token floods |
| Query complexity guards | `--max-filter-depth 10` | Max filter nesting, OR operands, and prefetch depth |
| CORS origins | `--allowed-origins` | Configurable CORS allowlist (replaces hardcoded `*`) |
| Query templates | `--templates` | Named operations for agents |
| Audit logging | `--audit --audit-file` | Structured JSON per request |
| Embeddings | `--embedding-endpoint` | Local/external embedding server |

## Example users

| User | Org | Team | Role |
|------|-----|------|------|
| alice@acme.com | acme | engineering | reader |
| bob@acme.com | acme | engineering | admin |
| carol@globex.com | globex | finance | reader |
| eve@globex.com | globex | engineering | manager |
| finn@initech.com | initech | engineering | admin |
| wendy@initech.com | initech | clinical | reader |
| quinn@umbrella.com | umbrella | operations | admin |
| glenn@umbrella.com | umbrella | engineering | reader |

Password pattern: `{name}123` (e.g. `alice123`, `bob123`)

## What the demo shows

| Step | Scenario | Result |
|------|----------|--------|
| 1 | Alice (acme/eng) queries docs | Sees ACME engineering + org-wide docs |
| 2 | Dave (acme/finance) queries docs | Sees ACME finance + org-wide docs |
| 3 | Bob (acme/admin) queries docs | Sees all ACME docs (including confidential) |
| 4 | Carol (globex/finance) queries docs | Sees Globex finance + org-wide docs |
| 5 | Alice tries DELETE | DENIED — readers can't delete |
| 6 | Eve (manager) tries INSERT | Allowed — managers can write |
| 7 | Agent queries docs | Only public docs |
| 8 | Unauthenticated request | Rejected |
| 9 | Policy hot-reload | Edit policies.yaml → changes take effect immediately |

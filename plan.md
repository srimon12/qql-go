The blog's core idea is simple and powerful: **don't filter results after retrieval — inject the permission boundary into the query before scoring.** Forbidden chunks never get embedded into a candidate set, never get scored, never land in the prompt.

But the blog ties it to one specific auth stack. Your gateway should absorb the *pattern*, not the product. Here's how.

## The Principle, Generalized

The blog's architecture is:

```
User JWT → Authorizer → allowed_doc_ids → Qdrant must filter → search
```

Strip away Authorizer. The generic version is:

```
User JWT → JWKS validate → extract claims → policy engine → QQL filter injection → search
```

Three moving parts: **token validation** (any IdP), **policy resolution** (claim → filter mapping), **AST injection** (transform the query before execution). All three live inside the gateway's interceptor chain. The QQL AST is the key — you're not post-filtering JSON results, you're rewriting the query *before* it reaches Qdrant.

## Architecture: The Gateway as Policy Enforcement Point

```
Client (JWT in Authorization header)
  │
  ▼
┌─────────────────────────────────────────────────────┐
│  qql-go gateway                                     │
│                                                     │
│  1. JWKS Interceptor                                │
│     • fetch & cache keys from --jwks-url            │
│     • validate JWT signature, expiry, issuer         │
│     • extract claims into context                   │
│                                                     │
│  2. Policy Interceptor                              │
│     • match claims → policy rules                   │
│     • generate filter AST nodes                     │
│     • decide: allow / deny / transform              │
│                                                     │
│  3. Tenant Injection Interceptor                    │
│     • parse QQL → AST                               │
│     • inject policy filters into WHERE clause       │
│     • enforce operation allowlists                  │
│     • enforce LIMIT caps                            │
│                                                     │
│  4. Execute against Qdrant                          │
│  5. Structured audit log                            │
└─────────────────────────────────────────────────────┘
```

## JWKS Interceptor — Any IdP

One flag: `--jwks-url https://your-idp.com/.well-known/jwks.json`

Works with Auth0, Okta, Keycloak, Firebase, Azure AD, Cognito, Descope, SuperTokens — anything that exposes a JWKS endpoint. The gateway fetches the key set, caches it with TTL, validates the JWT signature on every request, and extracts claims into the request context.

```go
type JWTConfig struct {
    JWKSURL      string        // e.g. "https://idp.example.com/.well-known/jwks.json"
    Issuer       string        // optional: validate "iss" claim
    Audience     string        // optional: validate "aud" claim
    CacheTTL     time.Duration // key refresh interval
    ClaimMapping ClaimMapping  // which claims carry what info
}

type ClaimMapping struct {
    TenantClaim string // e.g. "org_id", "tenant", "https://myapp.com/tenant"
    RoleClaim   string // e.g. "role", "https://myapp.com/roles"
    SubjectClaim string // e.g. "sub", "email"
}
```

This is generic. No vendor lock-in. The claim names are configurable because every IdP structures JWTs differently.

## Policy Engine — The Real Abstraction

This is where qql-go becomes more than a proxy. Policies are **claim → QQL filter mappings** that the gateway enforces automatically. The policy engine doesn't just allow/deny — it *transforms queries*.

Policy file (`policies.yaml`):

```yaml
# Default: authenticated users can QUERY, no mutations
- match:
    authenticated: true
  allow: [QUERY, EXPLAIN, SCROLL, SHOW]
  deny:  [DELETE, ALTER, UPDATE, INSERT]

# Admin role: full access
- match:
    claims:
      role: admin
  allow: [QUERY, INSERT, UPDATE, DELETE, ALTER, CREATE, DROP]
  inject:
    # no filter injection — admin sees everything

# Tenant isolation: auto-inject tenant filter on every QUERY
- match:
    claims:
      role: [reader, writer]
  allow: [QUERY, SCROLL]
  inject:
    where:
      field: tenant_id
      from_claim: org_id    # JWT claim "org_id" → WHERE tenant_id = <value>
  limits:
    max_limit: 100

# Finance team: can QUERY finance collections
- match:
    claims:
      role: reader
      department: finance
  allow: [QUERY]
  collections: [finance_*]       # glob pattern — can only query matching collections
  inject:
    where:
      field: department
      from_claim: department

# Agent tokens: narrow surface
- match:
    claims:
      token_type: agent
  allow: [QUERY]
  collections: [public_*]
  inject:
    where:
      field: access_level
      value: public
  limits:
    max_limit: 20
```

The policy engine reads the JWT claims, matches the first applicable rule, and returns:
- **Operation allowlist** — what QQL operations this token can run
- **Collection allowlist** — what collections this token can query (glob patterns)
- **Filter injection** — WHERE clauses to auto-inject based on JWT claims
- **Limit caps** — maximum LIMIT the token can request

## AST Injection — The Unique Advantage

This is what makes qql-go's gateway fundamentally different from an HTTP proxy. You have the parsed AST. You can *transform* the query, not just filter results.

When a request comes in:

```sql
-- User sends:
QUERY 'chest pain treatment' FROM medical_docs LIMIT 50

-- After policy injection (tenant_id from JWT claim "org_id"):
QUERY 'chest pain treatment' FROM medical_docs LIMIT 50
  WHERE tenant_id = 'acme-corp'

-- After limit cap (policy says max 20):
QUERY 'chest pain treatment' FROM medical_docs LIMIT 20
  WHERE tenant_id = 'acme-corp'
```

The user never writes `WHERE tenant_id = 'acme-corp'`. The gateway injects it from the JWT. The user can't bypass it because the injection happens *after* parsing, on the AST, before execution.

For destructive operations:

```sql
-- User sends (reader role):
DELETE FROM docs WHERE status = 'archived'

-- Policy engine: reader role → deny DELETE
-- → Error: operation DELETE not permitted for role 'reader'
```

For collection restrictions:

```sql
-- User sends (finance role, policy says collections: [finance_*]):
QUERY 'quarterly results' FROM engineering_docs LIMIT 5

-- Policy engine: collection 'engineering_docs' doesn't match 'finance_*'
-- → Error: access to collection 'engineering_docs' not permitted
```

## What This Gives You That The Blog Doesn't

The blog's approach is **document-level** — each chunk has a source file, the gateway asks "which files can this user see?", and injects a `must` filter on the `source` field. That's fine for file-based RAG.

Your gateway operates at the **query level**. You can do:

| Capability | Blog (document-level) | qql-go (query-level) |
|---|---|---|
| Tenant isolation | Need per-tenant collections or doc-level ACLs | Auto-inject `WHERE tenant_id` from JWT |
| Operation control | Not possible (HTTP proxy can't parse queries) | Allow/deny by operation type (QUERY, DELETE, etc.) |
| Collection scoping | Not possible | Glob-pattern collection allowlists |
| Limit enforcement | Not possible | Cap LIMIT per role |
| Filter injection | Manual, app must build filter | Automatic, gateway transforms AST |
| Multi-tenant same collection | Hard (need doc-level ACL tuples) | Easy (inject tenant_id filter) |
| Agent safety | Narrow surface via templates | Narrow surface via operation+collection+limit policies |

The blog solves "Alice can't see finance docs." Your gateway solves "Alice can QUERY medical_docs with LIMIT ≤ 20, filtered to her tenant, and can't DELETE anything."

## Implementation Roadmap

| Phase | What | Effort | Value |
|-------|------|--------|-------|
| **1** | `--jwks-url` flag, JWT validation, claim extraction | 2 days | Auth works with any IdP |
| **2** | `policies.yaml` loader, claim→rule matching | 2 days | Policy engine exists |
| **3** | AST injection: `inject.where` from claims | 3 days | Tenant isolation without app changes |
| **4** | Operation allowlists + collection allowlists | 1 day | Destructive op protection |
| **5** | Limit caps + structured audit log | 1 day | Agent safety + observability |
| **6** | `policies.yaml` hot-reload (fsnotify) | 1 day | No restart for policy changes |
| **7** | Per-collection policy overrides | 2 days | Fine-grained control |

The JWKS interceptor is generic infra. The policy engine is the product differentiator. The AST injection is the technical moat — nobody else can do this because nobody else has the parsed query language.

Want me to start implementing the JWKS interceptor or the policy engine first?
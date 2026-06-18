"""
seed.py — Seed Qdrant through the qql-go gateway via ExecBatch.

Starts the gateway, sends QQL statements through the Connect RPC endpoint.
No custom HTTP calls. No QQL script files. Just the gateway.

Usage:
    uv run python seed.py
    uv run python seed.py --gateway http://localhost:50051
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
import urllib.request

from rich.console import Console

console = Console()

SCRIPT_DIR = __import__("pathlib").Path(__file__).parent
REPO_ROOT = SCRIPT_DIR.parent.parent

# ---------------------------------------------------------------------------
# Dataset — 4 orgs, 15 teams, 64 documents
# ---------------------------------------------------------------------------

DOCUMENTS = [
    # ACME Corp — Engineering
    {"id": 1001, "text": "ACME microservices run on Kubernetes with Go 1.24. Each service has a gRPC gateway, Prometheus metrics, and structured JSON logging. Deployment is via ArgoCD with canary releases.", "org": "acme", "team": "engineering", "access": "internal", "topic": "architecture", "doc_type": "runbook"},
    {"id": 1002, "text": "ACME API rate limits: Free tier 100 req/min, Pro 1000 req/min, Enterprise unlimited with dedicated support. Rate limit headers are X-RateLimit-Remaining and X-RateLimit-Reset.", "org": "acme", "team": "engineering", "access": "public", "topic": "api", "doc_type": "documentation"},
    {"id": 1003, "text": "ACME incident response: P1 triggers PagerDuty, war room in #incident channel, postmortem within 48 hours. P2 gets a Slack alert and 4-hour SLA. P3 is triaged in the next standup.", "org": "acme", "team": "engineering", "access": "internal", "topic": "operations", "doc_type": "runbook"},
    {"id": 1004, "text": "ACME database migration playbook: always create a backup, run migrations in a transaction, test rollback on staging first, and have a revert PR ready before merging.", "org": "acme", "team": "engineering", "access": "internal", "topic": "database", "doc_type": "runbook"},
    {"id": 1005, "text": "ACME vector search infrastructure uses Qdrant with hybrid retrieval (dense + sparse). Collections are sharded by tenant_id with payload indexes on department, access_level, and topic.", "org": "acme", "team": "engineering", "access": "internal", "topic": "search", "doc_type": "architecture"},
    # ACME Corp — Finance
    {"id": 1006, "text": "ACME Q4 2025 revenue was $14.2M, up 23% QoQ. Monthly cash burn is $1.1M, giving 26 months of runway. Enterprise ARR grew 45% driven by three Fortune 500 deals.", "org": "acme", "team": "finance", "access": "confidential", "topic": "financials", "doc_type": "report"},
    {"id": 1007, "text": "ACME pricing: Starter $49/mo, Pro $199/mo, Enterprise custom. Annual discount 20%. Volume discounts available for 100+ seats.", "org": "acme", "team": "finance", "access": "public", "topic": "pricing", "doc_type": "documentation"},
    {"id": 1008, "text": "ACME headcount plan: hire 8 engineers, 3 sales reps, and 2 support engineers in Q1 2026. Total comp budget increase: $2.4M. Equity pool: 15% remaining.", "org": "acme", "team": "finance", "access": "confidential", "topic": "planning", "doc_type": "report"},
    # ACME Corp — Security
    {"id": 1009, "text": "ACME SOC2 Type II audit passed with zero findings. Next audit scheduled for September 2026. All engineering teams must complete security training by March.", "org": "acme", "team": "security", "access": "internal", "topic": "compliance", "doc_type": "report"},
    {"id": 1010, "text": "ACME vulnerability disclosure policy: report via security@acme.com, triage within 24 hours, fix within 7 days for critical, 30 days for high. Bug bounty: $500-$5000.", "org": "acme", "team": "security", "access": "public", "topic": "security", "doc_type": "policy"},
    {"id": 1011, "text": "ACME penetration test results (January 2026): 2 medium findings (CORS misconfiguration, missing rate limit on login), 5 low findings. All remediated within SLA.", "org": "acme", "team": "security", "access": "confidential", "topic": "security", "doc_type": "report"},
    # ACME Corp — Cross-team
    {"id": 1012, "text": "ACME company all-hands is every Friday 3pm. Submit questions via the #all-hands Slack channel. Recordings are posted to the internal wiki within 24 hours.", "org": "acme", "team": "all", "access": "internal", "topic": "company", "doc_type": "announcement"},
    {"id": 1013, "text": "ACME holiday schedule 2026: office closed Dec 24-Jan 1, Memorial Day, July 4th, Thanksgiving (Thu-Fri). Flexible PTO: minimum 15 days recommended.", "org": "acme", "team": "all", "access": "public", "topic": "hr", "doc_type": "policy"},
    {"id": 1014, "text": "ACME remote work policy: 3 days in-office minimum for Bay Area employees. Fully remote available for non-Bay Area with manager approval. Core hours 10am-4pm PT.", "org": "acme", "team": "all", "access": "internal", "topic": "hr", "doc_type": "policy"},
    # Globex Corp — Engineering
    {"id": 2001, "text": "Globex platform runs on Rust services with a React frontend. Deployment is via GitLab CI to AWS EKS. Blue-green deployments with automatic rollback on error rate spikes.", "org": "globex", "team": "engineering", "access": "internal", "topic": "architecture", "doc_type": "documentation"},
    {"id": 2002, "text": "Globex API authentication uses OAuth2 with PKCE. Access tokens expire in 15 minutes, refresh tokens in 7 days. All endpoints require HTTPS.", "org": "globex", "team": "engineering", "access": "internal", "topic": "security", "doc_type": "runbook"},
    {"id": 2003, "text": "Globex database uses PostgreSQL with read replicas. Connection pooling via PgBouncer. Query timeout: 30 seconds. Slow query log threshold: 500ms.", "org": "globex", "team": "engineering", "access": "internal", "topic": "database", "doc_type": "runbook"},
    {"id": 2004, "text": "Globex mobile SDK supports iOS 15+ and Android 12+. Push notifications via Firebase. Biometric auth (Face ID, fingerprint) for transaction approval.", "org": "globex", "team": "engineering", "access": "internal", "topic": "mobile", "doc_type": "documentation"},
    # Globex Corp — Finance
    {"id": 2005, "text": "Globex Series B closed at $50M valuation. Cash position: $12M. Burn rate: $800K/month. Break-even projected Q3 2026.", "org": "globex", "team": "finance", "access": "confidential", "topic": "financials", "doc_type": "report"},
    {"id": 2006, "text": "Globex enterprise deals: Acme Industries $240K ARR, Wayne Enterprises $180K ARR, Stark Corp pending $500K. Pipeline total: $2.1M.", "org": "globex", "team": "finance", "access": "confidential", "topic": "sales", "doc_type": "report"},
    {"id": 2007, "text": "Globex pricing: Basic free (100 txns/mo), Pro $29/mo (unlimited txns), Enterprise custom (dedicated support, SLA). Transaction fee: 0.5% for Pro.", "org": "globex", "team": "finance", "access": "public", "topic": "pricing", "doc_type": "documentation"},
    # Globex Corp — Compliance
    {"id": 2008, "text": "Globex PCI DSS Level 1 compliance: annual audit by QSA, quarterly ASV scans, network segmentation, encryption at rest and in transit. Last audit: November 2025.", "org": "globex", "team": "compliance", "access": "confidential", "topic": "compliance", "doc_type": "report"},
    {"id": 2009, "text": "Globex KYC process: identity verification via Jumio, address verification via utility bill, sanctions screening via Dow Jones. Average processing time: 4 minutes.", "org": "globex", "team": "compliance", "access": "internal", "topic": "compliance", "doc_type": "runbook"},
    {"id": 2010, "text": "Globex GDPR data retention: transaction data 7 years (regulatory), marketing data until consent withdrawal, support tickets 2 years. Data deletion requests processed within 30 days.", "org": "globex", "team": "compliance", "access": "internal", "topic": "compliance", "doc_type": "policy"},
    # Globex Corp — Product
    {"id": 2011, "text": "Globex product roadmap Q1 2026: instant transfers, multi-currency support, budgeting tools. Q2: investment portfolio, crypto on-ramp. Q3: business accounts.", "org": "globex", "team": "product", "access": "internal", "topic": "roadmap", "doc_type": "planning"},
    {"id": 2012, "text": "Globex NPS score: 72 (Q4 2025). Top praise: fast transfers, clean UI. Top complaints: limited currency support, slow customer service response.", "org": "globex", "team": "product", "access": "internal", "topic": "feedback", "doc_type": "report"},
    {"id": 2013, "text": "Globex user onboarding: sign up in 2 minutes, KYC in 4 minutes, first transaction in 6 minutes. Conversion rate: 68% from signup to first transaction.", "org": "globex", "team": "product", "access": "internal", "topic": "onboarding", "doc_type": "analytics"},
    # Globex Corp — Cross-team
    {"id": 2014, "text": "Globex remote-first policy: all employees can work from anywhere. Quarterly in-person offsites. Core hours: overlap 4 hours with your team's timezone.", "org": "globex", "team": "all", "access": "internal", "topic": "hr", "doc_type": "policy"},
    {"id": 2015, "text": "Globex benefits: health insurance (US + international), $1000/year learning budget, $500/year wellness budget, 4 weeks PTO, 12 weeks parental leave.", "org": "globex", "team": "all", "access": "public", "topic": "hr", "doc_type": "documentation"},
    # Initech Inc — Engineering
    {"id": 3001, "text": "Initech firmware is written in C for ARM Cortex-M4. OTA updates via BLE. Battery life target: 14 days. Regulatory class: FDA Class II.", "org": "initech", "team": "engineering", "access": "internal", "topic": "firmware", "doc_type": "documentation"},
    {"id": 3002, "text": "Initech device communication protocol: MQTT over TLS 1.3. Message format: Protocol Buffers. Telemetry interval: 5 minutes. Alert interval: immediate.", "org": "initech", "team": "engineering", "access": "internal", "topic": "protocol", "doc_type": "specification"},
    {"id": 3003, "text": "Initech hardware revision B3 changes: upgraded accelerometer (BMI270), added temperature sensor (TMP117), improved antenna design for better BLE range.", "org": "initech", "team": "engineering", "access": "internal", "topic": "hardware", "doc_type": "changelog"},
    {"id": 3004, "text": "Initech device pairing: NFC tap to initiate, BLE handshake within 10 seconds, TLS certificate exchange, user confirmation on device screen.", "org": "initech", "team": "engineering", "access": "internal", "topic": "pairing", "doc_type": "runbook"},
    # Initech Inc — Clinical
    {"id": 3005, "text": "Initech clinical trial results (Phase II, n=200): device accuracy 97.3% vs gold standard. False positive rate: 1.2%. False negative rate: 1.5%. No serious adverse events.", "org": "initech", "team": "clinical", "access": "confidential", "topic": "clinical_trial", "doc_type": "report"},
    {"id": 3006, "text": "Initech patient data handling: all PHI encrypted at rest (AES-256) and in transit (TLS 1.3). Data stored in HIPAA-compliant cloud. Access logged and audited.", "org": "initech", "team": "clinical", "access": "confidential", "topic": "hipaa", "doc_type": "policy"},
    {"id": 3007, "text": "Initech adverse event reporting: report within 24 hours to safety team, FDA MedWatch within 15 days for serious events, annual reports for all events.", "org": "initech", "team": "clinical", "access": "internal", "topic": "safety", "doc_type": "runbook"},
    # Initech Inc — Compliance
    {"id": 3008, "text": "Initech FDA 510(k) submission status: predicate device identified, substantial equivalence demonstrated, review pending. Expected clearance: Q2 2026.", "org": "initech", "team": "compliance", "access": "confidential", "topic": "regulatory", "doc_type": "report"},
    {"id": 3009, "text": "Initech quality management system: ISO 13485 certified. Design controls per 21 CFR 820.30. CAPA process: identify, investigate, correct, verify, prevent.", "org": "initech", "team": "compliance", "access": "internal", "topic": "quality", "doc_type": "documentation"},
    {"id": 3010, "text": "Initech supplier qualification: all component suppliers must be ISO 9001 certified. Annual audits. Incoming inspection: 100% for critical components, AQL sampling for others.", "org": "initech", "team": "compliance", "access": "internal", "topic": "supply_chain", "doc_type": "policy"},
    # Initech Inc — R&D
    {"id": 3011, "text": "Initech R&D project Atlas: next-gen sensor with 10x sensitivity improvement. Prototype expected Q3 2026. Budget: $1.8M. Team: 4 engineers, 2 scientists.", "org": "initech", "team": "rd", "access": "confidential", "topic": "research", "doc_type": "planning"},
    {"id": 3012, "text": "Initech patent portfolio: 12 granted, 4 pending. Key patent: US11234567 (adaptive threshold algorithm). Licensing revenue: $200K/year.", "org": "initech", "team": "rd", "access": "confidential", "topic": "ip", "doc_type": "report"},
    {"id": 3013, "text": "Initech machine learning model for anomaly detection: trained on 50K patient records, AUC 0.98, deployed on-device via TensorFlow Lite. Retraining quarterly.", "org": "initech", "team": "rd", "access": "internal", "topic": "ml", "doc_type": "documentation"},
    # Initech Inc — Cross-team
    {"id": 3014, "text": "Initech mission: make healthcare monitoring accessible to everyone. Founded 2019, 150 employees, offices in Boston and San Diego.", "org": "initech", "team": "all", "access": "public", "topic": "company", "doc_type": "about"},
    {"id": 3015, "text": "Initech employee handbook: code of conduct, anti-harassment policy, dress code (business casual), travel policy (economy class under 4 hours), expense limits.", "org": "initech", "team": "all", "access": "internal", "topic": "hr", "doc_type": "policy"},
    # Umbrella Corp — Engineering
    {"id": 4001, "text": "Umbrella sensor network: 10K nodes across 50 factories. Communication: LoRaWAN for field sensors, Ethernet for edge gateways. Data volume: 2TB/day.", "org": "umbrella", "team": "engineering", "access": "internal", "topic": "iot", "doc_type": "architecture"},
    {"id": 4002, "text": "Umbrella edge computing: each factory has a Kubernetes cluster (3 nodes). Local processing for latency-sensitive alerts (<100ms). Cloud sync every 5 minutes.", "org": "umbrella", "team": "engineering", "access": "internal", "topic": "edge", "doc_type": "documentation"},
    {"id": 4003, "text": "Umbrella predictive maintenance model: trained on vibration, temperature, and current data. Predicts failures 48 hours ahead with 92% accuracy. Saves $2M/year in unplanned downtime.", "org": "umbrella", "team": "engineering", "access": "internal", "topic": "ml", "doc_type": "report"},
    {"id": 4004, "text": "Umbrella device firmware OTA: staged rollout (1% → 10% → 50% → 100%), automatic rollback on error spike, signed firmware images, secure boot chain.", "org": "umbrella", "team": "engineering", "access": "internal", "topic": "firmware", "doc_type": "runbook"},
    # Umbrella Corp — Supply Chain
    {"id": 4005, "text": "Umbrella supplier scorecard: on-time delivery (target 95%), defect rate (target <0.1%), lead time (target <14 days). Quarterly reviews with top 20 suppliers.", "org": "umbrella", "team": "supply_chain", "access": "internal", "topic": "suppliers", "doc_type": "report"},
    {"id": 4006, "text": "Umbrella inventory policy: safety stock = 2 weeks demand for A-items, 4 weeks for B-items, 8 weeks for C-items. Reorder point = lead time demand + safety stock.", "org": "umbrella", "team": "supply_chain", "access": "internal", "topic": "inventory", "doc_type": "policy"},
    {"id": 4007, "text": "Umbrella logistics: primary carrier FedEx (60%), secondary UPS (30%), expedited DHL (10%). Average shipping cost: $12.50/unit domestic, $45/unit international.", "org": "umbrella", "team": "supply_chain", "access": "internal", "topic": "logistics", "doc_type": "documentation"},
    # Umbrella Corp — Quality
    {"id": 4008, "text": "Umbrella quality control: 100% inspection for safety-critical components, AQL 1.0 for standard parts, AQL 2.5 for packaging. Reject rate target: <0.5%.", "org": "umbrella", "team": "quality", "access": "internal", "topic": "qc", "doc_type": "policy"},
    {"id": 4009, "text": "Umbrella ISO 9001:2015 certification renewed January 2026. Zero non-conformances. Two observations: improve document control, enhance training records.", "org": "umbrella", "team": "quality", "access": "internal", "topic": "certification", "doc_type": "report"},
    {"id": 4010, "text": "Umbrella product recall process: identify affected lots, notify customers within 24 hours, arrange returns, root cause analysis within 7 days, corrective action within 30 days.", "org": "umbrella", "team": "quality", "access": "internal", "topic": "recall", "doc_type": "runbook"},
    # Umbrella Corp — Operations
    {"id": 4011, "text": "Umbrella factory automation: 60% of assembly is robotic. Human oversight for quality checkpoints. Target: 80% automation by 2027. Investment: $5M.", "org": "umbrella", "team": "operations", "access": "internal", "topic": "automation", "doc_type": "planning"},
    {"id": 4012, "text": "Umbrella energy management: solar panels on 3 factories, LED lighting everywhere, HVAC optimization via IoT sensors. Energy cost reduction: 25% since 2024.", "org": "umbrella", "team": "operations", "access": "internal", "topic": "sustainability", "doc_type": "report"},
    {"id": 4013, "text": "Umbrella safety record: 300 days without lost-time incident. Monthly safety drills. PPE mandatory in all production areas. Near-miss reporting via mobile app.", "org": "umbrella", "team": "operations", "access": "internal", "topic": "safety", "doc_type": "report"},
    # Umbrella Corp — Cross-team
    {"id": 4014, "text": "Umbrella founded 1987, headquarters in Detroit. 2000 employees, 50 factories worldwide. Revenue $500M. CEO: Maria Chen.", "org": "umbrella", "team": "all", "access": "public", "topic": "company", "doc_type": "about"},
    {"id": 4015, "text": "Umbrella benefits: 401k match 6%, health/dental/vision, tuition reimbursement $5K/year, gym membership, employee stock purchase plan at 15% discount.", "org": "umbrella", "team": "all", "access": "public", "topic": "hr", "doc_type": "documentation"},
    # Public docs
    {"id": 9001, "text": "Welcome to the qql-go documentation. QQL is a SQL-like query language for Qdrant vector databases. It supports dense, sparse, and hybrid search.", "org": "public", "team": "all", "access": "public", "topic": "documentation", "doc_type": "guide"},
    {"id": 9002, "text": "qql-go gateway adds JWT authentication, policy enforcement, and tenant isolation on top of Qdrant. Any IdP works via JWKS. Policies are YAML.", "org": "public", "team": "all", "access": "public", "topic": "documentation", "doc_type": "guide"},
    {"id": 9003, "text": "QQL supports score boosting with BOOST formulas: arithmetic, math functions, geo-distance, decay functions, and CASE WHEN conditionals.", "org": "public", "team": "all", "access": "public", "topic": "documentation", "doc_type": "guide"},
    {"id": 9004, "text": "Vector databases store high-dimensional vectors and support similarity search. They are used for semantic search, recommendations, and RAG pipelines.", "org": "public", "team": "all", "access": "public", "topic": "education", "doc_type": "article"},
    {"id": 9005, "text": "Retrieval-Augmented Generation (RAG) combines vector search with LLMs. The retriever finds relevant documents, the generator produces answers grounded in those documents.", "org": "public", "team": "all", "access": "public", "topic": "education", "doc_type": "article"},
]


def build_create_qql() -> list[str]:
    """Generate CREATE COLLECTION + indexes as individual statements."""
    return [
        "CREATE COLLECTION docs HYBRID",
        "CREATE INDEX ON COLLECTION docs FOR org TYPE keyword",
        "CREATE INDEX ON COLLECTION docs FOR team TYPE keyword",
        "CREATE INDEX ON COLLECTION docs FOR access TYPE keyword",
        "CREATE INDEX ON COLLECTION docs FOR topic TYPE keyword",
        "CREATE INDEX ON COLLECTION docs FOR doc_type TYPE keyword",
    ]


def build_insert_qql(batch: list[dict]) -> str:
    """Generate a bulk INSERT statement for a batch of documents."""
    values = []
    for doc in batch:
        # Escape single quotes and backslashes in text
        text = doc["text"].replace("\\", "\\\\").replace("'", "\\'")
        vals = []
        for k, v in doc.items():
            if k == "text":
                continue
            if isinstance(v, str):
                vals.append(f"'{k}': '{v}'")
            else:
                vals.append(f"'{k}': {v}")
        values.append(f"  {{'id': {doc['id']}, 'text': '{text}', {', '.join(vals)}}}")

    return "INSERT INTO docs VALUES\n" + ",\n".join(values) + "\nUSING HYBRID\n"


def seed_via_gateway(gw_url: str, auth_url: str) -> None:
    """Seed data by sending QQL through the gateway's ExecBatch RPC."""
    console.rule("[bold]Seeding via Gateway ExecBatch[/bold]")
    console.print(f"[cyan]gateway:[/cyan] {gw_url}")

    # Get a token from the auth server (admin user).
    token = _get_admin_token(auth_url)
    console.print("[green]✓[/green] authenticated as admin")

    # Step 1: Create collection + indexes
    console.print("[cyan]creating collection + indexes...[/cyan]")
    create_stmts = build_create_qql()
    _exec_batch(gw_url, token, create_stmts)
    console.print("[green]✓[/green] collection + indexes created")

    # Step 2: Insert documents in batches of 10
    batch_size = 10
    total = len(DOCUMENTS)
    for i in range(0, total, batch_size):
        batch = DOCUMENTS[i : i + batch_size]
        qql = build_insert_qql(batch)
        _exec_batch(gw_url, token, [qql])
        console.print(f"[green]✓[/green] inserted {i + len(batch)}/{total}")

    console.print()
    console.print(f"[bold green]Done.[/bold green] {total} documents in collection [bold]docs[/bold]")


def _get_admin_token(auth_url: str) -> str:
    """Login as superadmin (god mode — no tenant scoping)."""
    body = json.dumps({"email": "admin@qql-go.io", "password": "admin123"}).encode()
    req = urllib.request.Request(
        f"{auth_url}/auth/login",
        data=body,
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read())["token"]
    except Exception as e:
        console.print(f"[red]✗[/red] auth login failed: {e}")
        sys.exit(1)


def _exec_batch(gw_url: str, token: str, queries: list[str]) -> dict:
    """Send queries to the gateway via ExecBatch Connect RPC."""
    body = json.dumps({
        "queries": [{"query": q.strip()} for q in queries if q.strip()],
        "stop_on_error": True,
    }).encode()
    req = urllib.request.Request(
        f"{gw_url}/qql.QQL/ExecBatch",
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        err_body = e.read().decode()
        try:
            err_json = json.loads(err_body)
            console.print(f"[red]✗[/red] ExecBatch {e.code}: {err_json.get('message', err_body)}")
        except Exception:
            console.print(f"[red]✗[/red] ExecBatch {e.code}: {err_body}")
        sys.exit(1)
    except Exception as e:
        console.print(f"[red]✗[/red] ExecBatch failed: {e}")
        sys.exit(1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Seed Qdrant via gateway ExecBatch")
    parser.add_argument("--gateway", default="http://localhost:50051",
                        help="Gateway URL (default: http://localhost:50051)")
    parser.add_argument("--auth", default="http://127.0.0.1:8081",
                        help="Auth server URL (default: http://127.0.0.1:8081)")
    args = parser.parse_args()
    seed_via_gateway(args.gateway, args.auth)

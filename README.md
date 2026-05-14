# qql-go - SQL-Like CLI for Qdrant Vector Databases

`qql-go` is an open-source CLI for [Qdrant](https://qdrant.tech) that gives vector search a SQL-like interface. It is built for humans who want a readable terminal workflow and for agents that need stable JSON output.

It is an independent Go port of the original [pavanjava/qql](https://github.com/pavanjava/qql), with a cleaner CLI surface, structured machine output, install scripts, release assets, and agent-oriented usage patterns.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev/)
[![Version](https://img.shields.io/badge/Version-0.1.4-blue.svg)](VERSION)
[![Platforms](https://img.shields.io/badge/Platforms-Windows%20%7C%20Linux%20%7C%20macOS-blue.svg)](https://github.com/srimon12/qql-go/releases)

`qql-go` supports:

- collection management
- collection quantization
- payload index creation
- document insertion
- point retrieval and scroll pagination
- dense, sparse, and hybrid retrieval
- recommend by example IDs
- rerank retrieval
- explain plans
- script execution and collection dump
- filter-based delete

It is designed for both:

- humans using a readable terminal CLI
- agents and scripts using stable JSON output

## Why qql-go?

- Use it as a **Qdrant CLI** for day-to-day collection and retrieval workflows.
- Use it as a **Qdrant SQL-like interface** when you want repeatable commands instead of ad-hoc API calls.
- Use it for **agent automation** when you need structured output that is easy to parse.
- Use it for **local scripts and demos** when you want a small, explicit command surface.

## Installation

Install the latest release on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/srimon12/qql-go/main/install.sh | sh
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/srimon12/qql-go/main/install.sh | VERSION=v0.1.4 sh
```

The Unix installer defaults to `~/.local/bin/qql-go`. Override with `INSTALL_DIR=/your/bin/path` when needed.

Install on Windows with PowerShell:

```powershell
irm https://raw.githubusercontent.com/srimon12/qql-go/main/install.ps1 | iex
```
install the bundled QQL skill for agent use:

```bash
npx skills add srimon12/qql-go --skill qql-skill
```

### Build from source

```bash
git clone https://github.com/srimon12/qql-go.git
cd qql-go
go build -o qql-go ./cmd/qql-go
```

## Quick Start

Connect to Qdrant Cloud for text insert and text search:

```bash
qql-go connect --url https://<your-cluster>.qdrant.io --secret <your-api-key>
```

Or connect to a local/self-hosted Qdrant instance for non-inference operations:

```bash
qql-go connect --url http://localhost:6334
```

Or connect in external/local inference mode with an OpenAI-compatible embeddings API:

```bash
qql-go connect \
  --url http://localhost:6334 \
  --inference-mode external \
  --embedding-endpoint https://api.openai.com/v1/embeddings \
  --embedding-key <embedding-api-key> \
  --embedding-model text-embedding-3-small
```

Run a simple query:

```bash
qql-go exec "SHOW COLLECTIONS"
qql-go exec "SHOW COLLECTION docs"
```

Explain a query without executing it:

```bash
qql-go explain "SEARCH docs SIMILAR TO 'vector db' LIMIT 5 USING HYBRID"
```

Check saved connection health:

```bash
qql-go doctor
```

## CLI Usage

Use the CLI directly:

```bash
qql-go exec "CREATE COLLECTION docs HYBRID"
qql-go exec "CREATE COLLECTION docs HYBRID QUANTIZE TURBO BITS 2 ALWAYS RAM"
qql-go exec "INSERT INTO COLLECTION docs VALUES {'text': 'Qdrant stores vectors', 'topic': 'search'} USING HYBRID"
qql-go exec "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID"
qql-go exec "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID FUSION 'dbsf'"
qql-go exec "SEARCH docs SIMILAR TO 'bm25 keyword' LIMIT 5 USING SPARSE"
qql-go exec "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID RERANK"
qql-go exec "SHOW COLLECTION docs"
qql-go exec "SELECT * FROM docs WHERE id = 'pt-1'"
qql-go exec "SCROLL FROM docs LIMIT 10"
```

Start the interactive shell:

```bash
qql-go
```

or:

```bash
qql-go repl
```

REPL shortcuts:

- `help`, `?`, `\h`
- `explain <query>`
- `exit`, `quit`, `\q`, `:q`

## Agent & Scripting Mode

For automation, do not parse human prose output.

Use compact structured output:

```bash
qql-go exec --quiet --json "<query>"
qql-go explain --quiet --json "<query>"
qql-go doctor --quiet --json
qql-go connect --quiet --json --url <url> [--secret <secret>]
qql-go disconnect --quiet --json
qql-go version --quiet --json
```

Recommended agent examples:

```bash
qql-go exec --quiet --json "SHOW COLLECTIONS"
qql-go exec --quiet --json "SHOW COLLECTION docs"
qql-go explain --quiet --json "SEARCH docs SIMILAR TO 'vector db' LIMIT 5 USING HYBRID"
qql-go doctor --quiet --json
```

Output contract notes:

- `--json` enables structured output
- `--quiet --json` emits compact JSON
- `qql-go explain --quiet "<query>"` prints raw plan text
- `qql-go connect --json` and `qql-go connect --quiet` do not enter REPL
- `qql-go version` prints the current version

## Use Cases

- Qdrant collection management
- Hybrid search and rerank exploration
- Agent-driven retrieval workflows
- Scripted demos and reproducible queries

## Known Limitations

Important behavior in the current Go build:

- cloud mode uses Qdrant Cloud inference for text `INSERT` and text `SEARCH ... SIMILAR TO ...`
- local and external modes generate dense and sparse vectors client-side through an OpenAI-compatible embeddings API
- use `--embedding-key` when the embedding provider requires bearer auth
- `RERANK` is still cloud-only in this build
- `SELECT`, `SCROLL`, `DELETE`, and `RECOMMEND` operate on stored vectors and work in every inference mode
- self-hosted/local Qdrant works well for management operations such as `SHOW`, `CREATE`, `DROP`, `CREATE INDEX`, `SELECT`, `SCROLL`, and `DELETE`
- collections auto-create on insert when missing
- `text` is required in `INSERT ... VALUES {...}`
- keys in `VALUES {...}` may be bare identifiers or quoted strings; quote them when they contain spaces or punctuation

Put differently:

- use Qdrant Cloud if you want the hosted inference path or rerank
- use local/external mode if you want client-side embeddings against any Qdrant instance
- use local/self-hosted freely for management operations even without inference

## Supported Statements

Supported statements:

```sql
CREATE COLLECTION <name>
CREATE COLLECTION <name> HYBRID
CREATE COLLECTION <name> HYBRID RERANK
CREATE COLLECTION <name> USING MODEL '<model>'
CREATE COLLECTION <name> QUANTIZE SCALAR
CREATE COLLECTION <name> QUANTIZE SCALAR QUANTILE <0.0-1.0>
CREATE COLLECTION <name> QUANTIZE SCALAR QUANTILE <0.0-1.0> ALWAYS RAM
CREATE COLLECTION <name> QUANTIZE BINARY
CREATE COLLECTION <name> QUANTIZE BINARY ALWAYS RAM
CREATE COLLECTION <name> QUANTIZE PRODUCT
CREATE COLLECTION <name> QUANTIZE PRODUCT ALWAYS RAM
CREATE COLLECTION <name> QUANTIZE TURBO
CREATE COLLECTION <name> QUANTIZE TURBO BITS <1|1.5|2|4>
CREATE COLLECTION <name> QUANTIZE TURBO BITS <1|1.5|2|4> ALWAYS RAM
DROP COLLECTION <name>
SHOW COLLECTIONS
SHOW COLLECTION <name>

CREATE INDEX ON COLLECTION <name> FOR <field> TYPE keyword
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE integer
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE float
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE bool

INSERT INTO COLLECTION <name> VALUES {...}
INSERT INTO COLLECTION <name> VALUES {...} USING MODEL '<model>'
INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID
INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'
INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID SPARSE MODEL '<model>'
INSERT BULK INTO COLLECTION <name> VALUES [{...}, {...}]
INSERT BULK INTO COLLECTION <name> VALUES [{...}, {...}] USING HYBRID

SEARCH <name> SIMILAR TO '<query>' LIMIT <n>
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING MODEL '<model>'
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID FUSION 'rrf|dbsf'
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE MODEL '<model>'
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef: <n>, exact: true|false, acorn: true|false }
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK MODEL '<model>'
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING SPARSE RERANK

SELECT * FROM <name> WHERE id = '<uuid>'
SELECT * FROM <name> WHERE id = <integer>

SCROLL FROM <name> LIMIT <n>
SCROLL FROM <name> WHERE <filter> LIMIT <n>
SCROLL FROM <name> AFTER '<point_id>' LIMIT <n>
SCROLL FROM <name> WHERE <filter> AFTER <point_id> LIMIT <n>

RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LIMIT <n>
RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) NEGATIVE IDS (<id>, ...) LIMIT <n>
RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) STRATEGY '<strategy>' LIMIT <n>
RECOMMEND FROM <name> POSITIVE IDS (<id>, ...) LOOKUP FROM <collection> [VECTOR '<name>'] USING '<vector_name>' LIMIT <n>

DELETE FROM <name> WHERE id = '<uuid>'
DELETE FROM <name> WHERE id = <integer>
DELETE FROM <name> WHERE <field> = '<value>'

qql-go execute <script.qql>
qql-go execute --stop-on-error <script.qql>
qql-go dump <collection> <output.qql>
qql-go dump --batch-size <n> <collection> <output.qql>

`qql-go explain <statement>`
```

## Retrieval Modes

Dense search:

- use plain `SEARCH`
- use `USING MODEL '<model>'` when you want to pin the dense model
- default dense model: `sentence-transformers/all-minilm-l6-v2`

Collection quantization:

- use `QUANTIZE SCALAR` for the default `int8` compression path
- use `QUANTIZE SCALAR QUANTILE <0..1>` when you want explicit scalar calibration
- use `QUANTIZE BINARY` for more aggressive compression
- use `QUANTIZE PRODUCT` for fixed `x4` product quantization
- use `QUANTIZE TURBO [BITS <1|1.5|2|4>]` for stronger compression with better recall than binary
- add `ALWAYS RAM` when quantized vectors should stay pinned in memory

Hybrid search:

- use `USING HYBRID` when exact terms and semantic similarity both matter; this uses `RRF` by default
- use `FUSION 'dbsf'` when you want the DBSF hybrid fusion strategy instead of the default RRF
- default sparse model: `qdrant/bm25`

Sparse-only search:

- use `USING SPARSE` when the request is primarily keyword or BM25 retrieval
- use `USING SPARSE RERANK` when sparse recall is good but top ordering needs cloud rerank

Point access:

- use `SELECT` when you already know the exact point ID
- use `SCROLL` when you need pagination or to browse points page by page

Rerank:

- use `RERANK` when retrieval is okay but ordering needs improvement
- default rerank model: `answerdotai/answerai-colbert-small-v1`
- requires a collection created with `HYBRID RERANK`

Search-time tuning:

- `EXACT`
- `WITH { hnsw_ef: <n> }`
- `WITH { exact: true|false }`
- `WITH { acorn: true|false }`

## Filter Syntax

Supported predicates:

- `=` `!=` `>` `>=` `<` `<=`
- `BETWEEN ... AND ...`
- `IN (...)` `NOT IN (...)`
- `IS NULL` `IS NOT NULL`
- `IS EMPTY` `IS NOT EMPTY`
- `MATCH` `MATCH ANY` `MATCH PHRASE`

Supported logical operators:

- `AND`
- `OR`
- `NOT`
- parentheses

Examples:

```sql
SEARCH articles SIMILAR TO 'deep learning' LIMIT 10 WHERE year >= 2020
SEARCH articles SIMILAR TO 'retrieval' LIMIT 10 WHERE status IN ('published', 'reviewed')
SEARCH articles SIMILAR TO 'search' LIMIT 10 WHERE title MATCH PHRASE 'semantic search'
SEARCH docs SIMILAR TO 'incident' LIMIT 10 WHERE (team = 'search' OR team = 'infra') AND severity >= 3
```

If you filter heavily, create payload indexes first.

## Release Notes

- [CHANGELOG.md](CHANGELOG.md) for the latest user-facing changes

## Demo Scripts

The repo includes demos under `skills/qql-skill/scripts` that shell out to the Go binary.

```bash
uv run python skills/qql-skill/scripts/demo_medical_records.py --execute
uv run python skills/qql-skill/scripts/demo_kitchen_sink.py --execute
uv run python skills/qql-skill/scripts/demo_retrieval_modes.py --json
```

The helper `skills/qql-skill/scripts/_qql_cli.py` uses:

```text
qql-go exec --quiet --json ...
```

so demos consume structured output instead of scraping prose.

## Skills

This repo publishes installable skills from `skills/`.

List available skills:

```bash
npx skills add srimon12/qql-go --list
```

Install the bundled QQL skill:

```bash
npx skills add srimon12/qql-go --skill qql-skill
```

Install from the GitHub URL form:

```bash
npx skills add https://github.com/srimon12/qql-go --skill qql-skill
```

Validate the local layout before publishing:

```bash
npx skills add . --list
```

More skill authoring notes live in [docs/SKILLS.md](docs/SKILLS.md).

## Configuration

Prebuilt binaries are published on [GitHub Releases](https://github.com/srimon12/qql-go/releases) for:

- Windows
- Linux
- macOS

The repo also ships direct install scripts:

- [install.sh](install.sh)
- [install.ps1](install.ps1)

Config is stored at:

```text
~/.qql/config.json
```

## Contributing And Maintenance

- [CONTRIBUTING.md](CONTRIBUTING.md) for contributors
- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for maintainers and release workflow details
- [CHANGELOG.md](CHANGELOG.md) for user-facing changes
- [docs/releases/0.1.4.md](docs/releases/0.1.4.md) for the current release note

## Project Layout

```text
qql-go/
├── cmd/qql-go/
├── internal/ast/
├── internal/cli/
├── internal/config/
├── internal/errors/
├── internal/filters/
├── internal/lexer/
├── internal/output/
├── internal/parser/
├── internal/repl/
├── skills/qql-skill/
├── install.sh
├── install.ps1
└── README.md
```

## Local Verification

```bash
go test ./...
go build ./cmd/qql-go
```

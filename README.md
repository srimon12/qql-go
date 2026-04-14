# QQL-Go

QQL-Go is a standalone, compiled CLI for [Qdrant](https://qdrant.tech). It gives you a SQL-like surface for collection management, insert, search, filtering, explain plans, and delete operations.

The current build is designed for two workflows:

- human CLI usage with readable text output
- agent/script usage with structured JSON output

```text
qql exec "CREATE COLLECTION docs HYBRID"
qql exec "INSERT INTO COLLECTION docs VALUES {'text': 'Qdrant stores vectors', 'topic': 'search'} USING HYBRID"
qql exec "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID"
qql exec "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID RERANK"
```

## Table of Contents

- [Quick Start](#quick-start)
- [Connection and Health Commands](#connection-and-health-commands)
- [Output Modes (Human vs Machine)](#output-modes-human-vs-machine)
- [Inference Compatibility (Important)](#inference-compatibility-important)
- [Supported QQL Statements](#supported-qql-statements)
- [Search Modes](#search-modes)
- [Query-Time Search Params](#query-time-search-params)
- [Where Filters](#where-filters)
- [REPL Behavior](#repl-behavior)
- [Script and Agent Usage](#script-and-agent-usage)
- [Demo Scripts](#demo-scripts)
- [Configuration File](#configuration-file)
- [Project Layout](#project-layout)
- [Testing](#testing)

## Quick Start

### Build

```bash
go build -o qql.exe ./cmd/qql
```

On non-Windows platforms:

```bash
go build -o qql ./cmd/qql
```

### Connect to Qdrant

For text `INSERT` and text `SEARCH` in the current Go build, connect to Qdrant Cloud.

```bash
qql connect --url https://<your-cluster>.qdrant.io --secret <your-api-key>
```

Self-hosted/local URLs are supported for non-inference operations such as collection and index management.

```bash
qql connect --url http://localhost:6333
```

By default, successful `connect` enters the interactive REPL.

### Run one query

```bash
qql exec "SHOW COLLECTIONS"
```

### Explain a query plan

```bash
qql explain "SEARCH docs SIMILAR TO 'vector db' LIMIT 5 USING HYBRID"
```

### Check connection health

```bash
qql doctor
```

## Connection and Health Commands

### connect

```bash
qql connect --url <url> [--secret <secret>]
```

- validates connectivity
- saves config to `~/.qql/config.json`
- enters REPL on success (default behavior)

### disconnect

```bash
qql disconnect
```

- removes saved config

### doctor

```bash
qql doctor
```

- checks saved connection
- reports status and collection count

## Output Modes (Human vs Machine)

QQL-Go supports a consistent output contract across `exec`, `explain`, `doctor`, and `connect`.

### Human-readable defaults

```bash
qql exec "<query>"
qql explain "<query>"
qql doctor
qql connect --url <url> [--secret <secret>]
```

### Structured JSON

```bash
qql exec --json "<query>"
qql explain --json "<query>"
qql doctor --json
qql connect --json --url <url> [--secret <secret>]
```

### Compact JSON (agent path)

```bash
qql exec --quiet --json "<query>"
qql explain --quiet --json "<query>"
qql doctor --quiet --json
qql connect --quiet --json --url <url> [--secret <secret>]
```

Notes:

- `--quiet --json` emits compact JSON (no pretty indentation).
- `qql explain --quiet "<query>"` prints the raw plan without the titled section wrapper.
- `qql connect --json` and `qql connect --quiet` do not drop into REPL.

## Inference Compatibility (Important)

Current Go behavior:

- Text `INSERT` and text `SEARCH ... SIMILAR TO ...` use Qdrant server-side document inference.
- `USING HYBRID` and `RERANK` are Qdrant Cloud inference paths in this build.
- Self-hosted/local Qdrant is currently best for non-inference operations (`SHOW`, `CREATE`, `DROP`, `CREATE INDEX`, `DELETE`).

Planned later:

- local/external dense+sparse generation so self-hosted hybrid retrieval works end-to-end without cloud inference.

## Supported QQL Statements

### Collection management

```sql
CREATE COLLECTION <name>
CREATE COLLECTION <name> HYBRID
CREATE COLLECTION <name> HYBRID RERANK
DROP COLLECTION <name>
SHOW COLLECTIONS
```

### Payload indexes

```sql
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE keyword
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE integer
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE float
CREATE INDEX ON COLLECTION <name> FOR <field> TYPE bool
```

### Insert

```sql
INSERT INTO COLLECTION <name> VALUES {...}
INSERT INTO COLLECTION <name> VALUES {...} USING MODEL '<model>'
INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID
INSERT INTO COLLECTION <name> VALUES {...} USING HYBRID DENSE MODEL '<model>' SPARSE MODEL '<model>'
```

Important:

- `text` field is required in `VALUES`.
- Collection must already exist; QQL-Go does not auto-create on insert.
- In the current Go build, text insert embedding is a cloud inference path.

### Search

```sql
SEARCH <name> SIMILAR TO '<query>' LIMIT <n>
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WHERE <filter>
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> EXACT
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> WITH { hnsw_ef: <n>, exact: true|false, acorn: true|false }
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> RERANK
SEARCH <name> SIMILAR TO '<query>' LIMIT <n> USING HYBRID RERANK
```

### Delete

```sql
DELETE FROM <name> WHERE id = '<uuid>'
DELETE FROM <name> WHERE id = <integer>
DELETE FROM <name> WHERE <field> = '<value>'
```

Field delete is equality-based filter delete.

### Explain

```sql
EXPLAIN <statement>
```

## Search Modes

### Dense

Use plain `SEARCH` for semantic retrieval.

Default dense model:

- `sentence-transformers/all-minilm-l6-v2`

### Hybrid

Use `USING HYBRID` when exact term matching and semantic matching both matter.

Default sparse model:

- `qdrant/bm25`

### Rerank

Use `RERANK` when candidate ordering needs improvement.

Default rerank model:

- `answerdotai/answerai-colbert-small-v1`

Rerank notes:

- relies on Qdrant Cloud inference path
- requires a collection created with `HYBRID RERANK`
- is slower than plain dense/hybrid retrieval

## Query-Time Search Params

Supported search-time options:

- `EXACT`
- `WITH { hnsw_ef: <n> }`
- `WITH { exact: true|false }`
- `WITH { acorn: true|false }`

Examples:

```sql
SEARCH docs SIMILAR TO 'retrieval' LIMIT 10 EXACT
SEARCH docs SIMILAR TO 'retrieval' LIMIT 10 WITH { hnsw_ef: 256 }
SEARCH docs SIMILAR TO 'retrieval' LIMIT 10 WHERE tag = 'search' WITH { acorn: true }
```

## Where Filters

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

## REPL Behavior

Run shell:

```bash
qql
```

or:

```bash
qql repl
```

Inside REPL:

- `help`, `?`, `\h` prints help
- `explain <query>` prints plan
- `exit`, `quit`, `\q`, `:q` exits

Multiline input is supported while brackets, braces, or parentheses remain open.

## Script and Agent Usage

Recommended command style:

- humans: `qql exec "<query>"`
- scripts/agents: `qql exec --quiet --json "<query>"`

Examples:

```powershell
qql exec --quiet --json "SHOW COLLECTIONS"
qql explain --quiet --json "SEARCH docs SIMILAR TO 'vector db' LIMIT 5 USING HYBRID"
qql doctor --quiet --json
qql connect --quiet --json --url https://<cluster>.qdrant.io --secret <api-key>
```

For text `INSERT`/`SEARCH`, use a cloud connection URL in the current build.

## Demo Scripts

The repo includes demos under `skills/qql-skill/scripts` that shell out to the Go binary.

```bash
python skills/qql-skill/scripts/demo_medical_records.py --execute
python skills/qql-skill/scripts/demo_kitchen_sink.py --execute
python skills/qql-skill/scripts/demo_retrieval_modes.py --json
```

The helper `skills/qql-skill/scripts/_qql_cli.py` uses:

```text
qql exec --quiet --json ...
```

so demos consume structured output instead of scraping prose.

## Configuration File

Saved at:

```text
~/.qql/config.json
```

Current fields:

- `url`
- `secret`
- `active_profile`
- `inference_model`

## Project Layout

```text
qql-go/
├── cmd/qql/
├── internal/ast/
├── internal/cli/
├── internal/config/
├── internal/errors/
├── internal/filters/
├── internal/lexer/
├── internal/output/
├── internal/parser/
├── internal/repl/
└── README.md
```

## Testing

```bash
go test ./...
```

The tests cover lexer, parser, filters, command request-building, config behavior, output formatting, and REPL command handling.

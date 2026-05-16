# Retrieval Debug Runbook

Use this example when someone says search is behaving wrong and you need to investigate quickly from a terminal.

Example complaint:

> billing policy search stopped returning the right docs after a retrieval change

This runbook checks:

- connection health
- collection schema and payload indexes
- hybrid vs exact vs sparse results for the same query
- filtered search behavior
- the expected document by ID
- the related runbook documents

## Run it

```bash
bash examples/retrieval-debug-runbook/run-demo.sh
```

## Files

- `01-seed.qql`
  Creates the sample collection used by this example.
- `run-demo.sh`
  Runs the investigation and saves JSON artifacts.
- `run-demo.ps1`
  PowerShell version.
- `validate-artifacts.sh`
  Checks that the expected retrieval behavior is still present.
- `validate-artifacts.ps1`
  PowerShell version.
- `agent-playbook.md`
  Short command list for an agent to follow.

## Steps

1. Create `retrieval_debug_runbook`
2. Inspect the collection
3. Explain the hybrid query plan
4. Run the same query in hybrid, exact, and sparse modes
5. Rerun the query with a billing filter
6. Inspect the expected document by ID
7. Scroll the runbook documents

## Artifacts

- `artifacts/01-doctor.json`
- `artifacts/02-seed.json`
- `artifacts/03-inspect.json`
- `artifacts/04-explain.json`
- `artifacts/05-search-hybrid.json`
- `artifacts/06-search-exact.json`
- `artifacts/07-search-sparse.json`
- `artifacts/08-search-filtered.json`
- `artifacts/09-select-doc.json`
- `artifacts/10-scroll-runbooks.json`

## What to look for

- `03-inspect.json`
  Confirm the collection is hybrid and the payload indexes exist.
- `04-explain.json`
  Confirm the query is taking the intended retrieval path.
- `05-search-hybrid.json`, `06-search-exact.json`, `07-search-sparse.json`
  Compare which document ranks first in each mode.
- `08-search-filtered.json`
  Check whether the filter changes the outcome.
- `09-select-doc.json`
  Verify the expected document is present and has the right payload.

For an agent-driven version of the same workflow, use [agent-playbook.md](agent-playbook.md).

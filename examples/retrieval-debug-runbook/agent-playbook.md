# Agent Playbook

Use this when an agent needs to investigate a retrieval complaint against the runbook collection.

Symptom example:

> billing policy search stopped returning the right docs after a retrieval change

## Command order

Run these commands in order and inspect the JSON after each step:

```bash
qql-go doctor --quiet --json
qql-go exec --quiet --json "SHOW COLLECTION retrieval_debug_runbook"
qql-go explain --quiet --json "SEARCH retrieval_debug_runbook SIMILAR TO 'billing policy search regression after index removal' LIMIT 3 USING HYBRID"
qql-go exec --quiet --json "SEARCH retrieval_debug_runbook SIMILAR TO 'billing policy search regression after index removal' LIMIT 3 USING HYBRID"
qql-go exec --quiet --json "SEARCH retrieval_debug_runbook SIMILAR TO 'billing policy search regression after index removal' LIMIT 3 EXACT"
qql-go exec --quiet --json "SEARCH retrieval_debug_runbook SIMILAR TO 'billing policy search regression after index removal' LIMIT 3 USING SPARSE"
qql-go exec --quiet --json "SEARCH retrieval_debug_runbook SIMILAR TO 'billing policy search regression after index removal' LIMIT 3 USING HYBRID WHERE team = 'billing'"
qql-go exec --quiet --json "SELECT * FROM retrieval_debug_runbook WHERE id = 4104"
qql-go exec --quiet --json "SCROLL FROM retrieval_debug_runbook WHERE doc_type = 'runbook' LIMIT 10"
```

## Report format

Return:

- whether the collection is healthy
- whether the payload indexes needed for filtering exist
- which document ranked first in hybrid, exact, and sparse
- whether the billing filter changed the result
- whether document `4104` exists and matches the complaint
- the likely cause
- the next operator action

## Scope

Stay on read and investigation commands first.

If you need broader syntax than this playbook, use the repo skill at `skills/qql-skill/SKILL.md`.

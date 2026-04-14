# Red Team Review: qql-go

## Scope and assumptions

- Scope reviewed: Go CLI/runtime code, tests, README, and `.agents/skills/qql-skill`.
- No code was changed as part of this pass.
- Analysis is based on the current checkout on April 14, 2026.
- Existing tests currently pass with `go test ./...`.
- Coverage is uneven: `internal/cli/commands` is `30.1%`, `internal/config` is `38.8%`, `internal/repl` is `51.0%`, while lexer/output are much higher.

## Executive summary

The repo is compact enough to reason about, but it already shows the shape of code that has grown by accretion rather than by a stable design loop.

The biggest problems are not in the lexer. They are in the application layer:

- too much responsibility collapsed into `internal/cli/commands/commands.go`
- control flow that still uses `os.Exit` deep inside command handlers
- duplicated parser branches for optional syntax
- stale or duplicated documentation across README and skill files
- tests that are green, but avoid the riskiest branches

This is fixable without a rewrite. The code can be materially reduced while improving stability if the next cleanup pass is surgical and centered on removing duplication, not adding abstraction.

## Highest-risk findings

### 1. Process exits are embedded inside command logic

Files:

- `internal/cli/cli.go`
- `internal/cli/commands/commands.go`

Why this matters:

- `RunE` handlers and helpers call `os.Exit` directly instead of returning errors.
- That makes the CLI harder to compose, harder to test, and more brittle around cleanup.
- It also explains why the tests mostly stay on happy-path helper functions instead of real command execution paths.

Minimize/improve:

- Move all process termination to `cmd/qql/main.go`.
- Let every command return an error and let one top-level place decide exit code and output mode.
- This will shrink command branching and unlock cleaner tests.

### 2. `internal/cli/commands/commands.go` is doing too much

File:

- `internal/cli/commands/commands.go`

Why this matters:

- At ~1088 lines, this file mixes parsing dispatch, Qdrant client config, command wiring, JSON/text output behavior, request builders, result formatting, collection polling, and business rules.
- That size by itself is not the issue. The issue is mixed responsibilities with duplicated logic and many unrelated helpers.
- This is the repo’s clearest “patchy growth” hotspot.

Minimize/improve:

- Split by responsibility, not by pattern:
  - command wiring
  - executor/query operations
  - Qdrant request builders
  - response/output structs
- Do not add interfaces everywhere. Just separate independent chunks that already exist.

### 3. Rerank search can multiply backend work without limits

File:

- `internal/cli/commands/commands.go`

Why this matters:

- `LIMIT` is multiplied by 4 for rerank fetch size with no cap or sanity check.
- A user-controlled limit can inflate query cost and memory use quickly.
- This is a real stability risk, not a style nit.

Minimize/improve:

- Clamp rerank prefetch to a reasonable upper bound.
- Make the multiplier explicit and named.
- Add tests around large limits and zero/edge behavior.

### 4. Interactive REPL has hidden bugs despite green tests

File:

- `internal/repl/repl.go`

Why this matters:

- `handleCommand` detects `explain` case-insensitively, but trims only exact lowercase/uppercase prefixes from the original string. Mixed-case `ExPlAiN ...` leaks the full command into the executor.
- `readLine` decrements delimiter depth below zero on stray closing braces/brackets/parens, which can trap the REPL in continuation mode.
- Current tests do not cover these cases.

Minimize/improve:

- Normalize command slicing once.
- Guard depth from going negative.
- Add small targeted tests instead of more broad integration machinery.

### 5. Numeric parsing is hand-rolled and silent

File:

- `internal/parser/parser.go`

Why this matters:

- `parseInt` and `parseFloat` never return errors.
- They silently accept whatever the lexer shape allowed and have no overflow or validation signaling.
- That is risky for `LIMIT`, numeric filters, and point IDs.

Minimize/improve:

- Replace custom numeric parsing with standard library parsing.
- Return errors instead of mutating via pointer arguments.
- This reduces code and improves correctness at the same time.

## Medium-risk design debt

### 6. Parser optional-clause logic is duplicated

File:

- `internal/parser/parser.go`

Why this matters:

- `USING HYBRID ... DENSE MODEL ... SPARSE MODEL ...` parsing logic is duplicated across `parseInsert` and `parseSearch`.
- Optional search modifiers are handled in a patchy order that is easy to drift.

Minimize/improve:

- Extract one helper for model/hybrid clause parsing.
- Keep syntax order enforcement in one place.
- This is a good reduction candidate because it removes duplication without inventing a framework.

### 7. Filter converter has strong AI-slop signals

File:

- `internal/filters/filter.go`

Why this matters:

- `buildCondition` repeats every AST case twice: value and pointer versions.
- The parser mostly emits pointers already, so much of this surface looks defensive-by-copy rather than intentionally designed.
- That doubles maintenance cost for every new filter node.

Minimize/improve:

- Standardize AST expression shapes and accept one form.
- Collapse duplicated switch branches.
- Keep conversion helpers, but remove unused polymorphism.

### 8. Config layer has dead or underused surface plus mutable globals

File:

- `internal/config/config.go`

Why this matters:

- `cfg` and `profiles` are package globals.
- Profile helpers exist but appear unused outside `internal/config`.
- `DeleteConfig` resets to `&Config{}` while missing config returns `nil`, so absence semantics are inconsistent.

Minimize/improve:

- If profiles are not a real product surface yet, remove them or isolate them until needed.
- Normalize absent-config behavior.
- Reduce package-global mutable state where possible.

### 9. Output layer is only half injectable

File:

- `internal/output/output.go`

Why this matters:

- `PrintError` writes directly to `os.Stderr`, while the rest uses `o.writer`.
- Tests have to patch global stdio to compensate.
- That is small, but it creates friction everywhere.

Minimize/improve:

- Inject stderr the same way stdout is injected, or keep one writer model consistently.
- This is a tiny change with outsized testability benefit.

### 10. Collection readiness wait is brittle and misleading

File:

- `internal/cli/commands/commands.go`

Why this matters:

- The code polls for readiness with a fixed 10 second timeout.
- On timeout it can effectively convert slow propagation into a misleading “does not exist” failure.

Minimize/improve:

- Separate “not ready yet” from “not found”.
- Avoid hardcoding backend timing assumptions into the success path.

## Tests: where green is misleading

### What is good

- Lexer coverage is strong.
- Output tests are direct and useful.
- Parser surface coverage is broader than the rest of the repo.

### What is weak

- `internal/cli/commands` has only `30.1%` coverage.
- Tests focus on helper functions and response encoding, not high-risk command execution paths.
- Root command behavior, `connect`, `doctor`, `exec`, and `os.Exit` branches are mostly untested.
- Some parser precedence tests assert shape and counts, but not full nested leaf correctness.

### Minimum high-value test additions

1. Mixed-case `explain` handling in REPL.
2. Stray closing delimiter behavior in REPL multiline reader.
3. Large-limit rerank clamping behavior.
4. Config malformed-file behavior.
5. Command error returns after removing deep `os.Exit`.
6. Parser precedence tests that assert full leaf ordering, not just node type.

## Docs and skill package review

### 11. Multiple sources of truth are drifting

Files:

- `README.md`
- `.agents/skills/qql-skill/SKILL.md`
- `.agents/skills/qql-skill/references/qql-capabilities.md`
- `.agents/skills/qql-skill/references/qql-query-patterns.md`
- `.agents/skills/qql-skill/references/qql-gaps.md`

Why this matters:

- The same capability boundaries are described in multiple places with different completeness.
- The skill docs currently omit supported syntax variants that README and parser tests show.
- That is dangerous in an AI-facing repo because the skill docs become operational truth for agents.

Minimize/improve:

- Pick one canonical capability source.
- Make skill docs point to that source instead of re-describing everything.
- Keep only agent-specific usage guidance in the skill.

### 12. Tracked generated artifacts add noise

Files:

- `.agents/skills/qql-skill/scripts/__pycache__/*.pyc`

Why this matters:

- These are checked-in bytecode artifacts.
- They are Python-version-specific and provide no review value.
- Their presence is a strong patchiness signal.

Minimize/improve:

- Remove tracked `__pycache__` artifacts.
- Extend `.gitignore` to cover Python bytecode and similar generated files.

### 13. Demo surface is duplicated and partly misleading

Files:

- `.agents/skills/qql-skill/scripts/demo_retrieval_modes.py`
- `.agents/skills/qql-skill/scripts/demo_retrieval_modes/main.go`
- `.agents/skills/qql-skill/scripts/demo_medical_records.py`
- `.agents/skills/qql-skill/scripts/demo_kitchen_sink.py`

Why this matters:

- Similar demos exist in both Python and Go with overlapping teaching value.
- Some scripts swallow setup/cleanup errors, which makes broken runs look healthy.
- One Python flag is effectively a no-op.

Minimize/improve:

- Keep one demo path per teaching goal.
- Do not suppress cleanup/setup errors unless explicitly labeling them as best-effort.
- Prefer fewer demos with stronger contracts.

## Concrete AI-slop / patchy-work signals

These are the strongest indicators that parts of the repo were assembled incrementally without enough consolidation:

- duplicated parser logic for `USING HYBRID` handling
- duplicated pointer/value type switch branches in filter conversion
- hardcoded literals that will drift (`versionString`, `~/.qql/config.json`)
- checked-in `__pycache__` artifacts
- partially injectable output API
- product surface in config (`profiles`) with little evidence of real use
- docs repeated in too many places with inconsistent completeness

None of these require a rewrite. They require deletion, consolidation, and sharper boundaries.

## Recommended minimization plan

### Phase 1: Highest return, lowest risk

1. Remove deep `os.Exit` usage and return errors upward.
2. Cap rerank fetch expansion.
3. Fix REPL mixed-case `explain` and negative-depth multiline handling.
4. Replace hand-rolled numeric parsing with standard library parsing.

Expected result:

- better stability
- simpler command control flow
- easier testing
- less hidden fragility

### Phase 2: Reduce code surface

1. Split `commands.go` by responsibility.
2. Deduplicate parser optional-clause parsing.
3. Collapse filter pointer/value duplication.
4. Normalize config absence semantics and trim dead profile surface if not needed.

Expected result:

- fewer places to change when syntax evolves
- less maintenance drag
- easier reviewability

### Phase 3: Reduce drift outside core code

1. Make one canonical capability document.
2. Shrink skill docs to agent-specific instructions only.
3. Remove tracked generated artifacts.
4. Delete duplicate or weak demos.

Expected result:

- less repo noise
- lower risk of stale guidance
- better signal for both humans and agents

## Final verdict

This repo is not bloated overall, but it is already carrying more accidental complexity than its size justifies.

The fastest wins are not “add architecture.” They are:

- remove duplicated parsing/conversion logic
- stop exiting from deep inside command handlers
- cap risky runtime behavior
- cut duplicate docs and generated artifacts

If that is done carefully, the codebase gets smaller and more stable at the same time.

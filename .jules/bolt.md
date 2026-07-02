## 2024-07-02 - Generated Switch Statements are 5x Faster for Lexer Lookups
**Learning:** In Go, replacing dynamic map lookups with a generated switch block matching length and unrolled case-insensitive byte comparisons provides a 5x speedup and prevents memory allocations, effectively reducing lexer overhead in `qql-go`.
**Action:** Identify dynamic lookups with known static datasets in tight loops and replace them with code-generated statically compiled structures using `go:generate`.

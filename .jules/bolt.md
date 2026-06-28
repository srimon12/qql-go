## 2024-05-24 - Lexer Keyword Optimization
**Learning:** Found that `lookupKeywordFast` is mentioned in memory but not actually implemented in the codebase yet. Code generation for lexer is also missing. The map lookup in `lookupKeyword` allocates memory implicitly when slicing strings to bytes, and map lookup itself isn't the absolute fastest.
**Action:** Need to implement `lookupKeywordFast` using a generated perfect hash or switch statement, and add the `//go:generate` directive to `lexer.go`.

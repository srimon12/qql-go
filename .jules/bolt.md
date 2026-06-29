## 2025-06-29 - O(1) Lexer Keyword Lookup via Code Generation
**Learning:** Using map lookups or reflection for dynamic string keyword matching causes major slowdowns inside the lexer, particularly because Go strings require allocations/manipulations to do case-insensitive comparisons properly.
**Action:** Generate a highly-optimized switch statement without memory allocations for case-insensitive matching in high-throughput parsing environments (like QQL). Use `//go:generate` to write Go code that checks byte characters manually at predetermined lengths.

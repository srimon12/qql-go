## 2024-06-30 - Lexer Memory Allocation Optimization
**Learning:** The lexer `lookupKeyword` function currently avoids allocating strings by converting to uppercase via `[16]byte` and converting to a string during map lookup `string(buf[:len(s)])`. Go's compiler optimizes `map[string(byte_slice)]` to not allocate, which means `lookupKeyword` already has zero allocations! See `BenchmarkLookupKeywordAlloc`.

**Action:** Look for other areas for optimizations, such as generating the keyword map lookup function to use a `switch` statement (perfect hash / hardcoded strings) instead of a map lookup, which is often faster.
## 2024-06-30 - Lexer Keyword Lookup Optimization
**Learning:** The lexer `lookupKeyword` function initially avoided allocating strings by converting to uppercase via `[16]byte` and mapping lookup via `string(buf[:len(s)])`. Go's compiler optimizes `map[string(byte_slice)]` to not allocate. However, benchmarking showed that generating a static, length-based perfect `switch` block (using `go generate`) for keyword string matching is around 2.5x faster (from ~76ns to ~28ns) than performing the map lookup, even without allocations.
**Action:** When a static set of known strings needs to be matched frequently (like lexer keywords), generating a perfect switch statement based on string length is a highly effective optimization compared to a map lookup.

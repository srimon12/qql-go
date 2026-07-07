## 2024-07-07 - Dynamic string allocation in keyword lookup
**Learning:** Checking keywords against a map, especially with dynamic string allocations and length checks dynamically, causes high memory allocations and CPU overhead during query tokenization.
**Action:** Used code generation (`go generate`) to create a fast switch statement based on string length and individual character comparisons (`lookupKeywordFast`). This avoids dynamic map lookups and allocations, significantly speeding up tokenization.

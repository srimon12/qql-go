## 2024-06-20 - Fast Keyword Lookup in Lexer
**Learning:** The QQL parser is extremely sensitive to memory allocations. Dynamic map lookups in a tight lexer loop limit performance.
**Action:** Use code generation (`go generate`) to create length-based byte-comparison switch statements for keyword lookups to avoid allocation and reduce latency.

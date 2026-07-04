## 2024-07-04 - Code Generation for Lexer Keyword Lookups
**Learning:** In Go, replacing map lookups with generated switch statements (`go generate` to produce `lookup_fast.go`) avoids heap allocations entirely and substantially reduces lookup times (e.g. from ~60ns/op to ~13ns/op in benchmarks). The `go generate` directive must be placed in a .go file so the standard toolchain picks it up.
**Action:** When working on lexer/parser performance in Go projects, consider replacing static maps with generated switch blocks or perfect hash functions if allocation overhead or latency is a problem.

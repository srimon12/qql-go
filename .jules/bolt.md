## 2025-02-28 - Avoid WriteString(Sprintf)
**Learning:** In Go, calling `builder.WriteString(fmt.Sprintf(...))` causes unnecessary heap allocations for the intermediate string created by Sprintf.
**Action:** Always replace `builder.WriteString(fmt.Sprintf(...))` with `fmt.Fprintf(&builder, ...)` to write directly into the string builder buffer, significantly improving performance and reducing GC overhead.

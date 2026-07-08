## 2026-07-08 - [Avoid Parsing Upsert Vectors]
**Learning:** During JSON-to-QQL conversion of large upsert payloads, unmarshaling dense vectors into `any` (`[]interface{}`) or `[]float32` causes massive amounts of allocation.
**Action:** Use `json.RawMessage` for vectors in `convertUpsert` so they skip unnecessary intermediate object allocations. The raw JSON buffer can be copied directly to the `strings.Builder` during query generation.

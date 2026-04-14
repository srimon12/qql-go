package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Example struct {
	Mode  string `json:"mode"`
	When  string `json:"when"`
	Query string `json:"query"`
}

var EXAMPLES = []Example{
	{
		Mode:  "dense",
		When:  "Use when semantic similarity matters more than exact term matching.",
		Query: "SEARCH articles SIMILAR TO 'vector database performance tuning' LIMIT 5",
	},
	{
		Mode:  "hybrid",
		When:  "Use when exact terms, acronyms, model names, or error strings matter.",
		Query: "SEARCH incidents SIMILAR TO 'out of memory hnsw_ef acorn' LIMIT 10 USING HYBRID",
	},
	{
		Mode:  "exact",
		When:  "Use when debugging recall and you need an exact KNN baseline.",
		Query: "SEARCH articles SIMILAR TO 'attention mechanism' LIMIT 10 EXACT",
	},
	{
		Mode:  "with-hnsw-ef",
		When:  "Use when you want query-time recall tuning without changing collection config.",
		Query: "SEARCH articles SIMILAR TO 'transformer inference' LIMIT 10 WITH { hnsw_ef: 256 }",
	},
	{
		Mode:  "with-acorn",
		When:  "Use when filtered-query recall is the focus and ACORN should be tested.",
		Query: "SEARCH incidents SIMILAR TO 'retrieval recall regression' LIMIT 10 WHERE team = 'search' WITH { acorn: true }",
	},
	{
		Mode:  "rerank",
		When:  "Use when recall is likely good but top-result ordering needs help.",
		Query: "SEARCH papers SIMILAR TO 'late interaction retrieval' LIMIT 5 RERANK",
	},
	{
		Mode:  "hybrid-rerank",
		When:  "Use when both keyword recall and top-rank precision matter.",
		Query: "SEARCH docs SIMILAR TO 'cross encoder ms marco minimlm' LIMIT 8 USING HYBRID RERANK",
	},
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(EXAMPLES)
		return
	}

	for _, ex := range EXAMPLES {
		fmt.Printf("[%s]\n%s\n%s\n\n", ex.Mode, ex.When, ex.Query)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/srimon12/qql-go/pkg/qql"
)

type payload struct {
	Label string `json:"label"`
	JSON  string `json:"json"`
}

func main() {
	data, err := os.ReadFile("all_payloads.json")
	if err != nil {
		fmt.Println("ERROR reading file:", err)
		return
	}
	var payloads []payload
	if err := json.Unmarshal(data, &payloads); err != nil {
		fmt.Println("ERROR parsing:", err)
		return
	}

	pass, fail := 0, 0
	for _, p := range payloads {
		stmts, err := qql.ConvertJSONToQQL(p.JSON)
		if err != nil {
			fmt.Printf("CONVERT ✗ [%s]: %v\n", p.Label, err)
			fail++
			continue
		}
		for _, s := range stmts {
			_, err := qql.Explain(s)
			if err != nil {
				fmt.Printf("PARSE  ✗ [%s]: %v\n  %s\n", p.Label, err, s)
				fail++
			} else {
				fmt.Printf("OK     ✓ [%s]\n  %s\n", p.Label, s)
				pass++
			}
		}
	}
	fmt.Printf("\n=== %d passed, %d failed out of %d payloads ===\n", pass, fail, len(payloads))
}

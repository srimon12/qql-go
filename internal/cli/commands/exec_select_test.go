package commands

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDoSelectReturnsRecordOrNil(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		client := newFakeQdrantClient()
		client.exists = true
		client.getRecords = []*qdrant.RetrievedPoint{
			{
				Id: qdrant.NewIDUUID("pt-1"),
				Payload: qdrant.NewValueMap(map[string]any{
					"text":  "hello",
					"topic": "search",
				}),
			},
		}
		exec := NewExecutor(client, &config.Config{})

		resp, err := exec.doSelect(&ast.SelectStmt{Collection: "docs", PointID: "pt-1"})
		require.NoError(t, err)
		require.True(t, resp.OK)
		require.Equal(t, map[string]any{
			"id": "pt-1",
			"payload": map[string]any{
				"text":  "hello",
				"topic": "search",
			},
		}, resp.Data)
	})

	t.Run("missing", func(t *testing.T) {
		client := newFakeQdrantClient()
		client.exists = true
		exec := NewExecutor(client, &config.Config{})

		resp, err := exec.doSelect(&ast.SelectStmt{Collection: "docs", PointID: "pt-404"})
		require.NoError(t, err)
		require.True(t, resp.OK)
		require.Nil(t, resp.Data)
	})
}

func TestDoScrollReturnsUpstreamStylePayload(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.scrollRecords = []*qdrant.RetrievedPoint{
		{
			Id: qdrant.NewIDNum(7),
			Payload: qdrant.NewValueMap(map[string]any{
				"text":  "hello",
				"topic": "search",
			}),
		},
	}
	client.scrollOffset = qdrant.NewIDUUID("pt-next")
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doScroll(&ast.ScrollStmt{Collection: "docs", Limit: 5})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, map[string]any{
		"points": []map[string]any{
			{
				"id": "7",
				"payload": map[string]any{
					"text":  "hello",
					"topic": "search",
				},
			},
		},
		"next_offset": "pt-next",
	}, resp.Data)
}

func TestDoScrollPreservesNumericNextOffsetType(t *testing.T) {
	client := newFakeQdrantClient()
	client.exists = true
	client.scrollOffset = qdrant.NewIDNum(42)
	exec := NewExecutor(client, &config.Config{})

	resp, err := exec.doScroll(&ast.ScrollStmt{Collection: "docs", Limit: 5})
	require.NoError(t, err)
	require.True(t, resp.OK)
	require.Equal(t, uint64(42), resp.Data.(map[string]any)["next_offset"])
}

func TestExplainSelectAndScrollQueries(t *testing.T) {
	exec := NewExecutor(nil, &config.Config{})

	t.Run("select", func(t *testing.T) {
		plan, err := exec.Explain(`SELECT * FROM docs WHERE id = 'pt-1'`)
		require.NoError(t, err)
		require.Contains(t, plan, "Statement: SELECT * FROM docs WHERE id = 'pt-1'")
		require.Contains(t, plan, "Retrieve a single point by ID")
	})

	t.Run("scroll basic", func(t *testing.T) {
		plan, err := exec.Explain(`SCROLL FROM docs LIMIT 10`)
		require.NoError(t, err)
		require.Contains(t, plan, "Statement: SCROLL FROM docs LIMIT 10")
		require.Contains(t, plan, "Scroll (paginate) through points")
	})

	t.Run("scroll with filter and after", func(t *testing.T) {
		plan, err := exec.Explain(`SCROLL FROM docs WHERE status = 'active' AFTER 'pt-5' LIMIT 20`)
		require.NoError(t, err)
		require.Contains(t, plan, "Filter:")
		require.Contains(t, plan, "After: pt-5")
	})

	t.Run("turbo quantization explain", func(t *testing.T) {
		plan, err := exec.Explain(`CREATE COLLECTION docs WITH QUANTIZATION (type = 'turbo', bits = 2, always_ram = true)`)
		require.NoError(t, err)
		require.Contains(t, plan, "Quantization: turbo")
		require.Contains(t, plan, "Turbo bits: 2")
		require.Contains(t, plan, "Quantization storage: ALWAYS RAM")
	})
}

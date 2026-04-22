package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbedBatchPostsRequestAndReordersResults(t *testing.T) {
	t.Parallel()

	var got request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/embeddings", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		require.Equal(t, "text-embedding-3-small", got.Model)
		require.Equal(t, []string{"hello", "world"}, got.Input)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response{
			Data: []responseItem{
				{Index: 1, Embedding: []float32{4, 5, 6}},
				{Index: 0, Embedding: []float32{1, 2, 3}},
			},
		}))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint:   server.URL + "/v1/embeddings",
		Model:      "text-embedding-3-small",
		APIKey:     "test-key",
		Dimension:  3,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	vectors, err := client.EmbedBatch(context.Background(), []string{"hello", "world"})
	require.NoError(t, err)
	require.Equal(t, [][]float32{{1, 2, 3}, {4, 5, 6}}, vectors)
}

func TestEmbedRejectsDimensionMismatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response{
			Data: []responseItem{
				{Index: 0, Embedding: []float32{1, 2}},
			},
		}))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint:   server.URL + "/v1/embeddings",
		Model:      "text-embedding-3-small",
		Dimension:  3,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	_, err = client.Embed(context.Background(), "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dimension mismatch")
}

package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Config struct {
	Endpoint   string
	Model      string
	APIKey     string
	Dimension  int
	HTTPClient *http.Client
}

type Client struct {
	endpoint   string
	model      string
	apiKey     string
	dimension  int
	httpClient *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("dimension must be positive")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		endpoint:   cfg.Endpoint,
		model:      cfg.Model,
		apiKey:     cfg.APIKey,
		dimension:  cfg.Dimension,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Embed(ctx context.Context, input string) ([]float32, error) {
	vectors, err := c.EmbedBatch(ctx, []string{input})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

// ProbeDimension calls the embedding endpoint with a dummy input and returns
// the dimension of the returned vector without validating against a target.
func (c *Client) ProbeDimension(ctx context.Context, input string) (int, error) {
	body, err := json.Marshal(request{
		Model: c.model,
		Input: []string{input},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to encode embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("failed to create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to call embedding endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		snippet = bytes.TrimSpace(snippet)
		if len(snippet) == 0 {
			return 0, fmt.Errorf("embedding endpoint returned %s", resp.Status)
		}
		return 0, fmt.Errorf("embedding endpoint returned %s: %s", resp.Status, snippet)
	}

	var decoded response
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return 0, fmt.Errorf("failed to decode embedding response: %w", err)
	}
	if len(decoded.Data) == 0 {
		return 0, fmt.Errorf("embedding response contained no vectors")
	}
	return len(decoded.Data[0].Embedding), nil
}

func (c *Client) EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("inputs are required")
	}

	body, err := json.Marshal(request{
		Model: c.model,
		Input: inputs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call embedding endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		snippet = bytes.TrimSpace(snippet)
		if len(snippet) == 0 {
			return nil, fmt.Errorf("embedding endpoint returned %s", resp.Status)
		}
		return nil, fmt.Errorf("embedding endpoint returned %s: %s", resp.Status, snippet)
	}

	var decoded response
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(decoded.Data) != len(inputs) {
		return nil, fmt.Errorf("embedding response returned %d vector(s) for %d input(s)", len(decoded.Data), len(inputs))
	}

	vectors := make([][]float32, len(inputs))
	seen := make([]bool, len(inputs))
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= len(inputs) {
			return nil, fmt.Errorf("embedding response index %d out of range", item.Index)
		}
		if seen[item.Index] {
			return nil, fmt.Errorf("embedding response duplicated index %d", item.Index)
		}
		if len(item.Embedding) != c.dimension {
			return nil, fmt.Errorf("embedding dimension mismatch for index %d: got %d want %d", item.Index, len(item.Embedding), c.dimension)
		}
		seen[item.Index] = true
		vectors[item.Index] = append([]float32(nil), item.Embedding...)
	}

	for i, ok := range seen {
		if !ok {
			return nil, fmt.Errorf("embedding response missing index %d", i)
		}
	}

	return vectors, nil
}

type request struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type response struct {
	Data []responseItem `json:"data"`
}

type responseItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

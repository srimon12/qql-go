package server

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/srimon12/qql-go/gen/qqlpb"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/pkg/qql"
)

// Handler implements the qql.QQL Connect RPC service.
type Handler struct {
	client qql.QdrantClient
	config *config.Config
}

// NewHandler creates a new QQL Connect RPC handler.
func NewHandler(client qql.QdrantClient) *Handler {
	return &Handler{client: client, config: &config.Config{}}
}

// NewHandlerWithConfig creates a new QQL Connect RPC handler with config for model resolution.
func NewHandlerWithConfig(client qql.QdrantClient, cfg *config.Config) *Handler {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &Handler{client: client, config: cfg}
}

// Exec parses and executes a single QQL query.
func (h *Handler) Exec(
	ctx context.Context,
	req *connect.Request[qqlpb.ExecRequest],
) (*connect.Response[qqlpb.ExecResponse], error) {
	query := req.Msg.GetQuery()
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("query is required"))
	}

	result, err := qql.ExecWithConfig(ctx, h.client, query, h.config)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	data, err := json.Marshal(result.Data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to marshal result data: %w", err))
	}

	return connect.NewResponse(&qqlpb.ExecResponse{
		Ok:        result.OK,
		Operation: result.Operation,
		Message:   result.Message,
		Data:      data,
	}), nil
}

// ExecBatch executes multiple queries in one round-trip.
func (h *Handler) ExecBatch(
	ctx context.Context,
	req *connect.Request[qqlpb.ExecBatchRequest],
) (*connect.Response[qqlpb.ExecBatchResponse], error) {
	batchReq := req.Msg
	if len(batchReq.GetQueries()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one query is required"))
	}

	queries := make([]string, len(batchReq.GetQueries()))
	for i, q := range batchReq.GetQueries() {
		queries[i] = q.GetQuery()
	}

	allQuery := true
	for _, q := range queries {
		node, err := qql.Parse(q)
		if err != nil {
			allQuery = false
			break
		}
		if _, ok := node.(*ast.QueryStmt); !ok {
			allQuery = false
			break
		}
	}

	var results []*qql.Result
	var err error

	if allQuery && len(queries) > 1 {
		results, err = qql.BatchQueryWithConfig(ctx, h.client, queries, h.config)
	} else {
		results, err = qql.ExecBatchWithConfig(ctx, h.client, queries, batchReq.GetStopOnError(), h.config)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	batchResults := make([]*qqlpb.ExecResponse, len(results))
	for i, r := range results {
		data, err := json.Marshal(r.Data)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to marshal batch result %d: %w", i, err))
		}
		batchResults[i] = &qqlpb.ExecResponse{
			Ok:        r.OK,
			Operation: r.Operation,
			Message:   r.Message,
			Data:      data,
		}
	}

	return connect.NewResponse(&qqlpb.ExecBatchResponse{
		Results: batchResults,
	}), nil
}

// Explain returns the execution plan without running the query.
func (h *Handler) Explain(
	_ context.Context,
	req *connect.Request[qqlpb.ExplainRequest],
) (*connect.Response[qqlpb.ExplainResponse], error) {
	query := req.Msg.GetQuery()
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("query is required"))
	}

	ok, q, plan, err := qql.ExplainResult(query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&qqlpb.ExplainResponse{
		Ok:    ok,
		Query: q,
		Plan:  plan,
	}), nil
}

// Health returns gateway and Qdrant connection status.
func (h *Handler) Health(
	ctx context.Context,
	_ *connect.Request[qqlpb.HealthRequest],
) (*connect.Response[qqlpb.HealthResponse], error) {
	qdrantOK := false
	qdrantStatus := "disconnected"

	if h.client != nil {
		collections, err := h.client.ListCollections(ctx)
		if err == nil {
			qdrantOK = true
			qdrantStatus = fmt.Sprintf("connected (%d collections)", len(collections))
		} else {
			qdrantStatus = fmt.Sprintf("error: %v", err)
		}
	}

	return connect.NewResponse(&qqlpb.HealthResponse{
		Version:         Version,
		QdrantConnected: qdrantOK,
		QdrantStatus:    qdrantStatus,
	}), nil
}

// Convert translates Qdrant REST JSON into QQL statements.
func (h *Handler) Convert(
	_ context.Context,
	req *connect.Request[qqlpb.ConvertRequest],
) (*connect.Response[qqlpb.ConvertResponse], error) {
	jsonPayload := req.Msg.GetJsonPayload()
	if jsonPayload == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("json_payload is required"))
	}

	statements, err := qql.ConvertJSONToQQL(jsonPayload)
	if err != nil {
		return connect.NewResponse(&qqlpb.ConvertResponse{
			Ok:    false,
			Error: err.Error(),
		}), nil
	}

	return connect.NewResponse(&qqlpb.ConvertResponse{
		Ok:         true,
		Statements: statements,
	}), nil
}

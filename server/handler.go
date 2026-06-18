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

// Exec parses and executes a single QQL query with policy enforcement.
func (h *Handler) Exec(
	ctx context.Context,
	req *connect.Request[qqlpb.ExecRequest],
) (*connect.Response[qqlpb.ExecResponse], error) {
	query := req.Msg.GetQuery()
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("query is required"))
	}

	claims := ExtractClaimsFromContext(ctx)
	policy := ExtractEvaluatedPolicy(ctx)

	// Parse the query into AST.
	node, err := qql.Parse(query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parse error: %w", err))
	}

	// Apply policy enforcement if we have a policy.
	if policy != nil {
		injector := NewASTInjector(*policy, claims)

		// Enforce operation permission.
		if err := injector.EnforceOperation(node); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}

		// Transform the AST based on node type.
		if err := transformNode(injector, node); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
	}

	if meta := ExtractAuditMeta(ctx); meta != nil {
		meta.Collection = collectionFromNode(node)
		meta.Query = query
		if policy != nil {
			meta.RuleIndex = policy.MatchedRule
			if policy.InjectField != "" {
				meta.FiltersInjected = []string{
					fmt.Sprintf("%s %s %s", policy.InjectField, policy.InjectOp, policy.InjectValue),
				}
			}
			if policy.MaxLimit > 0 {
				lim := policy.MaxLimit
				meta.LimitCapped = &lim
			}
		}
	}

	// Execute the (possibly transformed) AST.
	result, err := qql.ExecAST(ctx, h.client, node, h.config)
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

// ExecBatch executes multiple queries with policy enforcement.
func (h *Handler) ExecBatch(
	ctx context.Context,
	req *connect.Request[qqlpb.ExecBatchRequest],
) (*connect.Response[qqlpb.ExecBatchResponse], error) {
	batchReq := req.Msg
	if len(batchReq.GetQueries()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one query is required"))
	}

	claims := ExtractClaimsFromContext(ctx)
	policy := ExtractEvaluatedPolicy(ctx)

	queries := make([]string, len(batchReq.GetQueries()))
	for i, q := range batchReq.GetQueries() {
		queries[i] = q.GetQuery()
	}

	// If policy is active, parse and transform each query individually.
	if policy != nil {
		injector := NewASTInjector(*policy, claims)
		nodes := make([]ast.ASTNode, len(queries))
		for i, q := range queries {
			node, err := qql.Parse(q)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parse error in query %d: %w", i, err))
			}
			if err := injector.EnforceOperation(node); err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("query %d: %w", i, err))
			}
			if err := transformNode(injector, node); err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("query %d: %w", i, err))
			}
			nodes[i] = node
		}

		// Execute each transformed AST.
		if meta := ExtractAuditMeta(ctx); meta != nil {
			meta.Query = fmt.Sprintf("[Batch of %d queries]", len(queries))
			if len(nodes) > 0 {
				meta.Collection = collectionFromNode(nodes[0])
			}
			meta.RuleIndex = policy.MatchedRule
			if policy.InjectField != "" {
				meta.FiltersInjected = []string{
					fmt.Sprintf("%s %s %s", policy.InjectField, policy.InjectOp, policy.InjectValue),
				}
			}
			if policy.MaxLimit > 0 {
				lim := policy.MaxLimit
				meta.LimitCapped = &lim
			}
		}

		allQuery := true
		for _, n := range nodes {
			if _, ok := n.(*ast.QueryStmt); !ok {
				allQuery = false
				break
			}
		}

		var qqlResults []*qql.Result
		var batchErr error

		if allQuery && len(nodes) > 1 {
			qqlResults, batchErr = qql.BatchQueryASTWithConfig(ctx, h.client, nodes, h.config)
		}

		if batchErr != nil {
			if batchReq.GetStopOnError() {
				return nil, connect.NewError(connect.CodeInternal, batchErr)
			}
			// Batch failed but stopOnError is false — fall through to sequential.
			qqlResults = nil
		}

		results := make([]*qqlpb.ExecResponse, len(nodes))
		
		if qqlResults != nil {
			// Fast path succeeded
			for i, result := range qqlResults {
				if result == nil {
					continue // Should not happen
				}
				data, err := json.Marshal(result.Data)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to marshal batch result %d: %w", i, err))
				}
				results[i] = &qqlpb.ExecResponse{
					Ok:        result.OK,
					Operation: result.Operation,
					Message:   result.Message,
					Data:      data,
				}
			}
		} else {
			// Sequential execution fallback (for INSERT, DELETE, mixed batches, or single query)
			for i, node := range nodes {
				result, err := qql.ExecAST(ctx, h.client, node, h.config)
				if err != nil {
					if batchReq.GetStopOnError() {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
					results[i] = &qqlpb.ExecResponse{
						Ok:      false,
						Message: err.Error(),
					}
					continue
				}
				data, err := json.Marshal(result.Data)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to marshal batch result %d: %w", i, err))
				}
				results[i] = &qqlpb.ExecResponse{
					Ok:        result.OK,
					Operation: result.Operation,
					Message:   result.Message,
					Data:      data,
				}
			}
		}

		return connect.NewResponse(&qqlpb.ExecBatchResponse{Results: results}), nil
	}

	// No policy — use the original fast path.
	if meta := ExtractAuditMeta(ctx); meta != nil {
		meta.Query = fmt.Sprintf("[Batch of %d queries]", len(queries))
		if len(queries) > 0 {
			if node, err := qql.Parse(queries[0]); err == nil {
				meta.Collection = collectionFromNode(node)
			}
		}
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

// transformNode applies policy-based transformations to an AST node.
func transformNode(injector *ASTInjector, node ast.ASTNode) error {
	switch n := node.(type) {
	case *ast.QueryStmt:
		return injector.TransformQuery(n)
	case *ast.ScrollStmt:
		return injector.TransformScroll(n)
	case *ast.DeleteStmt:
		return injector.TransformDelete(n)
	case *ast.InsertStmt:
		return injector.TransformInsert(n)
	case *ast.UpdatePayloadStmt:
		return injector.TransformUpdatePayload(n)
	case *ast.UpdateVectorStmt:
		return injector.TransformUpdateVector(n)
	case *ast.CreateCollectionStmt:
		return injector.TransformCreateCollection(n)
	case *ast.DropCollectionStmt:
		return injector.TransformDropCollection(n)
	case *ast.AlterCollectionStmt:
		return injector.TransformAlterCollection(n)
	case *ast.CreateIndexStmt:
		return injector.TransformCreateIndex(n)
	case *ast.SelectStmt:
		return injector.EnforceCollection(n.Collection)
	case *ast.ShowCollectionsStmt, *ast.ShowCollectionStmt:
		return nil // always allowed
	default:
		return nil
	}
}



// collectionFromNode extracts the collection name from an AST node.
func collectionFromNode(node ast.ASTNode) string {
	switch n := node.(type) {
	case *ast.QueryStmt:
		return n.Collection
	case *ast.InsertStmt:
		return n.Collection
	case *ast.DeleteStmt:
		return n.Collection
	case *ast.UpdatePayloadStmt:
		return n.Collection
	case *ast.UpdateVectorStmt:
		return n.Collection
	case *ast.ScrollStmt:
		return n.Collection
	case *ast.SelectStmt:
		return n.Collection
	case *ast.CreateCollectionStmt:
		return n.Collection
	case *ast.DropCollectionStmt:
		return n.Collection
	case *ast.AlterCollectionStmt:
		return n.Collection
	case *ast.CreateIndexStmt:
		return n.Collection
	case *ast.ShowCollectionStmt:
		return n.Collection
	default:
		return ""
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

package commands

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/filters"
)

func (e *Executor) doUpdateVector(n *ast.UpdateVectorStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}
	denseName := denseVectorName
	isMultiVector := false
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
		isMultiVector = true
	}
	if n.VectorName != nil {
		denseName = *n.VectorName
	}
	request, err := e.buildUpdateVectorRequest(ctx, n, denseName, isMultiVector)
	if err != nil {
		return nil, err
	}
	if _, err := e.client.UpdateVectors(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to update vector: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "update_vector",
		Message:   fmt.Sprintf("Updated vector for point [%v] in '%s'", n.PointID, n.Collection),
		Data: map[string]any{
			"collection": n.Collection,
			"point_id":   n.PointID,
		},
	}, nil
}

func (e *Executor) doUpdatePayload(n *ast.UpdatePayloadStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	request, err := buildUpdatePayloadRequest(n, e.requestTimeout())
	if err != nil {
		return nil, err
	}
	if _, err := e.client.SetPayload(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to update payload: %w", err)
	}

	message := fmt.Sprintf("Payload updated for point [%v] in '%s'", n.PointID, n.Collection)
	if n.QueryFilter != nil {
		message = fmt.Sprintf("Payload updated in '%s' (filter-based)", n.Collection)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "update_payload",
		Message:   message,
		Data: map[string]any{
			"collection": n.Collection,
		},
	}, nil
}

func (e *Executor) doDelete(n *ast.DeleteStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	request, err := buildDeleteRequest(n, e.requestTimeout())
	if err != nil {
		return nil, err
	}

	_, err = e.client.Delete(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to delete point: %w", err)
	}

	if n.Field != "" {
		return &ExecResponse{
			OK:        true,
			Operation: "delete",
			Message:   fmt.Sprintf("Deleted points matching %s = '%v' from '%s'", n.Field, n.Value, n.Collection),
			Data: map[string]any{
				"collection": n.Collection,
				"field":      n.Field,
				"value":      n.Value,
			},
		}, nil
	}

	return &ExecResponse{
		OK:        true,
		Operation: "delete",
		Message:   fmt.Sprintf("Deleted point '%v' from '%s'", n.PointID, n.Collection),
		Data: map[string]any{
			"collection": n.Collection,
			"point_id":   n.PointID,
		},
	}, nil
}

func (e *Executor) buildUpdateVectorRequest(ctx context.Context, n *ast.UpdateVectorStmt, vectorName string, isMultiVector bool) (*qdrant.UpdatePointVectors, error) {
	wait := true
	vectors := qdrant.NewVectors(n.Vector...)
	if isMultiVector {
		name := vectorName
		if n.VectorName != nil {
			name = *n.VectorName
		}
		vectors = qdrant.NewVectorsMap(map[string]*qdrant.Vector{
			name: qdrant.NewVectorDense(n.Vector),
		})
	}

	pID, err := newPointID(n.PointID)
	if err != nil {
		return nil, err
	}

	return &qdrant.UpdatePointVectors{
		CollectionName: n.Collection,
		Wait:           &wait,
		Timeout:        e.requestTimeout(),
		Points: []*qdrant.PointVectors{
			{
				Id:      pID,
				Vectors: vectors,
			},
		},
	}, nil
}

func buildUpdatePayloadRequest(n *ast.UpdatePayloadStmt, timeout *uint64) (*qdrant.SetPayloadPoints, error) {
	wait := true
	request := &qdrant.SetPayloadPoints{
		CollectionName: n.Collection,
		Payload:        qdrant.NewValueMap(n.Payload),
		Wait:           &wait,
		Timeout:        timeout,
	}

	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build update payload filter: %w", err)
		}
		request.PointsSelector = qdrant.NewPointsSelectorFilter(filter)
		return request, nil
	}

	pID, err := newPointID(n.PointID)
	if err != nil {
		return nil, err
	}
	request.PointsSelector = qdrant.NewPointsSelector(pID)
	return request, nil
}

func buildDeleteRequest(n *ast.DeleteStmt, timeout *uint64) (*qdrant.DeletePoints, error) {
	wait := true

	if n.Field != "" {
		filter, err := filters.NewFilterConverter().BuildFilter(&ast.CompareExpr{
			Field: n.Field,
			Op:    "=",
			Value: n.Value,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build delete filter: %w", err)
		}
		return &qdrant.DeletePoints{
			CollectionName: n.Collection,
			Points:         qdrant.NewPointsSelectorFilter(filter),
			Wait:           &wait,
			Timeout:        timeout,
		}, nil
	}

	pid, err := newPointID(n.PointID)
	if err != nil {
		return nil, fmt.Errorf("invalid point ID: %w", err)
	}

	return &qdrant.DeletePoints{
		CollectionName: n.Collection,
		Points:         qdrant.NewPointsSelector(pid),
		Wait:           &wait,
		Timeout:        timeout,
	}, nil
}

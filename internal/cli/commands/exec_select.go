package commands

import (
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/filters"
)

func (e *Executor) doSelect(n *ast.SelectStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()
	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}
	pointID, err := newPointID(n.PointID)
	if err != nil {
		return nil, err
	}
	records, err := e.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: n.Collection,
		Ids:            []*qdrant.PointId{pointID},
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve point: %w", err)
	}
	if len(records) == 0 {
		return &ExecResponse{
			OK:        true,
			Operation: "select",
			Message:   fmt.Sprintf("Point '%v' not found in '%s'", n.PointID, n.Collection),
			Data:      nil,
		}, nil
	}
	record := records[0]
	return &ExecResponse{
		OK:        true,
		Operation: "select",
		Message:   fmt.Sprintf("Retrieved point '%v' from '%s'", n.PointID, n.Collection),
		Data: map[string]any{
			"id":      pointIDString(record.GetId()),
			"payload": convertRetrievedPayload(record.GetPayload()),
		},
	}, nil
}

func (e *Executor) doScroll(n *ast.ScrollStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()
	exists, err := e.client.CollectionExists(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("collection '%s' does not exist", n.Collection)
	}

	req := &qdrant.ScrollPoints{
		CollectionName: n.Collection,
		Limit:          qdrant.PtrOf(uint32(n.Limit)),
		WithPayload:    qdrant.NewWithPayload(true),
		WithVectors:    qdrant.NewWithVectors(false),
	}
	if n.After != nil {
		pID, err := newPointID(n.After)
		if err != nil {
			return nil, err
		}
		req.Offset = pID
	}
	if n.QueryFilter != nil {
		filter, err := filters.NewFilterConverter().BuildFilter(n.QueryFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to build filter: %w", err)
		}
		req.Filter = filter
	}

	records, nextOffset, err := e.client.ScrollAndOffset(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("scroll failed: %w", err)
	}

	points := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		points = append(points, map[string]any{
			"id":      pointIDString(rec.GetId()),
			"payload": convertRetrievedPayload(rec.GetPayload()),
		})
	}

	var next any
	if nextOffset != nil {
		next = pointIDValue(nextOffset)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "scroll",
		Message:   fmt.Sprintf("Scrolled %d point(s) from '%s'", len(points), n.Collection),
		Data: map[string]any{
			"points":      points,
			"next_offset": next,
		},
	}, nil
}

func convertRetrievedPayload(payload map[string]*qdrant.Value) map[string]any {
	result := make(map[string]any, len(payload))
	for key, val := range payload {
		result[key] = convertValue(val)
	}
	return result
}

func convertValue(val *qdrant.Value) any {
	switch v := val.GetKind().(type) {
	case *qdrant.Value_StringValue:
		return v.StringValue
	case *qdrant.Value_IntegerValue:
		return v.IntegerValue
	case *qdrant.Value_DoubleValue:
		return v.DoubleValue
	case *qdrant.Value_BoolValue:
		return v.BoolValue
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_ListValue:
		items := make([]any, 0, len(v.ListValue.GetValues()))
		for _, item := range v.ListValue.GetValues() {
			items = append(items, convertValue(item))
		}
		return items
	case *qdrant.Value_StructValue:
		return convertRetrievedPayload(v.StructValue.GetFields())
	default:
		return nil
	}
}

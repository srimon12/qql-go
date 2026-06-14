package commands

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/sparse"
)

func (e *Executor) doInsert(n *ast.InsertStmt) (*ExecResponse, error) {
	ctx := context.Background()

	textVal, ok := n.Values["text"]
	if !ok {
		return nil, fmt.Errorf("INSERT requires a 'text' field in VALUES")
	}
	text, ok := textVal.(string)
	if !ok {
		return nil, fmt.Errorf("'text' field must be a string")
	}

	created, err := e.ensureCollectionForInsert(ctx, n.Collection, n.Model, n.Hybrid, n.DenseVector, n.SparseVector)
	if err != nil {
		return nil, err
	}

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)
	useHybrid, err := e.shouldUseHybrid(ctx, n.Collection, n.Hybrid)
	if err != nil {
		return nil, err
	}
	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}
	includeRerank := topo != nil && topo.RerankVector != nil

	pointID, payload, err := insertPointIDAndPayload(n.PointID, n.Values)
	if err != nil {
		return nil, err
	}

	denseName, sparseName := denseVectorName, sparseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if topo != nil && topo.SparseVector != nil && *topo.SparseVector != "" {
		sparseName = *topo.SparseVector
	}
	if n.DenseVector != nil {
		denseName = *n.DenseVector
	}
	if n.SparseVector != nil {
		sparseName = *n.SparseVector
	}
	vectors, err := e.buildInsertVectors(ctx, text, model, sparseModel, useHybrid, includeRerank, n.Collection, denseName, sparseName)
	if err != nil {
		return nil, err
	}

	_, err = e.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: n.Collection,
		Points: []*qdrant.PointStruct{
			{
				Id:      newPointID(pointID),
				Vectors: qdrant.NewVectorsMap(vectors),
				Payload: e.buildPayload(payload),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "insert",
		Message:   fmt.Sprintf("Inserted 1 point [%v] into '%s'", pointID, n.Collection),
		Data: map[string]any{
			"id":           pointID,
			"collection":   n.Collection,
			"created":      created,
			"hybrid":       useHybrid,
			"dense_model":  model,
			"sparse_model": sparseModel,
			"rerank":       includeRerank,
		},
	}, nil
}

func (e *Executor) doInsertBulk(n *ast.InsertBulkStmt) (*ExecResponse, error) {
	ctx := context.Background()
	if len(n.ValuesList) == 0 {
		return nil, fmt.Errorf("INSERT BULK VALUES list is empty")
	}

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)

	texts := make([]string, 0, len(n.ValuesList))
	pointIDs := make([]any, 0, len(n.ValuesList))
	payloads := make([]map[string]any, 0, len(n.ValuesList))
	for idx, values := range n.ValuesList {
		textVal, ok := values["text"]
		if !ok {
			return nil, fmt.Errorf("INSERT BULK item %d requires a 'text' field in VALUES", idx)
		}
		text, ok := textVal.(string)
		if !ok {
			return nil, fmt.Errorf("INSERT BULK item %d 'text' field must be a string", idx)
		}
		pointID, payload, err := insertPointIDAndPayload(nil, values)
		if err != nil {
			return nil, fmt.Errorf("INSERT BULK item %d: %w", idx, err)
		}
		texts = append(texts, text)
		pointIDs = append(pointIDs, pointID)
		payloads = append(payloads, payload)
	}

	created, err := e.ensureCollectionForInsert(ctx, n.Collection, n.Model, n.Hybrid, n.DenseVector, n.SparseVector)
	if err != nil {
		return nil, err
	}
	useHybrid, err := e.shouldUseHybrid(ctx, n.Collection, n.Hybrid)
	if err != nil {
		return nil, err
	}
	topo, err := e.resolveVectorTopology(ctx, n.Collection)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect collection: %w", err)
	}
	includeRerank := topo != nil && topo.RerankVector != nil

	denseName, sparseName := denseVectorName, sparseVectorName
	if topo != nil && topo.DenseVector != nil && *topo.DenseVector != "" {
		denseName = *topo.DenseVector
	}
	if topo != nil && topo.SparseVector != nil && *topo.SparseVector != "" {
		sparseName = *topo.SparseVector
	}
	if n.DenseVector != nil {
		denseName = *n.DenseVector
	}
	if n.SparseVector != nil {
		sparseName = *n.SparseVector
	}
	vectorsBatch, err := e.buildInsertVectorsBatch(ctx, texts, model, sparseModel, useHybrid, includeRerank, n.Collection, denseName, sparseName)
	if err != nil {
		return nil, err
	}

	points := make([]*qdrant.PointStruct, 0, len(texts))
	for idx, vectors := range vectorsBatch {
		points = append(points, &qdrant.PointStruct{
			Id:      newPointID(pointIDs[idx]),
			Vectors: qdrant.NewVectorsMap(vectors),
			Payload: e.buildPayload(payloads[idx]),
		})
	}

	_, err = e.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: n.Collection,
		Points:         points,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert bulk points: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "insert_bulk",
		Message:   fmt.Sprintf("Inserted %d point(s) into '%s'", len(points), n.Collection),
		Data: map[string]any{
			"count":        len(points),
			"collection":   n.Collection,
			"created":      created,
			"hybrid":       useHybrid,
			"dense_model":  model,
			"sparse_model": sparseModel,
			"rerank":       includeRerank,
		},
	}, nil
}

func (e *Executor) buildInsertVectors(ctx context.Context, text, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string, denseName, sparseName string) (map[string]*qdrant.Vector, error) {
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}

		denseVector, sparseVec, err := embedConcurrent(ctx, embedClient, text, includeSparse)
		if err != nil {
			return nil, err
		}

		vectors := map[string]*qdrant.Vector{
			denseName: qdrant.NewVectorDense(denseVector),
		}
		if includeSparse {
			vectors[sparseName] = qdrant.NewVectorSparse(sparseVec.Indices, sparseVec.Values)
		}
		if includeRerank {
			return nil, fmt.Errorf("local/external rerank vectors are not implemented yet")
		}
		return vectors, nil
	}

	// Cloud inference path: send text as a Document, Qdrant cluster handles embedding.
	opts := buildDocumentOptionsFromMap(e.cloudModelOptions())
	vectors := map[string]*qdrant.Vector{
		denseName: qdrant.NewVectorDocument(&qdrant.Document{
			Text:    text,
			Model:   denseModel,
			Options: opts,
		}),
	}
	if includeSparse {
		vectors[sparseName] = qdrant.NewVectorDocument(&qdrant.Document{
			Text:    text,
			Model:   sparseModel,
			Options: opts,
		})
	}
	if includeRerank {
		vectors[rerankVectorName] = qdrant.NewVectorDocument(&qdrant.Document{
			Text:    text,
			Model:   rerankModelDefault,
			Options: opts,
		})
	}
	return vectors, nil
}

func (e *Executor) buildInsertVectorsBatch(ctx context.Context, texts []string, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string, denseName, sparseName string) ([]map[string]*qdrant.Vector, error) {
	if e.usesLocalEmbeddings() {
		embedClient, err := e.embeddingClient(denseModel)
		if err != nil {
			return nil, err
		}
		denseVectors, err := embedClient.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("failed to embed insert texts: %w", err)
		}
		if includeRerank {
			return nil, fmt.Errorf("local/external rerank vectors are not implemented yet")
		}

		batch := make([]map[string]*qdrant.Vector, len(texts))
		sparseVectors := make([]sparse.Vector, len(texts))

		if includeSparse {
			var wg sync.WaitGroup
			wg.Add(len(texts))
			for i, text := range texts {
				go func(idx int, t string) {
					defer wg.Done()
					sparseVectors[idx] = sparse.BuildDocument(t)
				}(i, text)
			}
			wg.Wait()
		}

		for idx := range texts {
			vectors := map[string]*qdrant.Vector{
				denseName: qdrant.NewVectorDense(denseVectors[idx]),
			}
			if includeSparse {
				sv := sparseVectors[idx]
				vectors[sparseName] = qdrant.NewVectorSparse(sv.Indices, sv.Values)
			}
			batch[idx] = vectors
		}
		return batch, nil
	}

	batch := make([]map[string]*qdrant.Vector, 0, len(texts))
	for _, text := range texts {
		vectors, err := e.buildInsertVectors(ctx, text, denseModel, sparseModel, includeSparse, includeRerank, collection, denseName, sparseName)
		if err != nil {
			return nil, err
		}
		batch = append(batch, vectors)
	}
	return batch, nil
}

func (e *Executor) buildPayload(values map[string]any) map[string]*qdrant.Value {
	return qdrant.NewValueMap(values)
}

func insertPointIDAndPayload(pointID any, values map[string]any) (any, map[string]any, error) {
	payload := make(map[string]any, len(values))
	maps.Copy(payload, values)
	rawID := pointID
	if rawID == nil {
		var ok bool
		rawID, ok = payload["id"]
		if ok {
			delete(payload, "id")
		}
	}
	if rawID == nil {
		return uuid.New().String(), payload, nil
	}
	switch value := rawID.(type) {
	case int:
		if value < 0 {
			return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
		}
		return value, payload, nil
	case string:
		if _, err := uuid.Parse(value); err != nil {
			return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
		}
		return value, payload, nil
	default:
		return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
	}
}

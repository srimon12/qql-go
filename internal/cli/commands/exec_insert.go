package commands

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/sparse"
)

func (e *Executor) doInsert(n *ast.InsertStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()
	if len(n.ValuesList) == 0 {
		return nil, fmt.Errorf("INSERT VALUES list is empty")
	}

	model := e.resolveDenseModel(n.Model)
	sparseModel := e.resolveSparseModel(n.SparseModel)

	hasEmbed := len(n.EmbedDirectives) > 0

	preProvidedVectors, err := extractProvidedVectors(n.ValuesList)
	if err != nil {
		return nil, err
	}
	hasProvidedVectors := len(preProvidedVectors) > 0

	// texts is only needed when building vectors from auto-embedding (no EMBED, no pre-provided vectors).
	// Declared here so it's available for the buildInsertVectorsBatch fallback path.
	var texts []string
	if !hasEmbed && !hasProvidedVectors {
		texts = make([]string, 0, len(n.ValuesList))
	}

	pointIDs := make([]any, 0, len(n.ValuesList))
	payloads := make([]map[string]any, 0, len(n.ValuesList))
	for idx, values := range n.ValuesList {
		if !hasEmbed {
			textVal, ok := values["text"]
			if !ok {
				return nil, fmt.Errorf("INSERT row %d requires a 'text' field in VALUES", idx)
			}
			if _, ok := textVal.(string); !ok {
				return nil, fmt.Errorf("INSERT row %d 'text' field must be a string", idx)
			}
			if texts != nil {
				texts = append(texts, textVal.(string))
			}
		}
		pointID, payload, err := insertPointIDAndPayload(values)
		if err != nil {
			return nil, fmt.Errorf("INSERT row %d: %w", idx, err)
		}
		// Strip vector keys from payload if pre-provided
		if hasProvidedVectors {
			stripVectorKeys(payload)
		}
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

	var vectorsBatch []map[string]*qdrant.Vector
	if hasEmbed {
		vectorsBatch, err = e.buildEmbedVectorsBatch(ctx, n.ValuesList, n.EmbedDirectives, model, sparseModel)
		if err != nil {
			return nil, err
		}
	} else if hasProvidedVectors {
		vectorsBatch = preProvidedVectors
	} else {
		vectorsBatch, err = e.buildInsertVectorsBatch(ctx, texts, model, sparseModel, useHybrid, includeRerank, n.Collection, denseName, sparseName)
		if err != nil {
			return nil, err
		}
	}

	points := make([]*qdrant.PointStruct, 0, len(n.ValuesList))
	for idx, vectors := range vectorsBatch {
		pID, err := newPointID(pointIDs[idx])
		if err != nil {
			return nil, err
		}
		points = append(points, &qdrant.PointStruct{
			Id:      pID,
			Vectors: qdrant.NewVectorsMap(vectors),
			Payload: e.buildPayload(payloads[idx]),
		})
	}

	_, err = e.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: n.Collection,
		Points:         points,
		Wait:           qdrant.PtrOf(true),
		Timeout:        e.requestTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert: %w", err)
	}

	return &ExecResponse{
		OK:        true,
		Operation: "insert",
		Message:   fmt.Sprintf("Inserted %d point(s) into '%s'", len(points), n.Collection),
		Data: map[string]any{
			"count":        len(points),
			"collection":   n.Collection,
			"created":      created,
			"hybrid":       useHybrid,
			"dense_model":  model,
			"sparse_model": sparseModel,
			"rerank":       includeRerank,
			"embed":        hasEmbed,
		},
	}, nil
}

func (e *Executor) buildInsertVectors(ctx context.Context, text, denseModel, sparseModel string, includeSparse, includeRerank bool, collection string, denseName, sparseName string) (map[string]*qdrant.Vector, error) {
	vectors := make(map[string]*qdrant.Vector)

	denseVec, err := e.embedTextToVector(ctx, text, denseModel, false)
	if err != nil {
		return nil, err
	}
	vectors[denseName] = denseVec

	if includeSparse {
		sparseVec, err := e.embedTextToVector(ctx, text, sparseModel, true)
		if err != nil {
			return nil, err
		}
		vectors[sparseName] = sparseVec
	}
	if includeRerank {
		if e.usesLocalEmbeddings() {
			return nil, fmt.Errorf("local/external rerank vectors are not implemented yet")
		}
		vectors[rerankVectorName] = qdrant.NewVectorDocument(&qdrant.Document{
			Text:    text,
			Model:   rerankModelDefault,
			Options: buildDocumentOptionsFromMap(e.cloudModelOptions()),
		})
	}
	return vectors, nil
}

// embedTextToVector embeds a single text string into a qdrant.Vector.
// When isSparse is true, it produces a sparse vector; otherwise dense.
func (e *Executor) embedTextToVector(ctx context.Context, text, model string, isSparse bool) (*qdrant.Vector, error) {
	if e.usesLocalEmbeddings() {
		if isSparse {
			sv := sparse.BuildDocument(text)
			return qdrant.NewVectorSparse(sv.Indices, sv.Values), nil
		}
		embedClient, err := e.embeddingClient(model)
		if err != nil {
			return nil, err
		}
		dv, err := embedClient.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		return qdrant.NewVectorDense(dv), nil
	}

	opts := buildDocumentOptionsFromMap(e.cloudModelOptions())
	return qdrant.NewVectorDocument(&qdrant.Document{
		Text:    text,
		Model:   model,
		Options: opts,
	}), nil
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

// buildEmbedVectorsBatch builds vectors for each row using EMBED directives.
// Each directive maps a source payload field to a target vector name.
func (e *Executor) buildEmbedVectorsBatch(ctx context.Context, valuesList []map[string]any, directives []ast.EmbedDirective, defaultDenseModel, defaultSparseModel string) ([]map[string]*qdrant.Vector, error) {
	// Validate no duplicate target vectors.
	seen := make(map[string]string, len(directives))
	for _, dir := range directives {
		if prev, exists := seen[dir.TargetVector]; exists {
			return nil, fmt.Errorf("EMBED duplicate target vector '%s': already mapped from '%s', cannot also map from '%s'", dir.TargetVector, prev, dir.SourceField)
		}
		seen[dir.TargetVector] = dir.SourceField
	}

	batch := make([]map[string]*qdrant.Vector, len(valuesList))

	for rowIdx, values := range valuesList {
		vectors := make(map[string]*qdrant.Vector, len(directives))

		for _, dir := range directives {
			sourceVal, ok := values[dir.SourceField]
			if !ok {
				return nil, fmt.Errorf("EMBED row %d: source field '%s' not found in VALUES", rowIdx, dir.SourceField)
			}
			sourceText, ok := sourceVal.(string)
			if !ok {
				return nil, fmt.Errorf("EMBED row %d: source field '%s' must be a string", rowIdx, dir.SourceField)
			}

			isSparse := dir.SparseModel != nil
			var model string
			if isSparse {
				if dir.SparseModel != nil && *dir.SparseModel != "" {
					model = *dir.SparseModel
				} else {
					model = defaultSparseModel
				}
			} else if dir.Model != nil && *dir.Model != "" {
				model = *dir.Model
			} else {
				model = defaultDenseModel
			}

			vec, err := e.embedTextToVector(ctx, sourceText, model, isSparse)
			if err != nil {
				return nil, fmt.Errorf("EMBED row %d field '%s': %w", rowIdx, dir.SourceField, err)
			}
			vectors[dir.TargetVector] = vec
		}

		batch[rowIdx] = vectors
	}
	return batch, nil
}

func insertPointIDAndPayload(values map[string]any) (any, map[string]any, error) {
	payload := make(map[string]any, len(values))
	maps.Copy(payload, values)
	rawID := extractID(payload)
	if rawID == nil {
		return nil, nil, fmt.Errorf("INSERT requires an 'id' field in VALUES (unsigned integer or UUID string)")
	}
	switch value := rawID.(type) {
	case int:
		if value < 0 {
			return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
		}
		return value, payload, nil
	case string:
		if num, err := parseUint64(value); err == nil {
			return num, payload, nil
		}
		if _, err := uuid.Parse(value); err != nil {
			return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
		}
		return value, payload, nil
	default:
		return nil, nil, fmt.Errorf("INSERT id must be an unsigned integer or UUID string when provided")
	}
}

func extractID(payload map[string]any) any {
	for key, value := range payload {
		if strings.ToLower(key) == "id" {
			delete(payload, key)
			return value
		}
	}
	return nil
}

func isVectorKey(key string) bool {
	return key == "vector" || key == "_v" || strings.HasPrefix(key, "_v_")
}

func extractProvidedVectors(valuesList []map[string]any) ([]map[string]*qdrant.Vector, error) {
	hasAny := false
	for _, values := range valuesList {
		for key := range values {
			if isVectorKey(key) {
				hasAny = true
				break
			}
		}
		if hasAny {
			break
		}
	}
	if !hasAny {
		return nil, nil
	}

	all := make([]map[string]*qdrant.Vector, len(valuesList))
	for i, values := range valuesList {
		var rowVectors map[string]*qdrant.Vector
		for key, value := range values {
			if !isVectorKey(key) {
				continue
			}
			if rowVectors == nil {
				rowVectors = make(map[string]*qdrant.Vector)
			}
			if key == "vector" {
				vecMap, ok := value.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid vector data: expected dictionary for 'vector' key")
				}
				for vName, vData := range vecMap {
					vec, err := parseAnyToQdrantVector(vData)
					if err != nil {
						return nil, fmt.Errorf("invalid vector data for '%s': %w", vName, err)
					}
					rowVectors[vName] = vec
				}
				continue
			}

			var vecName string
			if key != "_v" {
				vecName = unescapeVectorKey(key[3:])
			}
			vec, err := parseAnyToQdrantVector(value)
			if err != nil {
				return nil, fmt.Errorf("invalid vector data for key '%s': %w", key, err)
			}
			rowVectors[vecName] = vec
		}
		all[i] = rowVectors
	}
	return all, nil
}

func parseAnyToQdrantVector(value any) (*qdrant.Vector, error) {
	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", value)
	}
	if len(arr) == 0 {
		return qdrant.NewVectorDense(nil), nil
	}
	if _, isMulti := arr[0].([]any); isMulti {
		var multiDense [][]float32
		for i, inner := range arr {
			floats, err := anyToFloat32Slice(inner)
			if err != nil {
				return nil, fmt.Errorf("at index %d: %w", i, err)
			}
			multiDense = append(multiDense, floats)
		}
		return qdrant.NewVectorMulti(multiDense), nil
	}
	floats, err := anyToFloat32Slice(value)
	if err != nil {
		return nil, err
	}
	return qdrant.NewVectorDense(floats), nil
}

func anyToFloat32Slice(value any) ([]float32, error) {
	arr, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", value)
	}
	out := make([]float32, len(arr))
	for i, v := range arr {
		switch f := v.(type) {
		case float64:
			out[i] = float32(f)
		case int:
			out[i] = float32(f)
		case int64:
			out[i] = float32(f)
		case uint64:
			out[i] = float32(f)
		default:
			return nil, fmt.Errorf("element %d is not a number (got %T)", i, v)
		}
	}
	return out, nil
}

func stripVectorKeys(payload map[string]any) {
	for key := range payload {
		if isVectorKey(key) {
			delete(payload, key)
		}
	}
}

func unescapeVectorKey(name string) string {
	return strings.ReplaceAll(name, "__", "_")
}

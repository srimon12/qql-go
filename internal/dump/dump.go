package dump

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/qdrantutil"
)

type Client interface {
	CollectionExists(ctx context.Context, collectionName string) (bool, error)
	GetCollectionInfo(ctx context.Context, collectionName string) (*qdrant.CollectionInfo, error)
	Count(ctx context.Context, request *qdrant.CountPoints) (uint64, error)
	ScrollAndOffset(ctx context.Context, request *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
}

func Collection(ctx context.Context, client Client, collection, outputPath string, batchSize int) (int, int, error) {
	return CollectionWithModel(ctx, client, collection, outputPath, batchSize, "", "")
}

func CollectionWithModel(ctx context.Context, client Client, collection, outputPath string, batchSize int, denseModel, sparseModel string) (int, int, error) {
	if batchSize <= 0 {
		return 0, 0, fmt.Errorf("batch size must be greater than 0")
	}
	exists, err := client.CollectionExists(ctx, collection)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to check collection: %w", err)
	}
	if !exists {
		return 0, 0, fmt.Errorf("collection '%s' does not exist", collection)
	}

	info, err := client.GetCollectionInfo(ctx, collection)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get collection info: %w", err)
	}

	hybrid, denseName, sparseName, err := getVectorTopology(info)
	if err != nil {
		return 0, 0, err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 0, 0, fmt.Errorf("failed to prepare output directory: %w", err)
	}

	var header strings.Builder
	fmt.Fprintf(&header, "-- QQL dump for %s\n", collection)
	if denseModel != "" {
		fmt.Fprintf(&header, "-- Default model: %s\n", denseModel)
		if hybrid && sparseModel != "" {
			fmt.Fprintf(&header, "-- Sparse model : %s\n", sparseModel)
		}
	}

	var builder strings.Builder
	createLine := buildDumpCreateLine(collection, hybrid, denseName, sparseName, denseModel, sparseModel, info)
	builder.WriteString(createLine)
	builder.WriteString("\n\n")

	// Payload indexes
	payloadIndexStmts := buildPayloadIndexStatements(collection, info.PayloadSchema)
	if len(payloadIndexStmts) > 0 {
		builder.WriteString(strings.Join(payloadIndexStmts, "\n"))
		builder.WriteString("\n\n")
	}

	written := 0
	skipped := 0
	var offset *qdrant.PointId
	for {
		if err := ctx.Err(); err != nil {
			return written, skipped, err
		}
		points, nextOffset, err := client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: collection,
			Limit:          qdrant.PtrOf(uint32(batchSize)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(true),
		})
		if err != nil {
			return written, skipped, fmt.Errorf("failed to scroll collection: %w", err)
		}
		if len(points) == 0 {
			break
		}

		batch := make([]map[string]any, 0, len(points))
		for _, point := range points {
			payload := point.GetPayload()
			textValue, ok := payload["text"]
			if !ok || textValue.GetStringValue() == "" {
				skipped++
				continue
			}
			record := payloadToMap(payload)
			record["id"] = pointIDValue(point.GetId())

			if point.Vectors != nil {
				addVectorsToRecord(record, point.Vectors)
			}

			batch = append(batch, record)
		}

		if len(batch) > 0 {
			fmt.Fprintf(&builder, "INSERT INTO %s VALUES\n", collection)
			for idx, record := range batch {
				builder.WriteString(indent(serializeMap(record), "  "))
				if idx+1 < len(batch) {
					builder.WriteString(",")
				}
				builder.WriteString("\n")
				written++
			}
			builder.WriteString(buildInsertUsingClause(hybrid, denseName, sparseName, denseModel, sparseModel))
			builder.WriteString("\n\n")
		}

		if nextOffset == nil {
			break
		}
		offset = nextOffset
	}

	fmt.Fprintf(&header, "-- Points: %d\n\n", written)

	finalOutput := header.String() + builder.String() + fmt.Sprintf("-- Written: %d\n-- Skipped: %d\n", written, skipped)

	if err := os.WriteFile(outputPath, []byte(finalOutput), 0o600); err != nil {
		return written, skipped, fmt.Errorf("failed to write dump: %w", err)
	}
	return written, skipped, nil
}

func buildInsertUsingClause(hybrid bool, denseName, sparseName, denseModel, sparseModel string) string {
	if hybrid {
		if denseModel != "" {
			parts := []string{" USING HYBRID"}
			parts = append(parts, fmt.Sprintf("DENSE MODEL '%s'", escapeString(denseModel)))
			if sparseModel != "" {
				parts = append(parts, fmt.Sprintf("SPARSE MODEL '%s'", escapeString(sparseModel)))
			}
			return strings.Join(parts, " ")
		}
		if denseName != "dense" || sparseName != "sparse" {
			return fmt.Sprintf(" USING HYBRID DENSE VECTOR '%s' SPARSE VECTOR '%s'", escapeString(denseName), escapeString(sparseName))
		}
		return " USING HYBRID"
	}
	if denseModel != "" {
		return fmt.Sprintf(" USING MODEL '%s'", escapeString(denseModel))
	}
	if denseName != "dense" && denseName != "" {
		return fmt.Sprintf(" USING VECTOR '%s'", escapeString(denseName))
	}
	return ""
}

func addVectorsToRecord(record map[string]any, vectors *qdrant.VectorsOutput) {
	if named := vectors.GetVectors(); named != nil {
		for vname, vout := range named.GetVectors() {
			vkey := "_v_" + escapeVectorKey(vname)
			if dense := vout.GetDense(); dense != nil {
				data := denseVectorToAny(dense.GetData())
				if len(data) > 0 {
					record[vkey] = data
				}
			}
		}
	} else if single := vectors.GetVector(); single != nil {
		if dense := single.GetDense(); dense != nil {
			data := denseVectorToAny(dense.GetData())
			if len(data) > 0 {
				record["_v"] = data
			}
		}
	}
}

func escapeVectorKey(name string) string {
	name = strings.ReplaceAll(name, "_", "__")
	return name
}

func unescapeVectorKey(name string) string {
	return strings.ReplaceAll(name, "__", "_")
}

func denseVectorToAny(data []float32) []any {
	out := make([]any, len(data))
	for i, v := range data {
		out[i] = float64(v)
	}
	return out
}

func getVectorTopology(info *qdrant.CollectionInfo) (hybrid bool, denseName, sparseName string, err error) {
	if info == nil {
		return false, "", "", nil
	}
	config := info.GetConfig()
	if config == nil {
		return false, "", "", nil
	}
	params := config.GetParams()
	if params == nil {
		return false, "", "", nil
	}

	if vcfg := params.GetVectorsConfig(); vcfg != nil {
		if vmap := vcfg.GetParamsMap(); vmap != nil {
			for k := range vmap.GetMap() {
				if k == "dense" {
					denseName = "dense"
				} else if denseName == "" {
					denseName = k
				}
			}
		} else {
			denseName = ""
		}
	} else {
		denseName = "dense"
	}
	if scfg := params.GetSparseVectorsConfig(); scfg != nil {
		hybrid = true
		for k := range scfg.GetMap() {
			if k == "sparse" {
				sparseName = "sparse"
			} else if sparseName == "" {
				sparseName = k
			}
		}
	}
	return hybrid, denseName, sparseName, nil
}

func payloadToMap(payload map[string]*qdrant.Value) map[string]any {
	result := make(map[string]any, len(payload))
	for key, val := range payload {
		result[key] = payloadValue(val)
	}
	return result
}

func payloadValue(value *qdrant.Value) any {
	switch kind := value.GetKind().(type) {
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_BoolValue:
		return kind.BoolValue
	case *qdrant.Value_IntegerValue:
		return kind.IntegerValue
	case *qdrant.Value_DoubleValue:
		return kind.DoubleValue
	case *qdrant.Value_StringValue:
		return kind.StringValue
	case *qdrant.Value_ListValue:
		items := make([]any, 0, len(kind.ListValue.GetValues()))
		for _, item := range kind.ListValue.GetValues() {
			items = append(items, payloadValue(item))
		}
		return items
	case *qdrant.Value_StructValue:
		return payloadToMap(kind.StructValue.GetFields())
	default:
		return nil
	}
}

func pointIDValue(id *qdrant.PointId) any {
	if id == nil {
		return ""
	}
	switch value := id.GetPointIdOptions().(type) {
	case *qdrant.PointId_Uuid:
		return value.Uuid
	case *qdrant.PointId_Num:
		return value.Num
	default:
		return fmt.Sprintf("%v", id)
	}
}

func serializeMap(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("{\n")
	for idx, key := range keys {
		fmt.Fprintf(&builder, "  '%s': %s", escapeString(key), serializeValue(values[key]))
		if idx+1 < len(keys) {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("}")
	return builder.String()
}

func serializeValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case uint64:
		return fmt.Sprintf("%d", typed)
	case float64:
		return fmt.Sprintf("%v", typed)
	case string:
		return "'" + escapeString(typed) + "'"
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, serializeValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return serializeMap(typed)
	default:
		return "'" + escapeString(fmt.Sprintf("%v", value)) + "'"
	}
}

func escapeString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "'", "\\'")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\t", "\\t")
	value = strings.ReplaceAll(value, "\x00", "\\0")
	return value
}

func buildDumpCreateLine(collection string, hybrid bool, denseName, sparseName, denseModel, sparseModel string, info *qdrant.CollectionInfo) string {
	var b strings.Builder
	b.WriteString("CREATE COLLECTION ")
	b.WriteString(collection)

	if hybrid {
		if denseModel != "" {
			b.WriteString(" HYBRID")
			fmt.Fprintf(&b, " DENSE MODEL '%s'", escapeString(denseModel))
			if sparseModel != "" {
				fmt.Fprintf(&b, " SPARSE MODEL '%s'", escapeString(sparseModel))
			}
		} else {
			b.WriteString(" HYBRID")
			if denseName != "dense" || sparseName != "sparse" {
				fmt.Fprintf(&b, " DENSE VECTOR '%s' SPARSE VECTOR '%s'", escapeString(denseName), escapeString(sparseName))
			}
		}
	} else if denseModel != "" {
		fmt.Fprintf(&b, " USING MODEL '%s'", escapeString(denseModel))
	} else if denseName != "dense" && denseName != "" {
		fmt.Fprintf(&b, " VECTOR '%s'", escapeString(denseName))
	}

	config := info.GetConfig()
	if config == nil {
		return b.String()
	}
	params := config.GetParams()
	if params == nil {
		return b.String()
	}

	// VECTORS
	if vectorsCfg := params.GetVectorsConfig(); vectorsCfg != nil {
		if paramsMap := vectorsCfg.GetParamsMap(); paramsMap != nil {
			for _, vconfig := range paramsMap.GetMap() {
				if vconfig.OnDisk != nil && vconfig.GetOnDisk() {
					b.WriteString(" WITH VECTORS (on_disk = true)")
					break
				}
			}
		} else if single := vectorsCfg.GetParams(); single != nil {
			if single.OnDisk != nil && single.GetOnDisk() {
				b.WriteString(" WITH VECTORS (on_disk = true)")
			}
		}
	}

	// HNSW
	hnswConfig := config.GetHnswConfig()
	if hnswConfig != nil {
		var hnswParts []string
		addIfSet := func(name string, getter func() uint64) {
			val := getter()
			if val != 0 {
				hnswParts = append(hnswParts, fmt.Sprintf("%s = %d", name, val))
			}
		}
		addIfSet("m", hnswConfig.GetM)
		addIfSet("ef_construct", hnswConfig.GetEfConstruct)
		if hnswConfig.FullScanThreshold != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("full_scan_threshold = %d", hnswConfig.GetFullScanThreshold()))
		}
		if hnswConfig.MaxIndexingThreads != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("max_indexing_threads = %d", hnswConfig.GetMaxIndexingThreads()))
		}
		if hnswConfig.OnDisk != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("on_disk = %v", hnswConfig.GetOnDisk()))
		}
		if hnswConfig.PayloadM != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("payload_m = %d", hnswConfig.GetPayloadM()))
		}
		if hnswConfig.InlineStorage != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("inline_storage = %v", hnswConfig.GetInlineStorage()))
		}
		if len(hnswParts) > 0 {
			b.WriteString(" WITH HNSW (")
			b.WriteString(strings.Join(hnswParts, ", "))
			b.WriteString(")")
		}
	}

	// OPTIMIZERS
	optimizerConfig := config.GetOptimizerConfig()
	if optimizerConfig != nil {
		var optParts []string
		if optimizerConfig.DeletedThreshold != nil {
			optParts = append(optParts, fmt.Sprintf("deleted_threshold = %v", *optimizerConfig.DeletedThreshold))
		}
		if optimizerConfig.VacuumMinVectorNumber != nil {
			optParts = append(optParts, fmt.Sprintf("vacuum_min_vector_number = %d", *optimizerConfig.VacuumMinVectorNumber))
		}
		if optimizerConfig.DefaultSegmentNumber != nil {
			optParts = append(optParts, fmt.Sprintf("default_segment_number = %d", *optimizerConfig.DefaultSegmentNumber))
		}
		if optimizerConfig.MaxSegmentSize != nil {
			optParts = append(optParts, fmt.Sprintf("max_segment_size = %d", *optimizerConfig.MaxSegmentSize))
		}
		if optimizerConfig.MemmapThreshold != nil {
			optParts = append(optParts, fmt.Sprintf("memmap_threshold = %d", *optimizerConfig.MemmapThreshold))
		}
		if optimizerConfig.IndexingThreshold != nil {
			optParts = append(optParts, fmt.Sprintf("indexing_threshold = %d", *optimizerConfig.IndexingThreshold))
		}
		if optimizerConfig.FlushIntervalSec != nil {
			optParts = append(optParts, fmt.Sprintf("flush_interval_sec = %d", *optimizerConfig.FlushIntervalSec))
		}
		if optimizerConfig.MaxOptimizationThreads != nil {
			switch optimizerConfig.MaxOptimizationThreads.GetVariant().(type) {
			case *qdrant.MaxOptimizationThreads_Value:
				optParts = append(optParts, fmt.Sprintf("max_optimization_threads = %d", optimizerConfig.MaxOptimizationThreads.GetValue()))
			case *qdrant.MaxOptimizationThreads_Setting_:
				if optimizerConfig.MaxOptimizationThreads.GetSetting() == qdrant.MaxOptimizationThreads_Auto {
					optParts = append(optParts, "max_optimization_threads = 'auto'")
				}
			}
		}
		if optimizerConfig.PreventUnoptimized != nil {
			optParts = append(optParts, fmt.Sprintf("prevent_unoptimized = %v", *optimizerConfig.PreventUnoptimized))
		}
		if len(optParts) > 0 {
			b.WriteString(" WITH OPTIMIZERS (")
			b.WriteString(strings.Join(optParts, ", "))
			b.WriteString(")")
		}
	}

	// PARAMS
	var paramParts []string
	if rf := params.GetReplicationFactor(); rf != 0 {
		paramParts = append(paramParts, fmt.Sprintf("replication_factor = %d", rf))
	}
	if wcf := params.GetWriteConsistencyFactor(); wcf != 0 {
		paramParts = append(paramParts, fmt.Sprintf("write_consistency_factor = %d", wcf))
	}
	if params.GetOnDiskPayload() {
		paramParts = append(paramParts, "on_disk_payload = true")
	}
	if len(paramParts) > 0 {
		b.WriteString(" WITH PARAMS (")
		b.WriteString(strings.Join(paramParts, ", "))
		b.WriteString(")")
	}

	// Quantization
	if qc := config.GetQuantizationConfig(); qc != nil {
		switch qc.Quantization.(type) {
		case *qdrant.QuantizationConfig_Scalar:
			scalar := qc.GetScalar()
			b.WriteString(" WITH QUANTIZATION (type = 'scalar'")
			if scalar.Quantile != nil {
				fmt.Fprintf(&b, ", quantile = %v", *scalar.Quantile)
			}
			if scalar.GetAlwaysRam() {
				b.WriteString(", always_ram = true")
			}
			b.WriteString(")")
		case *qdrant.QuantizationConfig_Binary:
			binary := qc.GetBinary()
			b.WriteString(" WITH QUANTIZATION (type = 'binary'")
			if binary.GetAlwaysRam() {
				b.WriteString(", always_ram = true")
			}
			b.WriteString(")")
		case *qdrant.QuantizationConfig_Product:
			product := qc.GetProduct()
			b.WriteString(" WITH QUANTIZATION (type = 'product'")
			if product.GetAlwaysRam() {
				b.WriteString(", always_ram = true")
			}
			b.WriteString(")")
		case *qdrant.QuantizationConfig_Turboquant:
			turbo := qc.GetTurboquant()
			b.WriteString(" WITH QUANTIZATION (type = 'turbo'")
			if turbo.Bits != nil {
				fmt.Fprintf(&b, ", bits = %g", turboBitsValue(*turbo.Bits))
			}
			if turbo.GetAlwaysRam() {
				b.WriteString(", always_ram = true")
			}
			b.WriteString(")")
		}
	}

	return b.String()
}

// buildPayloadIndexStatements generates CREATE INDEX statements from the collection's PayloadSchema.
func buildPayloadIndexStatements(collection string, schema map[string]*qdrant.PayloadSchemaInfo) []string {
	if len(schema) == 0 {
		return nil
	}
	stmts := make([]string, 0, len(schema))
	for fieldName, idxInfo := range schema {
		if idxInfo.GetParams() == nil {
			continue
		}
		fieldType := payloadSchemaTypeToString(idxInfo.GetDataType())
		if fieldType == "" {
			continue
		}
		options := serializeIndexParams(idxInfo.GetParams())
		if len(options) > 0 {
			parts := make([]string, 0, len(options))
			for k, v := range options {
				parts = append(parts, fmt.Sprintf("%s = %s", k, serializeIndexValue(v)))
			}
			sort.Strings(parts)
			stmts = append(stmts, fmt.Sprintf("CREATE INDEX ON COLLECTION %s FOR %s TYPE %s WITH (%s)",
				collection, fieldName, fieldType, strings.Join(parts, ", ")))
		} else {
			stmts = append(stmts, fmt.Sprintf("CREATE INDEX ON COLLECTION %s FOR %s TYPE %s",
				collection, fieldName, fieldType))
		}
	}
	sort.Strings(stmts)
	return stmts
}

func payloadSchemaTypeToString(dt qdrant.PayloadSchemaType) string {
	switch dt {
	case qdrant.PayloadSchemaType_Keyword:
		return "keyword"
	case qdrant.PayloadSchemaType_Integer:
		return "integer"
	case qdrant.PayloadSchemaType_Float:
		return "float"
	case qdrant.PayloadSchemaType_Geo:
		return "geo"
	case qdrant.PayloadSchemaType_Text:
		return "text"
	case qdrant.PayloadSchemaType_Bool:
		return "bool"
	case qdrant.PayloadSchemaType_Datetime:
		return "datetime"
	case qdrant.PayloadSchemaType_Uuid:
		return "uuid"
	default:
		return ""
	}
}

func serializeIndexParams(params *qdrant.PayloadIndexParams) map[string]any {
	switch typed := params.GetIndexParams().(type) {
	case *qdrant.PayloadIndexParams_KeywordIndexParams:
		return serializeKeywordParams(typed.KeywordIndexParams)
	case *qdrant.PayloadIndexParams_TextIndexParams:
		return serializeTextParams(typed.TextIndexParams)
	case *qdrant.PayloadIndexParams_UuidIndexParams:
		return serializeUUIDParams(typed.UuidIndexParams)
	case *qdrant.PayloadIndexParams_IntegerIndexParams:
		return serializeIntegerParams(typed.IntegerIndexParams)
	default:
		return nil
	}
}

func serializeKeywordParams(params *qdrant.KeywordIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	return qdrantutil.SerializeKeywordBoolFields(params.IsTenant, params.OnDisk, params.EnableHnsw)
}

func serializeTextParams(params *qdrant.TextIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	data := map[string]any{}
	if params.Tokenizer != qdrant.TokenizerType_Unknown {
		data["tokenizer"] = strings.ToLower(strings.TrimPrefix(params.Tokenizer.String(), "TokenizerType_"))
	}
	if params.MinTokenLen != nil {
		data["min_token_len"] = params.GetMinTokenLen()
	}
	if params.MaxTokenLen != nil {
		data["max_token_len"] = params.GetMaxTokenLen()
	}
	if params.Lowercase != nil {
		data["lowercase"] = params.GetLowercase()
	}
	if params.AsciiFolding != nil {
		data["ascii_folding"] = params.GetAsciiFolding()
	}
	if params.PhraseMatching != nil {
		data["phrase_matching"] = params.GetPhraseMatching()
	}
	if params.OnDisk != nil {
		data["on_disk"] = params.GetOnDisk()
	}
	if params.EnableHnsw != nil {
		data["enable_hnsw"] = params.GetEnableHnsw()
	}
	return data
}

func serializeUUIDParams(params *qdrant.UuidIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	return qdrantutil.SerializeKeywordBoolFields(params.IsTenant, params.OnDisk, params.EnableHnsw)
}

func serializeIntegerParams(params *qdrant.IntegerIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	data := map[string]any{}
	if params.Lookup != nil {
		data["lookup"] = params.GetLookup()
	}
	if params.Range != nil {
		data["range"] = params.GetRange()
	}
	if params.IsPrincipal != nil {
		data["is_principal"] = params.GetIsPrincipal()
	}
	if params.OnDisk != nil {
		data["on_disk"] = params.GetOnDisk()
	}
	if params.EnableHnsw != nil {
		data["enable_hnsw"] = params.GetEnableHnsw()
	}
	return data
}

func serializeIndexValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case string:
		return "'" + escapeString(typed) + "'"
	case int, int64, uint64, float64:
		return fmt.Sprintf("%v", typed)
	default:
		return fmt.Sprintf("'%v'", value)
	}
}

func turboBitsValue(bits qdrant.TurboQuantBitSize) float64 {
	switch bits {
	case qdrant.TurboQuantBitSize_Bits1:
		return 1
	case qdrant.TurboQuantBitSize_Bits1_5:
		return 1.5
	case qdrant.TurboQuantBitSize_Bits2:
		return 2
	case qdrant.TurboQuantBitSize_Bits4:
		return 4
	default:
		return float64(bits)
	}
}

func indent(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for idx := range lines {
		lines[idx] = prefix + lines[idx]
	}
	return strings.Join(lines, "\n")
}

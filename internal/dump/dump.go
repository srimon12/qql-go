package dump

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

const batchSize = 50

type Client interface {
	CollectionExists(ctx context.Context, collectionName string) (bool, error)
	GetCollectionInfo(ctx context.Context, collectionName string) (*qdrant.CollectionInfo, error)
	Count(ctx context.Context, request *qdrant.CountPoints) (uint64, error)
	ScrollAndOffset(ctx context.Context, request *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
}

func Collection(ctx context.Context, client Client, collection, outputPath string, batchSize int) (int, int, error) {
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

	hybrid, err := isHybrid(ctx, client, collection)
	if err != nil {
		return 0, 0, err
	}
	total, err := client.Count(ctx, &qdrant.CountPoints{
		CollectionName: collection,
		Exact:          qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count points: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return 0, 0, fmt.Errorf("failed to prepare output directory: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("-- QQL dump for %s\n", collection))
	builder.WriteString(fmt.Sprintf("-- Points: %d\n\n", total))

	info, err := client.GetCollectionInfo(ctx, collection)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get collection info: %w", err)
	}
	createLine := buildDumpCreateLine(collection, hybrid, info)
	builder.WriteString(createLine)
	builder.WriteString("\n\n")

	written := 0
	skipped := 0
	var offset *qdrant.PointId
	for {
		points, nextOffset, err := client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: collection,
			Limit:          qdrant.PtrOf(uint32(batchSize)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(false),
		})
		if err != nil {
			return written, skipped, fmt.Errorf("failed to scroll collection: %w", err)
		}
		if len(points) == 0 {
			break
		}

		batch := make([]map[string]interface{}, 0, len(points))
		for _, point := range points {
			payload := point.GetPayload()
			textValue, ok := payload["text"]
			if !ok || textValue.GetStringValue() == "" {
				skipped++
				continue
			}
			record := payloadToMap(payload)
			record["id"] = pointIDValue(point.GetId())
			batch = append(batch, record)
		}

		if len(batch) > 0 {
			builder.WriteString(fmt.Sprintf("INSERT BULK INTO COLLECTION %s VALUES [\n", collection))
			for idx, record := range batch {
				builder.WriteString(indent(serializeMap(record), "  "))
				if idx+1 < len(batch) {
					builder.WriteString(",")
				}
				builder.WriteString("\n")
				written++
			}
			builder.WriteString("]")
			if hybrid {
				builder.WriteString(" USING HYBRID")
			}
			builder.WriteString("\n\n")
		}

		if nextOffset == nil {
			break
		}
		offset = nextOffset
	}

	builder.WriteString(fmt.Sprintf("-- Written: %d\n-- Skipped: %d\n", written, skipped))
	if err := os.WriteFile(outputPath, []byte(builder.String()), 0o644); err != nil {
		return written, skipped, fmt.Errorf("failed to write dump: %w", err)
	}
	return written, skipped, nil
}

func isHybrid(ctx context.Context, client Client, collection string) (bool, error) {
	info, err := client.GetCollectionInfo(ctx, collection)
	if err != nil {
		return false, fmt.Errorf("failed to inspect collection: %w", err)
	}
	return info.GetConfig().GetParams().GetSparseVectorsConfig() != nil, nil
}

func payloadToMap(payload map[string]*qdrant.Value) map[string]interface{} {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]interface{}, len(payload))
	for _, key := range keys {
		result[key] = payloadValue(payload[key])
	}
	return result
}

func payloadValue(value *qdrant.Value) interface{} {
	switch kind := value.GetKind().(type) {
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_BoolValue:
		return kind.BoolValue
	case *qdrant.Value_IntegerValue:
		return int(kind.IntegerValue)
	case *qdrant.Value_DoubleValue:
		return kind.DoubleValue
	case *qdrant.Value_StringValue:
		return kind.StringValue
	case *qdrant.Value_ListValue:
		items := make([]interface{}, 0, len(kind.ListValue.GetValues()))
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

func pointIDValue(id *qdrant.PointId) interface{} {
	if id == nil {
		return ""
	}
	switch value := id.GetPointIdOptions().(type) {
	case *qdrant.PointId_Uuid:
		return value.Uuid
	case *qdrant.PointId_Num:
		return int(value.Num)
	default:
		return fmt.Sprintf("%v", id)
	}
}

func serializeMap(values map[string]interface{}) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("{\n")
	for idx, key := range keys {
		builder.WriteString(fmt.Sprintf("  '%s': %s", escapeString(key), serializeValue(values[key])))
		if idx+1 < len(keys) {
			builder.WriteString(",")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("}")
	return builder.String()
}

func serializeValue(value interface{}) string {
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
	case float64:
		return fmt.Sprintf("%v", typed)
	case string:
		return "'" + escapeString(typed) + "'"
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, serializeValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		return serializeMap(typed)
	default:
		return "'" + escapeString(fmt.Sprintf("%v", value)) + "'"
	}
}

func escapeString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "'", "\\'")
	return value
}

func buildDumpCreateLine(collection string, hybrid bool, info *qdrant.CollectionInfo) string {
	var b strings.Builder
	b.WriteString("CREATE COLLECTION ")
	b.WriteString(collection)
	if hybrid {
		b.WriteString(" HYBRID")
	}

	config := info.GetConfig()
	params := config.GetParams()

	// VECTORS
	if vectorsCfg := params.GetVectorsConfig(); vectorsCfg != nil {
		if paramsMap := vectorsCfg.GetParamsMap(); paramsMap != nil {
			for _, vconfig := range paramsMap.GetMap() {
				if vconfig.OnDisk != nil && vconfig.GetOnDisk() {
					b.WriteString(" WITH VECTORS { on_disk: true }")
					break
				}
			}
		} else if single := vectorsCfg.GetParams(); single != nil {
			if single.OnDisk != nil && single.GetOnDisk() {
				b.WriteString(" WITH VECTORS { on_disk: true }")
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
				hnswParts = append(hnswParts, fmt.Sprintf("%s: %d", name, val))
			}
		}
		addIfSet("m", hnswConfig.GetM)
		addIfSet("ef_construct", hnswConfig.GetEfConstruct)
		if hnswConfig.FullScanThreshold != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("full_scan_threshold: %d", hnswConfig.GetFullScanThreshold()))
		}
		if hnswConfig.MaxIndexingThreads != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("max_indexing_threads: %d", hnswConfig.GetMaxIndexingThreads()))
		}
		if hnswConfig.OnDisk != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("on_disk: %v", hnswConfig.GetOnDisk()))
		}
		if hnswConfig.PayloadM != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("payload_m: %d", hnswConfig.GetPayloadM()))
		}
		if hnswConfig.InlineStorage != nil {
			hnswParts = append(hnswParts, fmt.Sprintf("inline_storage: %v", hnswConfig.GetInlineStorage()))
		}
		if len(hnswParts) > 0 {
			b.WriteString(" WITH HNSW { ")
			b.WriteString(strings.Join(hnswParts, ", "))
			b.WriteString(" }")
		}
	}

	// OPTIMIZERS
	optimizerConfig := config.GetOptimizerConfig()
	if optimizerConfig != nil {
		var optParts []string
		if optimizerConfig.DeletedThreshold != nil {
			optParts = append(optParts, fmt.Sprintf("deleted_threshold: %v", *optimizerConfig.DeletedThreshold))
		}
		if optimizerConfig.VacuumMinVectorNumber != nil {
			optParts = append(optParts, fmt.Sprintf("vacuum_min_vector_number: %d", *optimizerConfig.VacuumMinVectorNumber))
		}
		if optimizerConfig.DefaultSegmentNumber != nil {
			optParts = append(optParts, fmt.Sprintf("default_segment_number: %d", *optimizerConfig.DefaultSegmentNumber))
		}
		if optimizerConfig.MaxSegmentSize != nil {
			optParts = append(optParts, fmt.Sprintf("max_segment_size: %d", *optimizerConfig.MaxSegmentSize))
		}
		if optimizerConfig.MemmapThreshold != nil {
			optParts = append(optParts, fmt.Sprintf("memmap_threshold: %d", *optimizerConfig.MemmapThreshold))
		}
		if optimizerConfig.IndexingThreshold != nil {
			optParts = append(optParts, fmt.Sprintf("indexing_threshold: %d", *optimizerConfig.IndexingThreshold))
		}
		if optimizerConfig.FlushIntervalSec != nil {
			optParts = append(optParts, fmt.Sprintf("flush_interval_sec: %d", *optimizerConfig.FlushIntervalSec))
		}
		if optimizerConfig.PreventUnoptimized != nil {
			optParts = append(optParts, fmt.Sprintf("prevent_unoptimized: %v", *optimizerConfig.PreventUnoptimized))
		}
		if len(optParts) > 0 {
			b.WriteString(" WITH OPTIMIZERS { ")
			b.WriteString(strings.Join(optParts, ", "))
			b.WriteString(" }")
		}
	}

	// PARAMS
	if params.GetReplicationFactor() != 0 || params.GetWriteConsistencyFactor() != 0 {
		var paramParts []string
		if rf := params.GetReplicationFactor(); rf != 0 {
			paramParts = append(paramParts, fmt.Sprintf("replication_factor: %d", rf))
		}
		if wcf := params.GetWriteConsistencyFactor(); wcf != 0 {
			paramParts = append(paramParts, fmt.Sprintf("write_consistency_factor: %d", wcf))
		}
		if params.GetOnDiskPayload() {
			paramParts = append(paramParts, "on_disk_payload: true")
		}
		if len(paramParts) > 0 {
			b.WriteString(" WITH PARAMS { ")
			b.WriteString(strings.Join(paramParts, ", "))
			b.WriteString(" }")
		}
	}

	// Quantization
	if qc := config.GetQuantizationConfig(); qc != nil {
		switch qc.Quantization.(type) {
		case *qdrant.QuantizationConfig_Scalar:
			scalar := qc.GetScalar()
			b.WriteString(" QUANTIZE SCALAR")
			if scalar.Quantile != nil {
				b.WriteString(fmt.Sprintf(" QUANTILE %v", *scalar.Quantile))
			}
			if scalar.GetAlwaysRam() {
				b.WriteString(" ALWAYS RAM")
			}
		case *qdrant.QuantizationConfig_Binary:
			binary := qc.GetBinary()
			b.WriteString(" QUANTIZE BINARY")
			if binary.GetAlwaysRam() {
				b.WriteString(" ALWAYS RAM")
			}
		case *qdrant.QuantizationConfig_Product:
			product := qc.GetProduct()
			b.WriteString(" QUANTIZE PRODUCT")
			if product.GetAlwaysRam() {
				b.WriteString(" ALWAYS RAM")
			}
		case *qdrant.QuantizationConfig_Turboquant:
			turbo := qc.GetTurboquant()
			b.WriteString(" QUANTIZE TURBO")
			if turbo.Bits != nil {
				b.WriteString(fmt.Sprintf(" BITS %v", turbo.Bits))
			}
			if turbo.GetAlwaysRam() {
				b.WriteString(" ALWAYS RAM")
			}
		}
	}

	return b.String()
}

func indent(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for idx := range lines {
		lines[idx] = prefix + lines[idx]
	}
	return strings.Join(lines, "\n")
}

package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/ast"
)

func (e *Executor) doCreateIndex(n *ast.CreateIndexStmt) (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()

	fieldType, err := parseFieldType(n.FieldType)
	if err != nil {
		return nil, err
	}

	fieldIndexParams, err := buildPayloadIndexParams(n.FieldType, n.Options)
	if err != nil {
		return nil, err
	}
	wait := true
	_, err = e.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName:   n.Collection,
		FieldName:        n.Field,
		FieldType:        &fieldType,
		FieldIndexParams: fieldIndexParams,
		Wait:             &wait,
		Timeout:          e.requestTimeout(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}
	return &ExecResponse{
		OK:        true,
		Operation: "create_index",
		Message:   fmt.Sprintf("Index created on '%s.%s'", n.Collection, n.Field),
		Data: map[string]any{
			"collection": n.Collection,
			"field":      n.Field,
			"field_type": n.FieldType,
		},
	}, nil
}

func (e *Executor) doShowCollections() (*ExecResponse, error) {
	ctx, cancel := e.defaultContext()
	defer cancel()
	names, err := e.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collections: %w", err)
	}

	if len(names) == 0 {
		return &ExecResponse{
			OK:        true,
			Operation: "show_collections",
			Message:   "No collections found",
			Data: map[string]any{
				"count":       0,
				"collections": []string{},
			},
		}, nil
	}

	return &ExecResponse{
		OK:        true,
		Operation: "show_collections",
		Message:   fmt.Sprintf("Found %d collection(s): %s", len(names), strings.Join(names, ", ")),
		Data: map[string]any{
			"count":       len(names),
			"collections": names,
		},
	}, nil
}

func buildQuantizationConfig(cfg *ast.QuantizationConfig) (*qdrant.QuantizationConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	switch cfg.Type {
	case ast.QuantizationTypeScalar:
		scalar := &qdrant.ScalarQuantization{
			Type:      qdrant.QuantizationType_Int8,
			AlwaysRam: qdrant.PtrOf(cfg.AlwaysRAM),
		}
		if cfg.Quantile != nil {
			scalar.Quantile = qdrant.PtrOf(float32(*cfg.Quantile))
		}
		return qdrant.NewQuantizationScalar(scalar), nil
	case ast.QuantizationTypeBinary:
		return qdrant.NewQuantizationBinary(&qdrant.BinaryQuantization{
			AlwaysRam: qdrant.PtrOf(cfg.AlwaysRAM),
		}), nil
	case ast.QuantizationTypeProduct:
		return qdrant.NewQuantizationProduct(&qdrant.ProductQuantization{
			Compression: qdrant.CompressionRatio_x4,
			AlwaysRam:   qdrant.PtrOf(cfg.AlwaysRAM),
		}), nil
	case ast.QuantizationTypeTurbo:
		turbo := &qdrant.TurboQuantization{
			AlwaysRam: qdrant.PtrOf(cfg.AlwaysRAM),
		}
		if cfg.TurboBits != nil {
			bits := turboBitsEnum(*cfg.TurboBits)
			if bits == nil {
				return nil, fmt.Errorf("unsupported TURBO bit depth %.4g; expected one of 1, 1.5, 2, or 4", *cfg.TurboBits)
			}
			turbo.Bits = bits
		}
		return qdrant.NewQuantizationTurbo(turbo), nil
	default:
		return nil, nil
	}
}

func buildHnswConfigDiff(cfg *ast.HnswRuntimeConfig) *qdrant.HnswConfigDiff {
	if cfg == nil {
		return nil
	}
	diff := &qdrant.HnswConfigDiff{}
	if cfg.M != nil {
		diff.M = cfg.M
	}
	if cfg.EfConstruct != nil {
		diff.EfConstruct = cfg.EfConstruct
	}
	if cfg.FullScanThreshold != nil {
		diff.FullScanThreshold = cfg.FullScanThreshold
	}
	if cfg.MaxIndexingThreads != nil {
		diff.MaxIndexingThreads = cfg.MaxIndexingThreads
	}
	if cfg.OnDisk != nil {
		diff.OnDisk = cfg.OnDisk
	}
	if cfg.PayloadM != nil {
		diff.PayloadM = cfg.PayloadM
	}
	if cfg.InlineStorage != nil {
		diff.InlineStorage = cfg.InlineStorage
	}
	return diff
}

func buildOptimizersConfigDiff(cfg *ast.OptimizersRuntimeConfig) *qdrant.OptimizersConfigDiff {
	if cfg == nil {
		return nil
	}
	diff := &qdrant.OptimizersConfigDiff{}
	if cfg.DeletedThreshold != nil {
		diff.DeletedThreshold = cfg.DeletedThreshold
	}
	if cfg.VacuumMinVectorNumber != nil {
		diff.VacuumMinVectorNumber = cfg.VacuumMinVectorNumber
	}
	if cfg.DefaultSegmentNumber != nil {
		diff.DefaultSegmentNumber = cfg.DefaultSegmentNumber
	}
	if cfg.MaxSegmentSize != nil {
		diff.MaxSegmentSize = cfg.MaxSegmentSize
	}
	if cfg.MemmapThreshold != nil {
		diff.MemmapThreshold = cfg.MemmapThreshold
	}
	if cfg.IndexingThreshold != nil {
		diff.IndexingThreshold = cfg.IndexingThreshold
	}
	if cfg.FlushIntervalSec != nil {
		diff.FlushIntervalSec = cfg.FlushIntervalSec
	}
	if cfg.PreventUnoptimized != nil {
		diff.PreventUnoptimized = cfg.PreventUnoptimized
	}
	if cfg.MaxOptimizationThreads != nil {
		if cfg.MaxOptimizationThreads.Auto {
			diff.MaxOptimizationThreads = &qdrant.MaxOptimizationThreads{
				Variant: &qdrant.MaxOptimizationThreads_Setting_{
					Setting: qdrant.MaxOptimizationThreads_Auto,
				},
			}
		} else {
			diff.MaxOptimizationThreads = &qdrant.MaxOptimizationThreads{
				Variant: &qdrant.MaxOptimizationThreads_Value{
					Value: cfg.MaxOptimizationThreads.Value,
				},
			}
		}
	}
	return diff
}

func applyCollectionParamsCreate(cfg *ast.CollectionParamsConfig, req *qdrant.CreateCollection) {
	if cfg.ReplicationFactor != nil {
		req.ReplicationFactor = qdrant.PtrOf(uint32(*cfg.ReplicationFactor))
	}
	if cfg.WriteConsistencyFactor != nil {
		req.WriteConsistencyFactor = qdrant.PtrOf(uint32(*cfg.WriteConsistencyFactor))
	}
	if cfg.OnDiskPayload != nil {
		req.OnDiskPayload = cfg.OnDiskPayload
	}
}

func buildCollectionParamsDiff(cfg *ast.CollectionParamsConfig) *qdrant.CollectionParamsDiff {
	if cfg == nil {
		return nil
	}
	diff := &qdrant.CollectionParamsDiff{}
	if cfg.ReplicationFactor != nil {
		diff.ReplicationFactor = qdrant.PtrOf(uint32(*cfg.ReplicationFactor))
	}
	if cfg.WriteConsistencyFactor != nil {
		diff.WriteConsistencyFactor = qdrant.PtrOf(uint32(*cfg.WriteConsistencyFactor))
	}
	if cfg.ReadFanOutFactor != nil {
		diff.ReadFanOutFactor = qdrant.PtrOf(uint32(*cfg.ReadFanOutFactor))
	}
	if cfg.ReadFanOutDelayMs != nil {
		diff.ReadFanOutDelayMs = cfg.ReadFanOutDelayMs
	}
	if cfg.OnDiskPayload != nil {
		diff.OnDiskPayload = cfg.OnDiskPayload
	}
	return diff
}

func buildAlterQuantizationConfig(update *ast.QuantizationUpdate) (*qdrant.QuantizationConfigDiff, error) {
	if update == nil {
		return nil, nil
	}
	if update.Disabled {
		return &qdrant.QuantizationConfigDiff{
			Quantization: &qdrant.QuantizationConfigDiff_Disabled{
				Disabled: &qdrant.Disabled{},
			},
		}, nil
	}
	if update.Config != nil {
		cfg, err := buildQuantizationConfig(update.Config)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			switch cfg.Quantization.(type) {
			case *qdrant.QuantizationConfig_Scalar:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Scalar{
						Scalar: cfg.GetScalar(),
					},
				}, nil
			case *qdrant.QuantizationConfig_Binary:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Binary{
						Binary: cfg.GetBinary(),
					},
				}, nil
			case *qdrant.QuantizationConfig_Product:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Product{
						Product: cfg.GetProduct(),
					},
				}, nil
			case *qdrant.QuantizationConfig_Turboquant:
				return &qdrant.QuantizationConfigDiff{
					Quantization: &qdrant.QuantizationConfigDiff_Turboquant{
						Turboquant: cfg.GetTurboquant(),
					},
				}, nil
			}
		}
	}
	return nil, nil
}

func buildPayloadIndexParams(fieldType string, options map[string]any) (*qdrant.PayloadIndexParams, error) {
	if len(options) == 0 {
		return nil, nil
	}

	switch fieldType {
	case "keyword":
		if err := validateIndexOptionKeys(fieldType, options, []string{"is_tenant", "on_disk", "enable_hnsw"}); err != nil {
			return nil, err
		}
		isTenant, err := boolOption(options, "is_tenant")
		if err != nil {
			return nil, err
		}
		onDisk, err := boolOption(options, "on_disk")
		if err != nil {
			return nil, err
		}
		enableHnsw, err := boolOption(options, "enable_hnsw")
		if err != nil {
			return nil, err
		}
		return qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{
			IsTenant:   isTenant,
			OnDisk:     onDisk,
			EnableHnsw: enableHnsw,
		}), nil
	case "uuid":
		if err := validateIndexOptionKeys(fieldType, options, []string{"is_tenant", "on_disk", "enable_hnsw"}); err != nil {
			return nil, err
		}
		isTenant, err := boolOption(options, "is_tenant")
		if err != nil {
			return nil, err
		}
		onDisk, err := boolOption(options, "on_disk")
		if err != nil {
			return nil, err
		}
		enableHnsw, err := boolOption(options, "enable_hnsw")
		if err != nil {
			return nil, err
		}
		return qdrant.NewPayloadIndexParamsUUID(&qdrant.UuidIndexParams{
			IsTenant:   isTenant,
			OnDisk:     onDisk,
			EnableHnsw: enableHnsw,
		}), nil
	case "text":
		if err := validateIndexOptionKeys(fieldType, options, []string{"tokenizer", "min_token_len", "max_token_len", "lowercase", "ascii_folding", "phrase_matching", "stopwords", "on_disk", "enable_hnsw"}); err != nil {
			return nil, err
		}
		minTokenLen, err := uint64Option(options, "min_token_len")
		if err != nil {
			return nil, err
		}
		maxTokenLen, err := uint64Option(options, "max_token_len")
		if err != nil {
			return nil, err
		}
		if minTokenLen != nil && maxTokenLen != nil && *minTokenLen > *maxTokenLen {
			return nil, fmt.Errorf("CREATE INDEX text option min_token_len cannot be greater than max_token_len")
		}
		tokenizer, err := tokenizerOption(options, "tokenizer")
		if err != nil {
			return nil, err
		}
		stopwords, err := stopwordsOption(options, "stopwords")
		if err != nil {
			return nil, err
		}
		lowercase, err := boolOption(options, "lowercase")
		if err != nil {
			return nil, err
		}
		asciiFolding, err := boolOption(options, "ascii_folding")
		if err != nil {
			return nil, err
		}
		phraseMatching, err := boolOption(options, "phrase_matching")
		if err != nil {
			return nil, err
		}
		onDisk, err := boolOption(options, "on_disk")
		if err != nil {
			return nil, err
		}
		enableHnsw, err := boolOption(options, "enable_hnsw")
		if err != nil {
			return nil, err
		}
		return qdrant.NewPayloadIndexParamsText(&qdrant.TextIndexParams{
			Tokenizer:      tokenizer,
			MinTokenLen:    minTokenLen,
			MaxTokenLen:    maxTokenLen,
			Lowercase:      lowercase,
			AsciiFolding:   asciiFolding,
			PhraseMatching: phraseMatching,
			Stopwords:      stopwords,
			OnDisk:         onDisk,
			EnableHnsw:     enableHnsw,
		}), nil
	default:
		return nil, fmt.Errorf("CREATE INDEX type '%s' does not support advanced options yet", fieldType)
	}
}

func validateIndexOptionKeys(fieldType string, options map[string]any, allowed []string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range options {
		if _, ok := allowedSet[key]; !ok {
			return fmt.Errorf("Unknown CREATE INDEX option '%s' for type '%s'. Expected one of: %s", key, fieldType, strings.Join(allowed, ", "))
		}
	}
	return nil
}

func boolOption(options map[string]any, key string) (*bool, error) {
	value, ok := options[key]
	if !ok {
		return nil, nil
	}
	typed, ok := value.(bool)
	if !ok {
		return nil, fmt.Errorf("CREATE INDEX option '%s' must be a boolean", key)
	}
	return qdrant.PtrOf(typed), nil
}

func uint64Option(options map[string]any, key string) (*uint64, error) {
	value, ok := options[key]
	if !ok {
		return nil, nil
	}
	typed, ok := value.(int)
	if !ok || typed <= 0 {
		return nil, fmt.Errorf("CREATE INDEX option '%s' must be a positive integer", key)
	}
	result := uint64(typed)
	return &result, nil
}

func tokenizerOption(options map[string]any, key string) (qdrant.TokenizerType, error) {
	value, ok := options[key]
	if !ok {
		return qdrant.TokenizerType_Unknown, nil
	}
	typed, ok := value.(string)
	if !ok {
		return qdrant.TokenizerType_Unknown, fmt.Errorf("CREATE INDEX option '%s' must be a string", key)
	}
	switch strings.ToLower(typed) {
	case "prefix":
		return qdrant.TokenizerType_Prefix, nil
	case "whitespace":
		return qdrant.TokenizerType_Whitespace, nil
	case "word":
		return qdrant.TokenizerType_Word, nil
	case "multilingual":
		return qdrant.TokenizerType_Multilingual, nil
	default:
		return qdrant.TokenizerType_Unknown, fmt.Errorf("CREATE INDEX option '%s' must be one of: prefix, whitespace, word, multilingual", key)
	}
}

func stopwordsOption(options map[string]any, key string) (*qdrant.StopwordsSet, error) {
	value, ok := options[key]
	if !ok {
		return nil, nil
	}
	switch typed := value.(type) {
	case string:
		return &qdrant.StopwordsSet{Languages: []string{strings.ToLower(typed)}}, nil
	case []any:
		words := make([]string, 0, len(typed))
		for _, item := range typed {
			word, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("CREATE INDEX option '%s' list values must be strings", key)
			}
			words = append(words, word)
		}
		return &qdrant.StopwordsSet{Custom: words}, nil
	default:
		return nil, fmt.Errorf("CREATE INDEX option '%s' must be a string language name or a list of strings", key)
	}
}

func (e *Executor) waitForCollectionReady(ctx context.Context, collection string) error {
	return waitForCollectionReady(ctx, collection, collectionReadyTimeout, collectionReadyInterval, e.collectionReady)
}

func waitForCollectionReady(
	ctx context.Context,
	collection string,
	timeout time.Duration,
	interval time.Duration,
	ready func(context.Context, string) (bool, bool, error),
) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		exists, readyNow, err := ready(waitCtx, collection)
		if err == nil && readyNow {
			return nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			if err != nil {
				return fmt.Errorf("collection '%s' did not become ready within %s: %w", collection, timeout, err)
			}
			if exists {
				return fmt.Errorf("collection '%s' exists but is not ready yet after %s", collection, timeout)
			}
			return fmt.Errorf("collection '%s' did not become visible within %s", collection, timeout)
		case <-timer.C:
		}
	}
}

func parseFieldType(s string) (qdrant.FieldType, error) {
	switch s {
	case "keyword":
		return qdrant.FieldType_FieldTypeKeyword, nil
	case "integer":
		return qdrant.FieldType_FieldTypeInteger, nil
	case "float":
		return qdrant.FieldType_FieldTypeFloat, nil
	case "bool":
		return qdrant.FieldType_FieldTypeBool, nil
	case "text":
		return qdrant.FieldType_FieldTypeText, nil
	case "geo":
		return qdrant.FieldType_FieldTypeGeo, nil
	case "datetime":
		return qdrant.FieldType_FieldTypeDatetime, nil
	case "uuid":
		return qdrant.FieldType_FieldTypeUuid, nil
	default:
		return 0, fmt.Errorf("unknown field type '%s'; expected one of: keyword, integer, float, bool, text, geo, datetime, uuid", s)
	}
}

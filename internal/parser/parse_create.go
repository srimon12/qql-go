package parser

import (
	"sort"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/errors"
	"github.com/srimon12/qql-go/internal/lexer"
)

func (p *Parser) parseCreate() (ast.ASTNode, error) {
	p.advance()
	tok := p.peek()
	if tok.Kind == lexer.TokenKindIndex {
		return p.parseCreateIndex()
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	hybrid := false
	rerank := false
	var model *string
	var denseVector *string
	var sparseVector *string
	if p.peek().Kind == lexer.TokenKindHybrid {
		p.advance()
		hybrid = true
		if p.peek().Kind == lexer.TokenKindRerank {
			p.advance()
			rerank = true
		} else {
			var err error
			for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse {
				mode := p.advance().Kind
				if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && toUpper(p.peek().Value) == "VECTOR") {
					p.advance()
					v, err2 := p.parseStringPtr()
					if err2 != nil {
						return nil, err2
					}
					if mode == lexer.TokenKindDense {
						denseVector = v
					} else {
						sparseVector = v
					}
				} else {
					return nil, errors.NewQQLSyntaxError("Expected VECTOR after DENSE/SPARSE", p.peek().Pos)
				}
			}
			err = nil // suppress unused
			_ = err
		}
	} else if p.peek().Kind == lexer.TokenKindUsing {
		// Old qql-go specific path
		p.advance()
		if p.peek().Kind == lexer.TokenKindHybrid {
			p.advance()
			hybrid = true
			if p.peek().Kind == lexer.TokenKindDense {
				p.advance()
				var err error
				model, err = p.parseRequiredModelString()
				if err != nil {
					return nil, err
				}
			}
		} else {
			var err error
			model, err = p.parseRequiredModelString()
			if err != nil {
				return nil, err
			}
		}
	}

	config, err := p.parseCollectionConfigBlocks(false)
	if err != nil {
		return nil, err
	}
	quantization, err := p.parseOptionalCreateQuantization()
	if err != nil {
		return nil, err
	}

	return &ast.CreateCollectionStmt{
		Collection:   collection,
		Hybrid:       hybrid,
		Rerank:       rerank,
		Model:        model,
		Quantization: quantization,
		Config:       config,
		DenseVector:  denseVector,
		SparseVector: sparseVector,
	}, nil
}

func (p *Parser) parseCollectionConfigBlocks(forAlter bool) (*ast.CollectionConfig, error) {
	var config *ast.CollectionConfig
	for p.peek().Kind == lexer.TokenKindWith {
		p.advance()
		block, err := p.parseCollectionConfigClause(forAlter)
		if err != nil {
			return nil, err
		}
		if config == nil {
			config = block
		} else {
			config, err = mergeCollectionConfig(config, block, p.peek().Pos)
			if err != nil {
				return nil, err
			}
		}
	}
	return config, nil
}

func (p *Parser) parseOptionalCreateQuantization() (*ast.QuantizationConfig, error) {
	if p.peek().Kind != lexer.TokenKindQuantize {
		return nil, nil
	}
	p.advance()
	return p.parseQuantizeClause()
}

func (p *Parser) parseOptionalAlterQuantization() (*ast.QuantizationUpdate, error) {
	if p.peek().Kind != lexer.TokenKindQuantize {
		return nil, nil
	}
	p.advance()
	if p.peek().Kind == lexer.TokenKindDisabled {
		p.advance()
		return &ast.QuantizationUpdate{Disabled: true}, nil
	}
	config, err := p.parseQuantizeClause()
	if err != nil {
		return nil, err
	}
	return &ast.QuantizationUpdate{Config: config}, nil
}

func mergeCollectionConfig(current, new *ast.CollectionConfig, pos int) (*ast.CollectionConfig, error) {
	if new.Vectors != nil {
		if current.Vectors != nil {
			return nil, errors.NewQQLSyntaxError("VECTORS clause may only appear once", pos)
		}
		current.Vectors = new.Vectors
	}
	if new.Hnsw != nil {
		if current.Hnsw != nil {
			return nil, errors.NewQQLSyntaxError("HNSW clause may only appear once", pos)
		}
		current.Hnsw = new.Hnsw
	}
	if new.Optimizers != nil {
		if current.Optimizers != nil {
			return nil, errors.NewQQLSyntaxError("OPTIMIZERS clause may only appear once", pos)
		}
		current.Optimizers = new.Optimizers
	}
	if new.Params != nil {
		if current.Params != nil {
			return nil, errors.NewQQLSyntaxError("PARAMS clause may only appear once", pos)
		}
		current.Params = new.Params
	}
	return current, nil
}

func (p *Parser) parseCollectionConfigClause(forAlter bool) (*ast.CollectionConfig, error) {
	tok := p.peek()
	if tok.Kind == lexer.TokenKindHnsw {
		p.advance()
		return p.parseHnswConfigBlock()
	}
	if tok.Kind == lexer.TokenKindVectors {
		p.advance()
		return p.parseVectorsConfigBlock()
	}
	if tok.Kind == lexer.TokenKindOptimizers {
		p.advance()
		return p.parseOptimizersConfigBlock()
	}
	if tok.Kind == lexer.TokenKindParams {
		p.advance()
		return p.parseCollectionParamsConfigBlock(forAlter)
	}
	return nil, errors.NewQQLSyntaxError(
		"Expected HNSW, VECTORS, OPTIMIZERS, or PARAMS after WITH, got '"+tok.Value+"'",
		tok.Pos,
	)
}

func (p *Parser) parseHnswConfigBlock() (*ast.CollectionConfig, error) {
	config, err := p.parseDict()
	if err != nil {
		return nil, err
	}
	for key := range config {
		lower := toLower(key)
		switch lower {
		case "m", "ef_construct", "full_scan_threshold", "max_indexing_threads", "on_disk", "payload_m", "inline_storage":
			continue
		default:
			return nil, errors.NewQQLSyntaxError("Unknown HNSW parameter '"+key+"'. Expected: m, ef_construct, full_scan_threshold, max_indexing_threads, on_disk, payload_m, inline_storage", p.peek().Pos)
		}
	}
	for key, raw := range config {
		if err := p.validateHnswValue(key, raw); err != nil {
			return nil, err
		}
	}
	mVal, err := collectionPositiveUint64(config, "m", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	if mVal != nil && *mVal < 4 {
		return nil, errors.NewQQLSyntaxError("m must be >= 4", p.peek().Pos)
	}
	efConstruct, err := collectionPositiveUint64(config, "ef_construct", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	fullScanThreshold, err := collectionNonNegativeUint64(config, "full_scan_threshold", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	maxIndexingThreads, err := collectionPositiveUint64(config, "max_indexing_threads", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	payloadM, err := collectionPositiveUint64(config, "payload_m", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	return &ast.CollectionConfig{
		Hnsw: &ast.HnswRuntimeConfig{
			M:                  mVal,
			EfConstruct:        efConstruct,
			FullScanThreshold:  fullScanThreshold,
			MaxIndexingThreads: maxIndexingThreads,
			OnDisk:             collectionBool(config, "on_disk"),
			PayloadM:           payloadM,
			InlineStorage:      collectionBool(config, "inline_storage"),
		},
	}, nil
}

func (p *Parser) parseVectorsConfigBlock() (*ast.CollectionConfig, error) {
	config, err := p.parseDict()
	if err != nil {
		return nil, err
	}
	for key := range config {
		if toLower(key) != "on_disk" {
			return nil, errors.NewQQLSyntaxError("Unknown VECTORS parameter '"+key+"'. Expected: on_disk", p.peek().Pos)
		}
	}
	for key, raw := range config {
		if err := p.validateVectorsValue(key, raw); err != nil {
			return nil, err
		}
	}
	return &ast.CollectionConfig{
		Vectors: &ast.VectorsConfig{
			OnDisk: collectionBool(config, "on_disk"),
		},
	}, nil
}

func (p *Parser) parseOptimizersConfigBlock() (*ast.CollectionConfig, error) {
	config, err := p.parseDict()
	if err != nil {
		return nil, err
	}
	for key := range config {
		lower := toLower(key)
		switch lower {
		case "deleted_threshold", "vacuum_min_vector_number", "default_segment_number", "max_segment_size", "memmap_threshold", "indexing_threshold", "flush_interval_sec", "max_optimization_threads", "prevent_unoptimized":
			continue
		default:
			return nil, errors.NewQQLSyntaxError("Unknown OPTIMIZERS parameter '"+key+"'. Expected: deleted_threshold, vacuum_min_vector_number, default_segment_number, max_segment_size, memmap_threshold, indexing_threshold, flush_interval_sec, max_optimization_threads, prevent_unoptimized", p.peek().Pos)
		}
	}
	for key, raw := range config {
		if err := p.validateOptimizersValue(key, raw); err != nil {
			return nil, err
		}
	}
	for key, raw := range config {
		lower := toLower(key)
		if lower == "deleted_threshold" {
			switch v := raw.(type) {
			case int:
				f := float64(v)
				if f < 0.0 || f > 1.0 {
					return nil, errors.NewQQLSyntaxError("deleted_threshold must be between 0.0 and 1.0", p.peek().Pos)
				}
			case float64:
				if v < 0.0 || v > 1.0 {
					return nil, errors.NewQQLSyntaxError("deleted_threshold must be between 0.0 and 1.0", p.peek().Pos)
				}
			}
		}
		if lower == "max_optimization_threads" {
			switch v := raw.(type) {
			case int:
				if v <= 0 {
					return nil, errors.NewQQLSyntaxError("max_optimization_threads must be a positive integer or 'auto'", p.peek().Pos)
				}
			case string:
				if toLower(v) != "auto" {
					return nil, errors.NewQQLSyntaxError("max_optimization_threads must be a positive integer or 'auto'", p.peek().Pos)
				}
			}
		}
	}
	vacuumMinVectorNumber, err := collectionPositiveUint64(config, "vacuum_min_vector_number", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	defaultSegmentNumber, err := collectionPositiveUint64(config, "default_segment_number", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	maxSegmentSize, err := collectionPositiveUint64(config, "max_segment_size", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	flushIntervalSec, err := collectionPositiveUint64(config, "flush_interval_sec", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	memmapThreshold, err := collectionNonNegativeUint64(config, "memmap_threshold", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	indexingThreshold, err := collectionNonNegativeUint64(config, "indexing_threshold", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	return &ast.CollectionConfig{
		Optimizers: &ast.OptimizersRuntimeConfig{
			DeletedThreshold:       collectionFloatRange(config, "deleted_threshold", 0.0, 1.0),
			VacuumMinVectorNumber:  vacuumMinVectorNumber,
			DefaultSegmentNumber:   defaultSegmentNumber,
			MaxSegmentSize:         maxSegmentSize,
			MemmapThreshold:        memmapThreshold,
			IndexingThreshold:      indexingThreshold,
			FlushIntervalSec:       flushIntervalSec,
			MaxOptimizationThreads: collectionMaxOptimizationThreads(config, "max_optimization_threads"),
			PreventUnoptimized:     collectionBool(config, "prevent_unoptimized"),
		},
	}, nil
}

func (p *Parser) parseCollectionParamsConfigBlock(forAlter bool) (*ast.CollectionConfig, error) {
	config, err := p.parseDict()
	if err != nil {
		return nil, err
	}
	for key := range config {
		lower := toLower(key)
		switch lower {
		case "replication_factor", "write_consistency_factor", "read_fan_out_factor", "read_fan_out_delay_ms", "on_disk_payload":
			continue
		default:
			return nil, errors.NewQQLSyntaxError("Unknown PARAMS parameter '"+key+"'. Expected: replication_factor, write_consistency_factor, read_fan_out_factor, read_fan_out_delay_ms, on_disk_payload", p.peek().Pos)
		}
	}
	for key, raw := range config {
		if err := p.validateParamsValue(key, raw); err != nil {
			return nil, err
		}
	}
	if !forAlter {
		if configHasKey(config, "read_fan_out_factor") || configHasKey(config, "read_fan_out_delay_ms") {
			return nil, errors.NewQQLSyntaxError("WITH PARAMS { read_fan_out_factor, read_fan_out_delay_ms } is supported only for ALTER COLLECTION", p.peek().Pos)
		}
	}
	replicationFactor, err := collectionPositiveUint64(config, "replication_factor", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	writeConsistencyFactor, err := collectionPositiveUint64(config, "write_consistency_factor", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	readFanOutFactor, err := collectionPositiveUint64(config, "read_fan_out_factor", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	readFanOutDelayMs, err := collectionNonNegativeUint64(config, "read_fan_out_delay_ms", p.peek().Pos)
	if err != nil {
		return nil, err
	}
	return &ast.CollectionConfig{
		Params: &ast.CollectionParamsConfig{
			ReplicationFactor:      replicationFactor,
			WriteConsistencyFactor: writeConsistencyFactor,
			ReadFanOutFactor:       readFanOutFactor,
			ReadFanOutDelayMs:      readFanOutDelayMs,
			OnDiskPayload:          collectionBool(config, "on_disk_payload"),
		},
	}, nil
}

func collectionBool(config map[string]any, key string) *bool {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil
	}
	if b, ok := v.(bool); ok {
		return &b
	}
	// This shouldn't happen at runtime since parseDict validates types
	return nil
}

func collectionPositiveUint64(config map[string]any, key string, pos int) (*uint64, error) {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil, nil
	}
	if num, ok := v.(int); ok && num > 0 {
		val := uint64(num)
		return &val, nil
	}
	return nil, errors.NewQQLSyntaxError(key+" must be a positive integer", pos)
}

func collectionNonNegativeUint64(config map[string]any, key string, pos int) (*uint64, error) {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil, nil
	}
	if num, ok := v.(int); ok && num >= 0 {
		val := uint64(num)
		return &val, nil
	}
	return nil, errors.NewQQLSyntaxError(key+" must be a non-negative integer", pos)
}

func collectionFloatRange(config map[string]any, key string, min, max float64) *float64 {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil
	}
	switch typed := v.(type) {
	case int:
		f := float64(typed)
		if f >= min && f <= max {
			return &f
		}
	case float64:
		if typed >= min && typed <= max {
			return &typed
		}
	}
	return nil
}

func collectionMaxOptimizationThreads(config map[string]any, key string) any {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil
	}
	switch typed := v.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case string:
		if toLower(typed) == "auto" {
			return "auto"
		}
	}
	return nil
}

func collectionValue(config map[string]any, key string) (any, bool) {
	if v, ok := config[key]; ok {
		return v, true
	}
	var matches []string
	for k := range config {
		if toLower(k) == key {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		return nil, false
	}
	sort.Strings(matches)
	return config[matches[0]], true
}

func configHasKey(config map[string]any, key string) bool {
	_, ok := collectionValue(config, key)
	return ok
}

func (p *Parser) validateHnswValue(key string, raw any) error {
	lower := toLower(key)
	switch lower {
	case "m", "ef_construct", "full_scan_threshold", "max_indexing_threads", "payload_m":
		if _, ok := raw.(int); !ok {
			return errors.NewQQLSyntaxError(key+" must be an integer", p.peek().Pos)
		}
	case "on_disk", "inline_storage":
		if _, ok := raw.(bool); !ok {
			return errors.NewQQLSyntaxError(key+" must be true or false", p.peek().Pos)
		}
	}
	return nil
}

func (p *Parser) validateVectorsValue(key string, raw any) error {
	lower := toLower(key)
	switch lower {
	case "on_disk":
		if _, ok := raw.(bool); !ok {
			return errors.NewQQLSyntaxError(key+" must be true or false", p.peek().Pos)
		}
	}
	return nil
}

func (p *Parser) validateOptimizersValue(key string, raw any) error {
	lower := toLower(key)
	switch lower {
	case "deleted_threshold":
		switch raw.(type) {
		case int, float64:
		default:
			return errors.NewQQLSyntaxError(key+" must be a number", p.peek().Pos)
		}
	case "vacuum_min_vector_number", "default_segment_number", "max_segment_size", "memmap_threshold", "indexing_threshold", "flush_interval_sec":
		if _, ok := raw.(int); !ok {
			return errors.NewQQLSyntaxError(key+" must be an integer", p.peek().Pos)
		}
	case "max_optimization_threads":
		switch raw.(type) {
		case int, string:
		default:
			return errors.NewQQLSyntaxError(key+" must be a positive integer or 'auto'", p.peek().Pos)
		}
	case "prevent_unoptimized":
		if _, ok := raw.(bool); !ok {
			return errors.NewQQLSyntaxError(key+" must be true or false", p.peek().Pos)
		}
	}
	return nil
}

func (p *Parser) validateParamsValue(key string, raw any) error {
	lower := toLower(key)
	switch lower {
	case "replication_factor", "write_consistency_factor", "read_fan_out_factor", "read_fan_out_delay_ms":
		if _, ok := raw.(int); !ok {
			return errors.NewQQLSyntaxError(key+" must be an integer", p.peek().Pos)
		}
	case "on_disk_payload":
		if _, ok := raw.(bool); !ok {
			return errors.NewQQLSyntaxError(key+" must be true or false", p.peek().Pos)
		}
	}
	return nil
}

func (p *Parser) parseQuantizeClause() (*ast.QuantizationConfig, error) {
	tok := p.peek()

	switch tok.Kind {
	case lexer.TokenKindScalar:
		p.advance()
		var quantile *float64
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindQuantile {
			p.advance()
			valueTok := p.peek()
			value, err := p.parseNumericLiteral()
			if err != nil {
				return nil, err
			}
			if value < 0 || value > 1 {
				return nil, errors.NewQQLSyntaxError("QUANTILE must be between 0 and 1 inclusive, got '"+valueTok.Value+"'", valueTok.Pos)
			}
			quantile = &value
		}
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeScalar,
			Quantile:  quantile,
			AlwaysRAM: alwaysRAM,
		}, nil
	case lexer.TokenKindBinary:
		p.advance()
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeBinary,
			AlwaysRAM: alwaysRAM,
		}, nil
	case lexer.TokenKindProduct:
		p.advance()
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeProduct,
			AlwaysRAM: alwaysRAM,
		}, nil
	case lexer.TokenKindTurbo:
		p.advance()
		var turboBits *float64
		alwaysRAM := false
		if p.peek().Kind == lexer.TokenKindBits {
			p.advance()
			bitsTok := p.peek()
			raw, err := p.parseNumericLiteral()
			if err != nil {
				return nil, err
			}
			if raw != 1.0 && raw != 1.5 && raw != 2.0 && raw != 4.0 {
				return nil, errors.NewQQLSyntaxError("BITS must be one of 1, 1.5, 2, or 4 for TURBO quantization, got '"+bitsTok.Value+"'", bitsTok.Pos)
			}
			turboBits = &raw
		}
		if p.peek().Kind == lexer.TokenKindAlways {
			p.advance()
			if _, err := p.expect(lexer.TokenKindRam); err != nil {
				return nil, err
			}
			alwaysRAM = true
		}
		return &ast.QuantizationConfig{
			Type:      ast.QuantizationTypeTurbo,
			TurboBits: turboBits,
			AlwaysRAM: alwaysRAM,
		}, nil
	default:
		return nil, errors.NewQQLSyntaxError("Expected SCALAR, BINARY, PRODUCT, or TURBO after QUANTIZE, got '"+tok.Value+"'", tok.Pos)
	}
}

func (p *Parser) parseCreateIndex() (*ast.CreateIndexStmt, error) {
	p.advance()
	if _, err := p.expect(lexer.TokenKindOn); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindCollection); err != nil {
		return nil, err
	}
	collection, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenKindFor); err != nil {
		return nil, err
	}
	field, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}
	fieldType := "keyword"
	if p.peek().Kind == lexer.TokenKindType {
		p.advance()
		typeTok, err := p.expect(lexer.TokenKindIdentifier)
		if err != nil {
			return nil, err
		}
		fieldType = toLower(typeTok.Value)
	}
	var options map[string]any
	if p.peek().Kind == lexer.TokenKindWith {
		p.advance()
		dict, err := p.parseDict()
		if err != nil {
			return nil, err
		}
		options = dict
	}
	return &ast.CreateIndexStmt{
		Collection: collection,
		Field:      field,
		FieldType:  fieldType,
		Options:    options,
	}, nil
}

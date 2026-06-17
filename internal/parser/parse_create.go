package parser

import (
	"sort"
	"strings"

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
	var explicitVectors []ast.VectorDef
	var explicitSparseVectors []ast.SparseVectorDef

	if p.peek().Kind == lexer.TokenKindLparen {
		p.advance()
		for p.peek().Kind != lexer.TokenKindRparen && p.peek().Kind != lexer.TokenKindEof {
			nameTok, err := p.expect(lexer.TokenKindIdentifier)
			if err != nil {
				return nil, err
			}
			if p.peek().Kind == lexer.TokenKindVector {
				p.advance()
				if _, err := p.expect(lexer.TokenKindLparen); err != nil {
					return nil, err
				}
				sizeTok := p.peek()
				size, err := p.parseNumericLiteral()
				if err != nil {
					return nil, err
				}
				if size <= 0 || float64(uint64(size)) != size {
					return nil, errors.NewQQLSyntaxError("Vector size must be a positive integer", sizeTok.Pos)
				}
				if _, err := p.expect(lexer.TokenKindComma); err != nil {
					return nil, err
				}
				distTok := p.peek()
				var distance ast.VectorDistance
				switch distTok.Kind {
				case lexer.TokenKindCosine:
					distance = ast.DistanceCosine
				case lexer.TokenKindDot:
					distance = ast.DistanceDot
				case lexer.TokenKindEuclid:
					distance = ast.DistanceEuclid
				case lexer.TokenKindManhattan:
					distance = ast.DistanceManhattan
				default:
					return nil, errors.NewQQLSyntaxError("Expected distance metric (COSINE, DOT, EUCLID, MANHATTAN)", distTok.Pos)
				}
				p.advance()
				if _, err := p.expect(lexer.TokenKindRparen); err != nil {
					return nil, err
				}

				var hnsw *ast.HnswRuntimeConfig
				var quant *ast.QuantizationConfig

				for p.peek().Kind == lexer.TokenKindWith {
					p.advance()
					if p.peek().Kind == lexer.TokenKindHnsw {
						p.advance()
						block, err := p.parseHnswConfigBlock()
						if err != nil {
							return nil, err
						}
						hnsw = block.Hnsw
					} else if p.peek().Kind == lexer.TokenKindQuantize || (p.peek().Kind == lexer.TokenKindIdentifier && asciiEqual(p.peek().Value, "QUANTIZATION")) {
						p.advance()
						block, err := p.parseQuantizationConfigBlock()
						if err != nil {
							return nil, err
						}
						quant = block.Quantization
					} else {
						return nil, errors.NewQQLSyntaxError("Expected HNSW or QUANTIZATION after WITH for vector configuration", p.peek().Pos)
					}
				}

				explicitVectors = append(explicitVectors, ast.VectorDef{
					Name:         nameTok.Value,
					Size:         uint64(size),
					Distance:     distance,
					Hnsw:         hnsw,
					Quantization: quant,
				})
			} else if p.peek().Kind == lexer.TokenKindSparse {
				p.advance()
				explicitSparseVectors = append(explicitSparseVectors, ast.SparseVectorDef{
					Name: nameTok.Value,
				})
			} else {
				return nil, errors.NewQQLSyntaxError("Expected VECTOR or SPARSE after vector name", p.peek().Pos)
			}

			if p.peek().Kind == lexer.TokenKindComma {
				p.advance()
			} else if p.peek().Kind != lexer.TokenKindRparen {
				return nil, errors.NewQQLSyntaxError("Expected comma or )", p.peek().Pos)
			}
		}
		if _, err := p.expect(lexer.TokenKindRparen); err != nil {
			return nil, err
		}
	}

	if p.peek().Kind == lexer.TokenKindHybrid {
		p.advance()
		hybrid = true
		if p.peek().Kind == lexer.TokenKindRerank {
			p.advance()
			rerank = true
		} else {
			for p.peek().Kind == lexer.TokenKindDense || p.peek().Kind == lexer.TokenKindSparse {
				mode := p.advance().Kind
				if p.peek().Kind == lexer.TokenKindVector || (p.peek().Kind == lexer.TokenKindIdentifier && asciiEqual(p.peek().Value, "VECTOR")) {
					p.advance()
					v, err := p.parseStringPtr()
					if err != nil {
						return nil, err
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
	return &ast.CreateCollectionStmt{
		Collection:    collection,
		Hybrid:        hybrid,
		Rerank:        rerank,
		Model:         model,
		Config:        config,
		DenseVector:   denseVector,
		SparseVector:  sparseVector,
		Vectors:       explicitVectors,
		SparseVectors: explicitSparseVectors,
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
	if new.Quantization != nil {
		if current.Quantization != nil {
			return nil, errors.NewQQLSyntaxError("QUANTIZATION clause may only appear once", pos)
		}
		current.Quantization = new.Quantization
	}
	if new.QuantizationUpdate != nil {
		if current.QuantizationUpdate != nil {
			return nil, errors.NewQQLSyntaxError("QUANTIZATION clause may only appear once", pos)
		}
		current.QuantizationUpdate = new.QuantizationUpdate
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
	if tok.Kind == lexer.TokenKindQuantize || (tok.Kind == lexer.TokenKindIdentifier && asciiEqual(tok.Value, "QUANTIZATION")) {
		p.advance()
		return p.parseQuantizationConfigBlock()
	}
	return nil, errors.NewQQLSyntaxError(
		"Expected HNSW, VECTORS, OPTIMIZERS, PARAMS, or QUANTIZATION after WITH, got '"+tok.Value+"'",
		tok.Pos,
	)
}

func (p *Parser) parseHnswConfigBlock() (*ast.CollectionConfig, error) {
	config, err := p.parseConfigBlock()
	if err != nil {
		return nil, err
	}
	for key := range config {
		lower := strings.ToLower(key)
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
	config, err := p.parseConfigBlock()
	if err != nil {
		return nil, err
	}
	for key := range config {
		if strings.ToLower(key) != "on_disk" {
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
	config, err := p.parseConfigBlock()
	if err != nil {
		return nil, err
	}
	for key := range config {
		lower := strings.ToLower(key)
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
		lower := strings.ToLower(key)
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
				if strings.ToLower(v) != "auto" {
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
	config, err := p.parseConfigBlock()
	if err != nil {
		return nil, err
	}
	for key := range config {
		lower := strings.ToLower(key)
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
			return nil, errors.NewQQLSyntaxError("WITH PARAMS (read_fan_out_factor, read_fan_out_delay_ms) is supported only for ALTER COLLECTION", p.peek().Pos)
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
	return nil
}

func collectionPositiveUint64(config map[string]any, key string, pos int) (*uint64, error) {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil, nil
	}
	switch num := v.(type) {
	case int:
		if num > 0 {
			val := uint64(num)
			return &val, nil
		}
	case float64:
		if num > 0 && num == float64(int(num)) {
			val := uint64(num)
			return &val, nil
		}
	}
	return nil, errors.NewQQLSyntaxError(key+" must be a positive integer", pos)
}

func collectionNonNegativeUint64(config map[string]any, key string, pos int) (*uint64, error) {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil, nil
	}
	switch num := v.(type) {
	case int:
		if num >= 0 {
			val := uint64(num)
			return &val, nil
		}
	case float64:
		if num >= 0 && num == float64(int(num)) {
			val := uint64(num)
			return &val, nil
		}
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

func collectionMaxOptimizationThreads(config map[string]any, key string) *ast.OptimizationThreads {
	v, ok := collectionValue(config, key)
	if !ok {
		return nil
	}
	switch typed := v.(type) {
	case int:
		if typed > 0 {
			return &ast.OptimizationThreads{Value: uint64(typed)}
		}
	case string:
		if strings.ToLower(typed) == "auto" {
			return &ast.OptimizationThreads{Auto: true}
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
		if strings.ToLower(k) == key {
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
	lower := strings.ToLower(key)
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
	lower := strings.ToLower(key)
	switch lower {
	case "on_disk":
		if _, ok := raw.(bool); !ok {
			return errors.NewQQLSyntaxError(key+" must be true or false", p.peek().Pos)
		}
	}
	return nil
}

func (p *Parser) validateOptimizersValue(key string, raw any) error {
	lower := strings.ToLower(key)
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
	lower := strings.ToLower(key)
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

func (p *Parser) parseQuantizationConfigBlock() (*ast.CollectionConfig, error) {
	config, err := p.parseConfigBlock()
	if err != nil {
		return nil, err
	}

	if configHasKey(config, "disabled") {
		if collectionBool(config, "disabled") != nil && *collectionBool(config, "disabled") {
			return &ast.CollectionConfig{
				QuantizationUpdate: &ast.QuantizationUpdate{Disabled: true},
			}, nil
		}
	}

	typeRaw, ok := collectionValue(config, "type")
	if !ok {
		return nil, errors.NewQQLSyntaxError("QUANTIZATION config requires a 'type' (scalar, binary, product, turbo)", p.peek().Pos)
	}

	typeStr, ok := typeRaw.(string)
	if !ok {
		return nil, errors.NewQQLSyntaxError("QUANTIZATION 'type' must be a string", p.peek().Pos)
	}

	typeStr = strings.ToLower(typeStr)

	var qType ast.QuantizationType
	switch typeStr {
	case "scalar":
		qType = ast.QuantizationTypeScalar
	case "binary":
		qType = ast.QuantizationTypeBinary
	case "product":
		qType = ast.QuantizationTypeProduct
	case "turbo":
		qType = ast.QuantizationTypeTurbo
	default:
		return nil, errors.NewQQLSyntaxError("Unknown QUANTIZATION type '"+typeStr+"'. Expected scalar, binary, product, turbo", p.peek().Pos)
	}

	var alwaysRAM bool
	if val := collectionBool(config, "always_ram"); val != nil {
		alwaysRAM = *val
	}

	var quantile *float64
	if qType == ast.QuantizationTypeScalar {
		if configHasKey(config, "quantile") {
			quantile = collectionFloatRange(config, "quantile", 0.0, 1.0)
			if quantile == nil {
				return nil, errors.NewQQLSyntaxError("quantile must be between 0.0 and 1.0", p.peek().Pos)
			}
		}
	}

	var turboBits *float64
	if qType == ast.QuantizationTypeTurbo {
		if v, ok := collectionValue(config, "bits"); ok {
			switch typed := v.(type) {
			case int:
				f := float64(typed)
				turboBits = &f
			case float64:
				turboBits = &typed
			}
			if turboBits != nil && *turboBits != 1.0 && *turboBits != 1.5 && *turboBits != 2.0 && *turboBits != 4.0 {
				return nil, errors.NewQQLSyntaxError("bits must be one of 1, 1.5, 2, or 4 for TURBO quantization", p.peek().Pos)
			}
		}
	}

	qConfig := &ast.QuantizationConfig{
		Type:      qType,
		AlwaysRAM: alwaysRAM,
		Quantile:  quantile,
		TurboBits: turboBits,
	}

	return &ast.CollectionConfig{
		Quantization:       qConfig,
		QuantizationUpdate: &ast.QuantizationUpdate{Config: qConfig},
	}, nil
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
		fieldType = strings.ToLower(typeTok.Value)
	}
	var options map[string]any
	if p.peek().Kind == lexer.TokenKindWith {
		p.advance()
		dict, err := p.parseConfigBlock()
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

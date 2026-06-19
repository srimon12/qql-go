package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCreate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *ast.CreateCollectionStmt
	}{
		{
			name:  "simple create",
			input: "CREATE COLLECTION mycollection",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
			},
		},
		{
			name:  "create with hybrid",
			input: "CREATE COLLECTION mycollection HYBRID",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Hybrid:     true,
			},
		},
		{
			name:  "create with model",
			input: "CREATE COLLECTION mycollection USING MODEL 'dense-model'",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Model:      strPtr("dense-model"),
			},
		},
		{
			name:  "create with scalar quantization",
			input: "CREATE COLLECTION mycollection WITH QUANTIZATION (type = 'scalar')",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Config: &ast.CollectionConfig{
					Quantization: &ast.QuantizationConfig{
						Type: ast.QuantizationTypeScalar,
					},
				},
			},
		},
		{
			name:  "create with scalar quantization integer boundary",
			input: "CREATE COLLECTION mycollection WITH QUANTIZATION (type = 'scalar', quantile = 1)",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Config: &ast.CollectionConfig{
					Quantization: &ast.QuantizationConfig{
						Type:     ast.QuantizationTypeScalar,
						Quantile: float64Ptr(1.0),
					},
				},
			},
		},
		{
			name:  "create with hybrid rerank product quantization",
			input: "CREATE COLLECTION mycollection HYBRID RERANK WITH QUANTIZATION (type = 'product', always_ram = true)",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Hybrid:     true,
				Rerank:     true,
				Config: &ast.CollectionConfig{
					Quantization: &ast.QuantizationConfig{
						Type:      ast.QuantizationTypeProduct,
						AlwaysRAM: true,
					},
				},
			},
		},
		{
			name:  "create with payload hnsw",
			input: "CREATE COLLECTION mycollection WITH HNSW (payload_m = 16)",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Config: &ast.CollectionConfig{
					Hnsw: &ast.HnswRuntimeConfig{
						PayloadM: uint64Ptr(16),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)

			stmt, ok := node.(*ast.CreateCollectionStmt)
			require.True(t, ok, "expected CreateCollectionStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			assert.Equal(t, tt.want.Rerank, stmt.Rerank)
			if tt.want.Model != nil {
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.Config != nil && tt.want.Config.Quantization != nil {
				require.NotNil(t, stmt.Config)
				require.NotNil(t, stmt.Config.Quantization)
				assert.Equal(t, tt.want.Config.Quantization.Type, stmt.Config.Quantization.Type)
				assert.Equal(t, tt.want.Config.Quantization.AlwaysRAM, stmt.Config.Quantization.AlwaysRAM)
				if tt.want.Config.Quantization.Quantile != nil {
					require.NotNil(t, stmt.Config.Quantization.Quantile)
					assert.InDelta(t, *tt.want.Config.Quantization.Quantile, *stmt.Config.Quantization.Quantile, 0.0001)
				} else {
					assert.Nil(t, stmt.Config.Quantization.Quantile)
				}
			}
			if tt.want.Config != nil {
				require.NotNil(t, stmt.Config)
				if tt.want.Config.Hnsw != nil {
					require.NotNil(t, stmt.Config.Hnsw)
					assert.Equal(t, tt.want.Config.Hnsw.PayloadM, stmt.Config.Hnsw.PayloadM)
				} else {
					assert.Nil(t, stmt.Config.Hnsw)
				}
			} else {
				assert.Nil(t, stmt.Config)
			}
		})
	}
}

func TestParseCollectionConfigCaseVariantKeysAreDeterministic(t *testing.T) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize("CREATE COLLECTION docs WITH HNSW ( M = 32, m = 16 )")
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)
	stmt, ok := node.(*ast.CreateCollectionStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.Config)
	require.NotNil(t, stmt.Config.Hnsw)
	require.NotNil(t, stmt.Config.Hnsw.M)
	require.Equal(t, uint64(16), *stmt.Config.Hnsw.M)
}

func TestParseCollectionConfigRejectsNonPositiveValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "hnsw m under 4",
			input: "CREATE COLLECTION docs WITH HNSW ( m = 3 )",
			want:  "m must be 0 or >= 4",
		},
		{
			name:  "params replication factor zero",
			input: "CREATE COLLECTION docs WITH PARAMS ( replication_factor = 0 )",
			want:  "replication_factor must be a positive integer",
		},
		{
			name:  "hnsw full scan threshold negative",
			input: "CREATE COLLECTION docs WITH HNSW ( full_scan_threshold = -1 )",
			want:  "full_scan_threshold must be a non-negative integer",
		},
		{
			name:  "optimizer indexing threshold negative",
			input: "CREATE COLLECTION docs WITH OPTIMIZERS ( indexing_threshold = -1 )",
			want:  "indexing_threshold must be a non-negative integer",
		},
		{
			name:  "alter read fan out delay negative",
			input: "ALTER COLLECTION docs WITH PARAMS ( read_fan_out_delay_ms = -1 )",
			want:  "read_fan_out_delay_ms must be a non-negative integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			_, err = NewParser().Parse(tokens)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseCreateQuantizeErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing quantize type",
			input: "CREATE COLLECTION docs QUANTIZE",
		},
		{
			name:  "unknown quantize type",
			input: "CREATE COLLECTION docs WITH QUANTIZATION (type = 'full')",
		},
		{
			name:  "quantile above one",
			input: "CREATE COLLECTION docs WITH QUANTIZATION (type = 'scalar', quantile = 1.5)",
		},
		{
			name:  "quantile integer above one",
			input: "CREATE COLLECTION docs WITH QUANTIZATION (type = 'scalar', quantile = 2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			_, err = NewParser().Parse(tokens)
			require.Error(t, err)
		})
	}
}

func TestParseCreateIndex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.CreateIndexStmt
		wantErr bool
	}{
		{
			name:  "simple create index",
			input: "CREATE INDEX ON COLLECTION mycollection FOR field",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "field",
				FieldType:  "keyword",
			},
		},
		{
			name:  "create index with type keyword",
			input: "CREATE INDEX ON COLLECTION mycollection FOR field TYPE keyword",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "field",
				FieldType:  "keyword",
			},
		},
		{
			name:  "create index with type integer",
			input: "CREATE INDEX ON COLLECTION mycollection FOR patient_id TYPE integer",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "patient_id",
				FieldType:  "integer",
			},
		},
		{
			name:  "create index with type float",
			input: "CREATE INDEX ON COLLECTION mycollection FOR score TYPE float",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "score",
				FieldType:  "float",
			},
		},
		{
			name:  "create index with type bool",
			input: "CREATE INDEX ON COLLECTION mycollection FOR is_active TYPE bool",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "is_active",
				FieldType:  "bool",
			},
		},
		{
			name:  "create index with keyword options",
			input: "CREATE INDEX ON COLLECTION mycollection FOR tenant_id TYPE keyword WITH (is_tenant = true, on_disk = true, enable_hnsw = false)",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "tenant_id",
				FieldType:  "keyword",
				Options: map[string]any{
					"is_tenant":   true,
					"on_disk":     true,
					"enable_hnsw": false,
				},
			},
		},
		{
			name:  "create index with text options",
			input: "CREATE INDEX ON COLLECTION mycollection FOR title TYPE text WITH (tokenizer = 'word', min_token_len = 2, max_token_len = 20, lowercase = true)",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "title",
				FieldType:  "text",
				Options: map[string]any{
					"tokenizer":     "word",
					"min_token_len": 2,
					"max_token_len": 20,
					"lowercase":     true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)

			p := NewParser()
			node, err := p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			stmt, ok := node.(*ast.CreateIndexStmt)
			require.True(t, ok, "expected CreateIndexStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.Field, stmt.Field)
			assert.Equal(t, tt.want.FieldType, stmt.FieldType)
			assert.Equal(t, tt.want.Options, stmt.Options)
		})
	}
}

func TestParseCreateCollectionWithTurboQuantization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType ast.QuantizationType
		wantBits *float64
		wantRAM  bool
	}{
		{
			name:     "turbo default",
			input:    `CREATE COLLECTION docs WITH QUANTIZATION (type = 'turbo')`,
			wantType: ast.QuantizationTypeTurbo,
			wantRAM:  false,
		},
		{
			name:     "turbo bits 1.5",
			input:    `CREATE COLLECTION docs WITH QUANTIZATION (type = 'turbo', bits = 1.5)`,
			wantType: ast.QuantizationTypeTurbo,
			wantBits: float64Ptr(1.5),
		},
		{
			name:     "turbo bits 1 always ram",
			input:    `CREATE COLLECTION docs WITH QUANTIZATION (type = 'turbo', bits = 1, always_ram = true)`,
			wantType: ast.QuantizationTypeTurbo,
			wantBits: float64Ptr(1.0),
			wantRAM:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			require.NoError(t, err)
			p := NewParser()
			node, err := p.Parse(tokens)
			require.NoError(t, err)
			create, ok := node.(*ast.CreateCollectionStmt)
			require.True(t, ok, "expected CreateCollectionStmt, got %T", node)
			require.NotNil(t, create.Config)
			require.NotNil(t, create.Config.Quantization)
			assert.Equal(t, tt.wantType, create.Config.Quantization.Type)
			assert.Equal(t, tt.wantRAM, create.Config.Quantization.AlwaysRAM)
			if tt.wantBits != nil {
				require.NotNil(t, create.Config.Quantization.TurboBits)
				assert.InDelta(t, *tt.wantBits, *create.Config.Quantization.TurboBits, 0.0001)
			} else {
				assert.Nil(t, create.Config.Quantization.TurboBits)
			}
		})
	}

	t.Run("invalid turbo bits", func(t *testing.T) {
		l := &lexer.Lexer{}
		tokens, err := l.Tokenize(`CREATE COLLECTION docs WITH QUANTIZATION (type = 'turbo', bits = 3)`)
		require.NoError(t, err)
		_, err = NewParser().Parse(tokens)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bits must be one of 1, 1.5, 2, or 4")
	})
}

func TestParseCreateMultiVector(t *testing.T) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(`CREATE COLLECTION knowledge_graph (
		dense_text VECTOR(384, COSINE),
		clip_img VECTOR(512, DOT),
		bm25_text SPARSE
	) WITH HNSW ( m = 32 )`)
	require.NoError(t, err)

	p := NewParser()
	stmt, err := p.Parse(tokens)
	require.NoError(t, err)

	create, ok := stmt.(*ast.CreateCollectionStmt)
	require.True(t, ok)
	require.Equal(t, "knowledge_graph", create.Collection)
	require.Len(t, create.Vectors, 2)
	require.Len(t, create.SparseVectors, 1)

	require.Equal(t, "dense_text", create.Vectors[0].Name)
	require.Equal(t, uint64(384), create.Vectors[0].Size)
	require.Equal(t, ast.DistanceCosine, create.Vectors[0].Distance)

	require.Equal(t, "clip_img", create.Vectors[1].Name)
	require.Equal(t, uint64(512), create.Vectors[1].Size)
	require.Equal(t, ast.DistanceDot, create.Vectors[1].Distance)

	require.Equal(t, "bm25_text", create.SparseVectors[0].Name)

	require.NotNil(t, create.Config)
	require.NotNil(t, create.Config.Hnsw)
	require.Equal(t, uint64(32), *create.Config.Hnsw.M)
}

func TestParseCreateMultiVectorWithOverrides(t *testing.T) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(`CREATE COLLECTION test_overrides (
		dense_vec VECTOR(384, COSINE) WITH HNSW ( m = 16 ) WITH QUANTIZATION (type = 'scalar', always_ram = true),
		colbert_vec VECTOR(128, DOT) WITH QUANTIZATION (type = 'turbo', bits = 2)
	)`)
	require.NoError(t, err)

	p := NewParser()
	stmt, err := p.Parse(tokens)
	require.NoError(t, err)

	create, ok := stmt.(*ast.CreateCollectionStmt)
	require.True(t, ok)
	require.Equal(t, "test_overrides", create.Collection)
	require.Len(t, create.Vectors, 2)

	require.Equal(t, "dense_vec", create.Vectors[0].Name)
	require.NotNil(t, create.Vectors[0].Hnsw)
	require.Equal(t, uint64(16), *create.Vectors[0].Hnsw.M)
	require.NotNil(t, create.Vectors[0].Quantization)
	require.Equal(t, ast.QuantizationTypeScalar, create.Vectors[0].Quantization.Type)
	require.True(t, create.Vectors[0].Quantization.AlwaysRAM)

	require.Equal(t, "colbert_vec", create.Vectors[1].Name)
	require.Nil(t, create.Vectors[1].Hnsw)
	require.NotNil(t, create.Vectors[1].Quantization)
	require.Equal(t, ast.QuantizationTypeTurbo, create.Vectors[1].Quantization.Type)
	require.NotNil(t, create.Vectors[1].Quantization.TurboBits)
	require.Equal(t, float64(2.0), *create.Vectors[1].Quantization.TurboBits)
}

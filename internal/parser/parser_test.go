package parser

import (
	"strings"
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInsert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.InsertStmt
		wantErr bool
	}{
		{
			name:  "simple insert",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"}`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello"},
			},
		},
		{
			name:  "insert with bare keys",
			input: `INSERT INTO COLLECTION test VALUES {text: 'hello', topic: 'search'}`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello", "topic": "search"},
			},
		},
		{
			name:  "insert with explicit id",
			input: `INSERT INTO COLLECTION test VALUES {id: 'point-123', text: 'hello'}`,
			want: &ast.InsertStmt{
				Collection: "test",
				PointID:    "point-123",
				Values:     map[string]interface{}{"text": "hello"},
			},
		},
		{
			name:  "insert with model",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING MODEL 'model-name'`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello"},
				Model:      strPtr("model-name"),
			},
		},
		{
			name:  "insert with hybrid",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING HYBRID`,
			want: &ast.InsertStmt{
				Collection: "test",
				Values:     map[string]interface{}{"text": "hello"},
				Hybrid:     true,
			},
		},
		{
			name:  "insert with hybrid and models",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING HYBRID DENSE MODEL 'dense-model' SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection:  "test",
				Values:      map[string]interface{}{"text": "hello"},
				Hybrid:      true,
				Model:       strPtr("dense-model"),
				SparseModel: strPtr("sparse-model"),
			},
		},
		{
			name:  "insert with sparse model only",
			input: `INSERT INTO COLLECTION test VALUES {"text": "hello"} USING HYBRID SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertStmt{
				Collection:  "test",
				Values:      map[string]interface{}{"text": "hello"},
				Hybrid:      true,
				SparseModel: strPtr("sparse-model"),
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

			stmt, ok := node.(*ast.InsertStmt)
			require.True(t, ok, "expected InsertStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.Values, stmt.Values)
			assert.Equal(t, tt.want.PointID, stmt.PointID)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			if tt.want.Model != nil {
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.SparseModel != nil {
				assert.Equal(t, *tt.want.SparseModel, *stmt.SparseModel)
			}
		})
	}
}

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
			input: "CREATE COLLECTION mycollection QUANTIZE SCALAR",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Quantization: &ast.QuantizationConfig{
					Type: ast.QuantizationTypeScalar,
				},
			},
		},
		{
			name:  "create with scalar quantization integer boundary",
			input: "CREATE COLLECTION mycollection QUANTIZE SCALAR QUANTILE 1",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Quantization: &ast.QuantizationConfig{
					Type:     ast.QuantizationTypeScalar,
					Quantile: float64Ptr(1.0),
				},
			},
		},
		{
			name:  "create with hybrid rerank product quantization",
			input: "CREATE COLLECTION mycollection HYBRID RERANK QUANTIZE PRODUCT ALWAYS RAM",
			want: &ast.CreateCollectionStmt{
				Collection: "mycollection",
				Hybrid:     true,
				Rerank:     true,
				Quantization: &ast.QuantizationConfig{
					Type:      ast.QuantizationTypeProduct,
					AlwaysRAM: true,
				},
			},
		},
		{
			name:  "create with payload hnsw",
			input: "CREATE COLLECTION mycollection WITH HNSW {payload_m: 16}",
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
			if tt.want.Quantization != nil {
				require.NotNil(t, stmt.Quantization)
				assert.Equal(t, tt.want.Quantization.Type, stmt.Quantization.Type)
				assert.Equal(t, tt.want.Quantization.AlwaysRAM, stmt.Quantization.AlwaysRAM)
				if tt.want.Quantization.Quantile != nil {
					require.NotNil(t, stmt.Quantization.Quantile)
					assert.InDelta(t, *tt.want.Quantization.Quantile, *stmt.Quantization.Quantile, 0.0001)
				} else {
					assert.Nil(t, stmt.Quantization.Quantile)
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

func TestParseCreateRejectsAlterOnlyParamsCaseInsensitive(t *testing.T) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize("CREATE COLLECTION docs WITH PARAMS { Read_Fan_Out_Factor: 4 }")
	require.NoError(t, err)

	p := NewParser()
	_, err = p.Parse(tokens)
	require.Error(t, err)
	require.Contains(t, err.Error(), "supported only for ALTER COLLECTION")
}

func TestParseCollectionConfigCaseVariantKeysAreDeterministic(t *testing.T) {
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize("CREATE COLLECTION docs WITH HNSW { M: 32, m: 16 }")
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
			name:  "hnsw m zero",
			input: "CREATE COLLECTION docs WITH HNSW { m: 0 }",
			want:  "m must be a positive integer",
		},
		{
			name:  "params replication factor zero",
			input: "CREATE COLLECTION docs WITH PARAMS { replication_factor: 0 }",
			want:  "replication_factor must be a positive integer",
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
			input: "CREATE COLLECTION docs QUANTIZE FULL",
		},
		{
			name:  "quantile above one",
			input: "CREATE COLLECTION docs QUANTIZE SCALAR QUANTILE 1.5",
		},
		{
			name:  "quantile integer above one",
			input: "CREATE COLLECTION docs QUANTIZE SCALAR QUANTILE 2",
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

func TestParseInsertBulk(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *ast.InsertBulkStmt
	}{
		{
			name:  "simple bulk insert",
			input: `INSERT BULK INTO COLLECTION test VALUES [{"text": "hello"}, {"text": "world"}]`,
			want: &ast.InsertBulkStmt{
				Collection: "test",
				ValuesList: []map[string]interface{}{
					{"text": "hello"},
					{"text": "world"},
				},
			},
		},
		{
			name:  "bulk insert with hybrid models",
			input: `INSERT BULK INTO COLLECTION test VALUES [{"text": "hello"}] USING HYBRID DENSE MODEL 'dense-model' SPARSE MODEL 'sparse-model'`,
			want: &ast.InsertBulkStmt{
				Collection: "test",
				ValuesList: []map[string]interface{}{
					{"text": "hello"},
				},
				Hybrid:      true,
				Model:       strPtr("dense-model"),
				SparseModel: strPtr("sparse-model"),
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

			stmt, ok := node.(*ast.InsertBulkStmt)
			require.True(t, ok, "expected InsertBulkStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.ValuesList, stmt.ValuesList)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			if tt.want.Model != nil {
				require.NotNil(t, stmt.Model)
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.SparseModel != nil {
				require.NotNil(t, stmt.SparseModel)
				assert.Equal(t, *tt.want.SparseModel, *stmt.SparseModel)
			}
		})
	}
}

func TestParseDrop(t *testing.T) {
	input := "DROP COLLECTION mycollection"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.DropCollectionStmt)
	require.True(t, ok, "expected DropCollectionStmt")
	assert.Equal(t, "mycollection", stmt.Collection)
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
			input: "CREATE INDEX ON COLLECTION mycollection FOR tenant_id TYPE keyword WITH {is_tenant: true, on_disk: true, enable_hnsw: false}",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "tenant_id",
				FieldType:  "keyword",
				Options: map[string]interface{}{
					"is_tenant":   true,
					"on_disk":     true,
					"enable_hnsw": false,
				},
			},
		},
		{
			name:  "create index with text options",
			input: "CREATE INDEX ON COLLECTION mycollection FOR title TYPE text WITH {tokenizer: 'word', min_token_len: 2, max_token_len: 20, lowercase: true}",
			want: &ast.CreateIndexStmt{
				Collection: "mycollection",
				Field:      "title",
				FieldType:  "text",
				Options: map[string]interface{}{
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

func TestParseShow(t *testing.T) {
	input := "SHOW COLLECTIONS"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	_, ok := node.(*ast.ShowCollectionsStmt)
	assert.True(t, ok, "expected ShowCollectionsStmt")
}

func TestParseShowCollection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "SHOW COLLECTION docs",
			want:  "docs",
		},
		{
			name:  "case insensitive",
			input: "show collection MY_COL",
			want:  "MY_COL",
		},
		{
			name:    "error without collection name",
			input:   "SHOW COLLECTION",
			wantErr: true,
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

			stmt, ok := node.(*ast.ShowCollectionStmt)
			assert.True(t, ok, "expected ShowCollectionStmt")
			assert.Equal(t, tt.want, stmt.Collection)
		})
	}
}

func TestParseSearch(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.SearchStmt
		wantErr bool
	}{
		{
			name:  "simple search",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
			},
		},
		{
			name:  "search with exact",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 EXACT",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Exact: true},
			},
		},
		{
			name:  "search with with clause hnsw_ef",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {hnsw_ef: 128}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{HnswEf: 128},
			},
		},
		{
			name:  "search with with clause exact true",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {exact: true}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Exact: true},
			},
		},
		{
			name:  "search with with clause exact false",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {exact: false}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Exact: false},
			},
		},
		{
			name:  "search with with clause acorn",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {acorn: true}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{Acorn: true},
			},
		},
		{
			name:  "search with indexed_only and quantization params",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {indexed_only: true, quantization: {ignore: true, rescore: false, oversampling: 2}}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{
					IndexedOnly: true,
					Quantization: &ast.QuantizationSearchWith{
						Ignore:       boolPtr(true),
						Rescore:      boolPtr(false),
						Oversampling: float64Ptr(2),
					},
				},
			},
		},
		{
			name:  "search with mmr params",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WITH {mmr_diversity: 0.5, mmr_candidates: 50}",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				WithClause: &ast.SearchWith{
					MmrDiversity:  float64Ptr(0.5),
					MmrCandidates: intPtr(50),
				},
			},
		},
		{
			name:  "search with boolean filter literal",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WHERE active = true",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				QueryFilter: &ast.CompareExpr{Field: "active", Op: "=", Value: true},
			},
		},
		{
			name:  "search with model",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING MODEL 'my-model'",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Model:      strPtr("my-model"),
			},
		},
		{
			name:  "search with sparse",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING SPARSE",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				SparseOnly: true,
			},
		},
		{
			name:  "search with sparse model",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING SPARSE MODEL 'sparse-model'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				SparseOnly:  true,
				SparseModel: strPtr("sparse-model"),
			},
		},
		{
			name:  "search with hybrid",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Hybrid:     true,
			},
		},
		{
			name:  "search with hybrid and models",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID DENSE MODEL 'dense' SPARSE MODEL 'sparse'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				Hybrid:      true,
				Model:       strPtr("dense"),
				SparseModel: strPtr("sparse"),
			},
		},
		{
			name:  "search with where clause",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WHERE tags = 'important'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				QueryFilter: &ast.CompareExpr{Field: "tags", Op: "=", Value: "important"},
			},
		},
		{
			name:  "search with rerank",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 RERANK",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Rerank:     true,
			},
		},
		{
			name:  "search with rerank model",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 RERANK MODEL 'cross-encoder'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				Rerank:      true,
				RerankModel: strPtr("cross-encoder"),
			},
		},
		{
			name:  "search with hybrid and rerank",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID RERANK",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Hybrid:     true,
				Rerank:     true,
			},
		},
		{
			name:  "search with reordered modifiers",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 EXACT WITH {hnsw_ef: 64, acorn: true} WHERE tags = 'important' RERANK MODEL 'cross-encoder'",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				QueryFilter: &ast.CompareExpr{Field: "tags", Op: "=", Value: "important"},
				Rerank:      true,
				RerankModel: strPtr("cross-encoder"),
				WithClause:  &ast.SearchWith{HnswEf: 64, Exact: true, Acorn: true},
			},
		},
		{
			name:  "grouped hybrid search",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 USING HYBRID GROUP BY category GROUP_SIZE 4",
			want: &ast.SearchStmt{
				Collection: "mycollection",
				QueryText:  "query text",
				Limit:      10,
				Hybrid:     true,
				GroupBy:    "category",
				GroupSize:  4,
			},
		},
		{
			name:  "grouped search with filter",
			input: "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 WHERE tags = 'important' GROUP BY meta.author",
			want: &ast.SearchStmt{
				Collection:  "mycollection",
				QueryText:   "query text",
				Limit:       10,
				GroupBy:     "meta.author",
				GroupSize:   3,
				QueryFilter: &ast.CompareExpr{Field: "tags", Op: "=", Value: "important"},
			},
		},
		{
			name:    "grouped rerank is rejected",
			input:   "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 RERANK GROUP BY category",
			wantErr: true,
		},
		{
			name:    "group size must be positive",
			input:   "SEARCH mycollection SIMILAR TO 'query text' LIMIT 10 GROUP BY category GROUP_SIZE 0",
			wantErr: true,
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

			stmt, ok := node.(*ast.SearchStmt)
			require.True(t, ok, "expected SearchStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.QueryText, stmt.QueryText)
			assert.Equal(t, tt.want.Limit, stmt.Limit)
			assert.Equal(t, tt.want.Hybrid, stmt.Hybrid)
			assert.Equal(t, tt.want.SparseOnly, stmt.SparseOnly)
			if tt.want.Model != nil {
				require.NotNil(t, stmt.Model)
				assert.Equal(t, *tt.want.Model, *stmt.Model)
			}
			if tt.want.SparseModel != nil {
				require.NotNil(t, stmt.SparseModel)
				assert.Equal(t, *tt.want.SparseModel, *stmt.SparseModel)
			}
			if tt.want.WithClause != nil {
				require.NotNil(t, stmt.WithClause)
				assert.Equal(t, tt.want.WithClause.HnswEf, stmt.WithClause.HnswEf)
				assert.Equal(t, tt.want.WithClause.Exact, stmt.WithClause.Exact)
				assert.Equal(t, tt.want.WithClause.Acorn, stmt.WithClause.Acorn)
				assert.Equal(t, tt.want.WithClause.IndexedOnly, stmt.WithClause.IndexedOnly)
				assert.Equal(t, tt.want.WithClause.Quantization, stmt.WithClause.Quantization)
				assert.Equal(t, tt.want.WithClause.MmrDiversity, stmt.WithClause.MmrDiversity)
				assert.Equal(t, tt.want.WithClause.MmrCandidates, stmt.WithClause.MmrCandidates)
			}
			if tt.want.QueryFilter != nil {
				require.NotNil(t, stmt.QueryFilter)
				assertFilterExprEqual(t, tt.want.QueryFilter, stmt.QueryFilter)
			}
			assert.Equal(t, tt.want.Rerank, stmt.Rerank)
			if tt.want.RerankModel != nil {
				require.NotNil(t, stmt.RerankModel)
				assert.Equal(t, *tt.want.RerankModel, *stmt.RerankModel)
			}
			assert.Equal(t, tt.want.GroupBy, stmt.GroupBy)
			assert.Equal(t, tt.want.GroupSize, stmt.GroupSize)
		})
	}
}

func TestParseRecommend(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *ast.RecommendStmt
	}{
		{
			name:  "basic recommend",
			input: "RECOMMEND FROM docs POSITIVE IDS ('seed-1', 'seed-2') NEGATIVE IDS ('seed-3') STRATEGY 'average' LIMIT 5 WHERE score > 0",
			want: &ast.RecommendStmt{
				Collection:  "docs",
				PositiveIDs: []interface{}{"seed-1", "seed-2"},
				NegativeIDs: []interface{}{"seed-3"},
				Limit:       5,
				Strategy:    strPtr("average"),
				QueryFilter: &ast.CompareExpr{Field: "score", Op: ">", Value: 0},
			},
		},
		{
			name:  "recommend with offset",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 OFFSET 5",
			want: &ast.RecommendStmt{
				Collection:  "docs",
				PositiveIDs: []interface{}{"a"},
				Limit:       10,
				Offset:      5,
			},
		},
		{
			name:  "recommend with score threshold",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 SCORE THRESHOLD 0.5",
			want: &ast.RecommendStmt{
				Collection:     "docs",
				PositiveIDs:    []interface{}{"a"},
				Limit:          10,
				ScoreThreshold: float64Ptr(0.5),
			},
		},
		{
			name:  "recommend with score threshold integer",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 SCORE THRESHOLD 1",
			want: &ast.RecommendStmt{
				Collection:     "docs",
				PositiveIDs:    []interface{}{"a"},
				Limit:          10,
				ScoreThreshold: float64Ptr(1.0),
			},
		},
		{
			name:  "recommend with with clause",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 WITH {exact: true, hnsw_ef: 128, indexed_only: true, quantization: {rescore: true}}",
			want: &ast.RecommendStmt{
				Collection:  "docs",
				PositiveIDs: []interface{}{"a"},
				Limit:       10,
				WithClause: &ast.SearchWith{
					Exact:       true,
					HnswEf:      128,
					IndexedOnly: true,
					Quantization: &ast.QuantizationSearchWith{
						Rescore: boolPtr(true),
					},
				},
			},
		},
		{
			name:  "recommend with lookup from",
			input: "RECOMMEND FROM target_collection POSITIVE IDS ('a') LOOKUP FROM source_collection LIMIT 5",
			want: &ast.RecommendStmt{
				Collection:  "target_collection",
				PositiveIDs: []interface{}{"a"},
				Limit:       5,
				LookupFrom:  "source_collection",
			},
		},
		{
			name:  "recommend with lookup from and vector",
			input: "RECOMMEND FROM target_collection POSITIVE IDS ('a') LOOKUP FROM source_collection VECTOR 'dense' LIMIT 5",
			want: &ast.RecommendStmt{
				Collection:   "target_collection",
				PositiveIDs:  []interface{}{"a"},
				Limit:        5,
				LookupFrom:   "source_collection",
				LookupVector: strPtr("dense"),
			},
		},
		{
			name:  "recommend with using",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') USING 'sparse' LIMIT 5",
			want: &ast.RecommendStmt{
				Collection:  "docs",
				PositiveIDs: []interface{}{"a"},
				Limit:       5,
				Using:       strPtr("sparse"),
			},
		},
		{
			name:  "recommend with lookup from and using",
			input: "RECOMMEND FROM target_collection POSITIVE IDS ('a') LOOKUP FROM source_collection VECTOR 'dense' USING 'sparse' LIMIT 5",
			want: &ast.RecommendStmt{
				Collection:   "target_collection",
				PositiveIDs:  []interface{}{"a"},
				Limit:        5,
				LookupFrom:   "source_collection",
				LookupVector: strPtr("dense"),
				Using:        strPtr("sparse"),
			},
		},
		{
			name:  "recommend full featured",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a', 'b') NEGATIVE IDS ('c') STRATEGY 'best_score' LOOKUP FROM src VECTOR 'dense' USING 'sparse' LIMIT 5 OFFSET 2 SCORE THRESHOLD 0.25 WHERE status = 'active' WITH {exact: true}",
			want: &ast.RecommendStmt{
				Collection:     "docs",
				PositiveIDs:    []interface{}{"a", "b"},
				NegativeIDs:    []interface{}{"c"},
				Limit:          5,
				Strategy:       strPtr("best_score"),
				Offset:         2,
				ScoreThreshold: float64Ptr(0.25),
				WithClause:     &ast.SearchWith{Exact: true},
				LookupFrom:     "src",
				LookupVector:   strPtr("dense"),
				Using:          strPtr("sparse"),
				QueryFilter:    &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
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

			stmt, ok := node.(*ast.RecommendStmt)
			require.True(t, ok, "expected RecommendStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.PositiveIDs, stmt.PositiveIDs)
			assert.Equal(t, tt.want.NegativeIDs, stmt.NegativeIDs)
			assert.Equal(t, tt.want.Limit, stmt.Limit)
			assert.Equal(t, tt.want.Offset, stmt.Offset)
			if tt.want.Strategy != nil {
				require.NotNil(t, stmt.Strategy)
				assert.Equal(t, *tt.want.Strategy, *stmt.Strategy)
			}
			if tt.want.ScoreThreshold != nil {
				require.NotNil(t, stmt.ScoreThreshold)
				assert.InDelta(t, *tt.want.ScoreThreshold, *stmt.ScoreThreshold, 0.0001)
			}
			if tt.want.WithClause != nil {
				require.NotNil(t, stmt.WithClause)
				assert.Equal(t, tt.want.WithClause.HnswEf, stmt.WithClause.HnswEf)
				assert.Equal(t, tt.want.WithClause.Exact, stmt.WithClause.Exact)
				assert.Equal(t, tt.want.WithClause.Acorn, stmt.WithClause.Acorn)
				assert.Equal(t, tt.want.WithClause.IndexedOnly, stmt.WithClause.IndexedOnly)
				assert.Equal(t, tt.want.WithClause.Quantization, stmt.WithClause.Quantization)
			}
			assert.Equal(t, tt.want.LookupFrom, stmt.LookupFrom)
			if tt.want.LookupVector != nil {
				require.NotNil(t, stmt.LookupVector)
				assert.Equal(t, *tt.want.LookupVector, *stmt.LookupVector)
			}
			if tt.want.Using != nil {
				require.NotNil(t, stmt.Using)
				assert.Equal(t, *tt.want.Using, *stmt.Using)
			}
			if tt.want.QueryFilter != nil {
				require.NotNil(t, stmt.QueryFilter)
				assertFilterExprEqual(t, tt.want.QueryFilter, stmt.QueryFilter)
			}
		})
	}
}

func TestParseDelete(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.DeleteStmt
		wantErr bool
	}{
		{
			name:  "delete with string id",
			input: "DELETE FROM mycollection WHERE id = 'point-123'",
			want: &ast.DeleteStmt{
				Collection: "mycollection",
				PointID:    "point-123",
			},
		},
		{
			name:  "delete with integer id",
			input: "DELETE FROM mycollection WHERE id = 42",
			want: &ast.DeleteStmt{
				Collection: "mycollection",
				PointID:    42,
			},
		},
		{
			name:  "delete by field",
			input: "DELETE FROM mycollection WHERE status = 'archived'",
			want: &ast.DeleteStmt{
				Collection: "mycollection",
				Field:      "status",
				Value:      "archived",
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

			stmt, ok := node.(*ast.DeleteStmt)
			require.True(t, ok, "expected DeleteStmt")
			assert.Equal(t, tt.want.Collection, stmt.Collection)
			assert.Equal(t, tt.want.PointID, stmt.PointID)
			assert.Equal(t, tt.want.Field, stmt.Field)
			assert.Equal(t, tt.want.Value, stmt.Value)
		})
	}
}

func TestParseUpdate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, node ast.ASTNode)
	}{
		{
			name:  "update vector by id",
			input: "UPDATE articles SET VECTOR WHERE id = 42 [0.1, 0.2]",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.UpdateVectorStmt)
				require.True(t, ok)
				assert.Equal(t, "articles", stmt.Collection)
				assert.Equal(t, 42, stmt.PointID)
				assert.Equal(t, []float32{0.1, 0.2}, stmt.Vector)
			},
		},
		{
			name:  "update payload by filter",
			input: "UPDATE articles SET PAYLOAD WHERE category = 'draft' {'status': 'published'}",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.UpdatePayloadStmt)
				require.True(t, ok)
				assert.Equal(t, "articles", stmt.Collection)
				assert.Nil(t, stmt.PointID)
				assertFilterExprEqual(t, &ast.CompareExpr{Field: "category", Op: "=", Value: "draft"}, stmt.QueryFilter)
				assert.Equal(t, map[string]interface{}{"status": "published"}, stmt.Payload)
			},
		},
		{
			name:  "update payload by id",
			input: "UPDATE articles SET PAYLOAD WHERE id = 'abc-123' {'year': 2025}",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.UpdatePayloadStmt)
				require.True(t, ok)
				assert.Equal(t, "abc-123", stmt.PointID)
				assert.Equal(t, map[string]interface{}{"year": 2025}, stmt.Payload)
			},
		},
		{
			name:    "update vector rejects bools",
			input:   "UPDATE articles SET VECTOR WHERE id = 1 [true, 0.2]",
			wantErr: true,
		},
		{
			name:    "update rejects invalid target",
			input:   "UPDATE articles SET NAME WHERE id = 1 {'x': 1}",
			wantErr: true,
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.check(t, node)
		})
	}
}

func TestParseDocumentedExamples(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, node ast.ASTNode)
		wantErr bool
	}{
		{
			name:  "readme create hybrid collection",
			input: "CREATE COLLECTION docs HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.CreateCollectionStmt)
				require.True(t, ok, "expected CreateCollectionStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
			},
		},
		{
			name:  "readme create hybrid rerank collection",
			input: "CREATE COLLECTION docs HYBRID RERANK",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.CreateCollectionStmt)
				require.True(t, ok, "expected CreateCollectionStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.True(t, stmt.Rerank)
			},
		},
		{
			name:  "readme hybrid insert",
			input: "INSERT INTO COLLECTION docs VALUES {'text': 'Qdrant stores vectors', 'topic': 'search'} USING HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.InsertStmt)
				require.True(t, ok, "expected InsertStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.Equal(t, map[string]interface{}{"text": "Qdrant stores vectors", "topic": "search"}, stmt.Values)
			},
		},
		{
			name:  "readme hybrid search",
			input: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.SearchStmt)
				require.True(t, ok, "expected SearchStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.Equal(t, "vector database", stmt.QueryText)
				assert.Equal(t, 5, stmt.Limit)
				assert.True(t, stmt.Hybrid)
			},
		},
		{
			name:  "readme hybrid search with filter",
			input: "SEARCH notes SIMILAR TO 'vector search' LIMIT 5 USING HYBRID WHERE topic = 'search'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.SearchStmt)
				require.True(t, ok, "expected SearchStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				require.NotNil(t, stmt.QueryFilter)
				assertFilterExprEqual(t, &ast.CompareExpr{Field: "topic", Op: "=", Value: "search"}, stmt.QueryFilter)
			},
		},
		{
			name:  "readme hybrid rerank search",
			input: "SEARCH docs SIMILAR TO 'vector database' LIMIT 5 USING HYBRID RERANK",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.SearchStmt)
				require.True(t, ok, "expected SearchStmt")
				assert.Equal(t, "docs", stmt.Collection)
				assert.True(t, stmt.Hybrid)
				assert.True(t, stmt.Rerank)
			},
		},
		{
			name:  "readme delete by id",
			input: "DELETE FROM notes WHERE id = 'uuid'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.DeleteStmt)
				require.True(t, ok, "expected DeleteStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.Equal(t, "uuid", stmt.PointID)
			},
		},
		{
			name:  "readme delete by field",
			input: "DELETE FROM notes WHERE specialty = 'search'",
			check: func(t *testing.T, node ast.ASTNode) {
				stmt, ok := node.(*ast.DeleteStmt)
				require.True(t, ok, "expected DeleteStmt")
				assert.Equal(t, "notes", stmt.Collection)
				assert.Equal(t, "specialty", stmt.Field)
				assert.Equal(t, "search", stmt.Value)
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
			require.NotNil(t, tt.check)
			tt.check(t, node)
		})
	}
}

func TestParseFilterComparison(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ast.FilterExpr
	}{
		{
			name:     "equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field = 'value'",
			expected: &ast.CompareExpr{Field: "field", Op: "=", Value: "value"},
		},
		{
			name:     "not equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field != 'value'",
			expected: &ast.CompareExpr{Field: "field", Op: "!=", Value: "value"},
		},
		{
			name:     "greater than",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count > 5",
			expected: &ast.CompareExpr{Field: "count", Op: ">", Value: 5},
		},
		{
			name:     "greater than or equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count >= 5",
			expected: &ast.CompareExpr{Field: "count", Op: ">=", Value: 5},
		},
		{
			name:     "less than",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count < 10",
			expected: &ast.CompareExpr{Field: "count", Op: "<", Value: 10},
		},
		{
			name:     "less than or equals",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count <= 10",
			expected: &ast.CompareExpr{Field: "count", Op: "<=", Value: 10},
		},
		{
			name:     "equals integer",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE count = 42",
			expected: &ast.CompareExpr{Field: "count", Op: "=", Value: 42},
		},
		{
			name:     "equals float",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE score = 3.14",
			expected: &ast.CompareExpr{Field: "score", Op: "=", Value: 3.14},
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

			stmt, ok := node.(*ast.SearchStmt)
			require.True(t, ok)
			require.NotNil(t, stmt.QueryFilter)
			assertFilterExprEqual(t, tt.expected, stmt.QueryFilter)
		})
	}
}

func TestParseFilterBetween(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE age BETWEEN 18 AND 65"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	between, ok := stmt.QueryFilter.(*ast.BetweenExpr)
	require.True(t, ok)
	assert.Equal(t, "age", between.Field)
	assert.Equal(t, 18, between.Low)
	assert.Equal(t, 65, between.High)
}

func TestParseFilterIn(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE status IN ('active', 'pending')"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	inExpr, ok := stmt.QueryFilter.(*ast.InExpr)
	require.True(t, ok)
	assert.Equal(t, "status", inExpr.Field)
	assert.Equal(t, []interface{}{"active", "pending"}, inExpr.Values)
}

func TestParseFilterNotIn(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE status NOT IN ('deleted', 'archived')"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	notIn, ok := stmt.QueryFilter.(*ast.NotInExpr)
	require.True(t, ok)
	assert.Equal(t, "status", notIn.Field)
	assert.Equal(t, []interface{}{"deleted", "archived"}, notIn.Values)
}

func TestParseFilterIsNull(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS NULL"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNull, ok := stmt.QueryFilter.(*ast.IsNullExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNull.Field)
}

func TestParseFilterIsNotNull(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS NOT NULL"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNotNull, ok := stmt.QueryFilter.(*ast.IsNotNullExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNotNull.Field)
}

func TestParseFilterIsEmpty(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS EMPTY"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isEmpty, ok := stmt.QueryFilter.(*ast.IsEmptyExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isEmpty.Field)
}

func TestParseFilterIsNotEmpty(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE field IS NOT EMPTY"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	isNotEmpty, ok := stmt.QueryFilter.(*ast.IsNotEmptyExpr)
	require.True(t, ok)
	assert.Equal(t, "field", isNotEmpty.Field)
}

func TestParseFilterMatch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ast.FilterExpr
	}{
		{
			name:     "match text",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE content MATCH 'hello world'",
			expected: &ast.MatchTextExpr{Field: "content", Text: "hello world"},
		},
		{
			name:     "match any",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE content MATCH ANY 'hello world'",
			expected: &ast.MatchAnyExpr{Field: "content", Text: "hello world"},
		},
		{
			name:     "match phrase",
			input:    "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE content MATCH PHRASE 'hello world'",
			expected: &ast.MatchPhraseExpr{Field: "content", Text: "hello world"},
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

			stmt, ok := node.(*ast.SearchStmt)
			require.True(t, ok)
			require.NotNil(t, stmt.QueryFilter)
			assertFilterExprEqual(t, tt.expected, stmt.QueryFilter)
		})
	}
}

func TestParseFilterAnd(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE a = 1 AND b = 2"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	andExpr, ok := stmt.QueryFilter.(*ast.AndExpr)
	require.True(t, ok)
	require.Len(t, andExpr.Operands, 2)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, andExpr.Operands[0])
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "b", Op: "=", Value: 2}, andExpr.Operands[1])
}

func TestParseFilterOr(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE a = 1 OR b = 2"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	require.Len(t, orExpr.Operands, 2)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, orExpr.Operands[0])
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "b", Op: "=", Value: 2}, orExpr.Operands[1])
}

func TestParseFilterNot(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE NOT a = 1"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	notExpr, ok := stmt.QueryFilter.(*ast.NotExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.CompareExpr{Field: "a", Op: "=", Value: 1}, notExpr.Operand)
}

func TestParseFilterComplex(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE (a = 1 AND b = 2) OR (c = 3 AND NOT d = 4)"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "a", Op: "=", Value: 1},
					&ast.CompareExpr{Field: "b", Op: "=", Value: 2},
				},
			},
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "c", Op: "=", Value: 3},
					&ast.NotExpr{
						Operand: &ast.CompareExpr{Field: "d", Op: "=", Value: 4},
					},
				},
			},
		},
	}, orExpr)
}

func TestParseFilterPrecedence(t *testing.T) {
	input := "SEARCH c SIMILAR TO 'text' LIMIT 10 WHERE a = 1 AND b = 2 OR c = 3"
	l := &lexer.Lexer{}
	tokens, err := l.Tokenize(input)
	require.NoError(t, err)

	p := NewParser()
	node, err := p.Parse(tokens)
	require.NoError(t, err)

	stmt, ok := node.(*ast.SearchStmt)
	require.True(t, ok)
	require.NotNil(t, stmt.QueryFilter)

	orExpr, ok := stmt.QueryFilter.(*ast.OrExpr)
	require.True(t, ok)
	assertFilterExprEqual(t, &ast.OrExpr{
		Operands: []ast.FilterExpr{
			&ast.AndExpr{
				Operands: []ast.FilterExpr{
					&ast.CompareExpr{Field: "a", Op: "=", Value: 1},
					&ast.CompareExpr{Field: "b", Op: "=", Value: 2},
				},
			},
			&ast.CompareExpr{Field: "c", Op: "=", Value: 3},
		},
	}, orExpr)
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid statement",
			input:   "INVALID KEYWORD",
			wantErr: true,
		},
		{
			name:    "insert missing values",
			input:   "INSERT INTO COLLECTION test",
			wantErr: true,
		},
		{
			name:    "search missing limit",
			input:   "SEARCH test SIMILAR TO 'text'",
			wantErr: true,
		},
		{
			name:    "search missing query text",
			input:   "SEARCH test SIMILAR TO LIMIT 10",
			wantErr: true,
		},
		{
			name:    "search with invalid with boolean",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WITH {exact: maybe}",
			wantErr: true,
		},
		{
			name:    "reject trailing tokens",
			input:   "INSERT INTO COLLECTION test VALUES {\"text\": \"hello\"} EXTRA",
			wantErr: true,
		},
		{
			name:    "reject explain in parser",
			input:   "EXPLAIN SEARCH test SIMILAR TO 'text' LIMIT 10",
			wantErr: true,
		},
		{
			name:    "reject overflowing limit",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "reject overflowing integer literal",
			input:   "DELETE FROM test WHERE id = 999999999999999999999999999",
			wantErr: true,
		},
		{
			name:    "reject overflowing float literal",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WHERE score = " + strings.Repeat("9", 400) + ".0",
			wantErr: true,
		},
		{
			name:    "reject duplicate where clause",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WHERE a = 1 WHERE b = 2",
			wantErr: true,
		},
		{
			name:    "reject duplicate with clause",
			input:   "SEARCH test SIMILAR TO 'text' LIMIT 10 WITH {exact: true} WITH {acorn: true}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &lexer.Lexer{}
			tokens, err := l.Tokenize(tt.input)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Fatalf("lexer error: %v", err)
			}

			p := NewParser()
			_, err = p.Parse(tokens)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(v int) *int {
	return &v
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func assertFilterExprEqual(t *testing.T, expected, actual ast.FilterExpr) {
	t.Helper()

	switch e := expected.(type) {
	case *ast.CompareExpr:
		a, ok := actual.(*ast.CompareExpr)
		require.True(t, ok, "expected CompareExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Op, a.Op)
		assert.Equal(t, e.Value, a.Value)
	case *ast.BetweenExpr:
		a, ok := actual.(*ast.BetweenExpr)
		require.True(t, ok, "expected BetweenExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Low, a.Low)
		assert.Equal(t, e.High, a.High)
	case *ast.InExpr:
		a, ok := actual.(*ast.InExpr)
		require.True(t, ok, "expected InExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Values, a.Values)
	case *ast.NotInExpr:
		a, ok := actual.(*ast.NotInExpr)
		require.True(t, ok, "expected NotInExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Values, a.Values)
	case *ast.IsNullExpr:
		a, ok := actual.(*ast.IsNullExpr)
		require.True(t, ok, "expected IsNullExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsNotNullExpr:
		a, ok := actual.(*ast.IsNotNullExpr)
		require.True(t, ok, "expected IsNotNullExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsEmptyExpr:
		a, ok := actual.(*ast.IsEmptyExpr)
		require.True(t, ok, "expected IsEmptyExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.IsNotEmptyExpr:
		a, ok := actual.(*ast.IsNotEmptyExpr)
		require.True(t, ok, "expected IsNotEmptyExpr")
		assert.Equal(t, e.Field, a.Field)
	case *ast.MatchTextExpr:
		a, ok := actual.(*ast.MatchTextExpr)
		require.True(t, ok, "expected MatchTextExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.MatchAnyExpr:
		a, ok := actual.(*ast.MatchAnyExpr)
		require.True(t, ok, "expected MatchAnyExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.MatchPhraseExpr:
		a, ok := actual.(*ast.MatchPhraseExpr)
		require.True(t, ok, "expected MatchPhraseExpr")
		assert.Equal(t, e.Field, a.Field)
		assert.Equal(t, e.Text, a.Text)
	case *ast.AndExpr:
		a, ok := actual.(*ast.AndExpr)
		require.True(t, ok, "expected AndExpr")
		require.Len(t, a.Operands, len(e.Operands))
		for i := range e.Operands {
			assertFilterExprEqual(t, e.Operands[i], a.Operands[i])
		}
	case *ast.OrExpr:
		a, ok := actual.(*ast.OrExpr)
		require.True(t, ok, "expected OrExpr")
		require.Len(t, a.Operands, len(e.Operands))
		for i := range e.Operands {
			assertFilterExprEqual(t, e.Operands[i], a.Operands[i])
		}
	case *ast.NotExpr:
		a, ok := actual.(*ast.NotExpr)
		require.True(t, ok, "expected NotExpr")
		assertFilterExprEqual(t, e.Operand, a.Operand)
	default:
		t.Fatalf("unexpected type %T", expected)
	}
}

func TestParseSelect(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.SelectStmt
		wantErr bool
	}{
		{
			name:  "select with string id",
			input: `SELECT * FROM docs WHERE id = 'point-123'`,
			want: &ast.SelectStmt{
				Collection: "docs",
				PointID:    "point-123",
			},
		},
		{
			name:  "select with integer id",
			input: `SELECT * FROM docs WHERE id = 42`,
			want: &ast.SelectStmt{
				Collection: "docs",
				PointID:    42,
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, ok := node.(*ast.SelectStmt)
			require.True(t, ok, "expected SelectStmt, got %T", node)
			assert.Equal(t, tt.want.Collection, got.Collection)
			assert.Equal(t, tt.want.PointID, got.PointID)
		})
	}
}

func TestParseScroll(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.ScrollStmt
		wantErr bool
	}{
		{
			name:  "basic scroll",
			input: `SCROLL FROM docs LIMIT 10`,
			want: &ast.ScrollStmt{
				Collection: "docs",
				Limit:      10,
			},
		},
		{
			name:  "scroll with where",
			input: `SCROLL FROM docs WHERE status = 'active' LIMIT 5`,
			want: &ast.ScrollStmt{
				Collection:  "docs",
				Limit:       5,
				QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
			},
		},
		{
			name:  "scroll with after",
			input: `SCROLL FROM docs AFTER 'point-123' LIMIT 10`,
			want: &ast.ScrollStmt{
				Collection: "docs",
				Limit:      10,
				After:      "point-123",
			},
		},
		{
			name:  "scroll with after integer",
			input: `SCROLL FROM docs AFTER 42 LIMIT 10`,
			want: &ast.ScrollStmt{
				Collection: "docs",
				Limit:      10,
				After:      42,
			},
		},
		{
			name:  "scroll with where and after",
			input: `SCROLL FROM docs WHERE status = 'active' AFTER 'point-50' LIMIT 20`,
			want: &ast.ScrollStmt{
				Collection:  "docs",
				Limit:       20,
				QueryFilter: &ast.CompareExpr{Field: "status", Op: "=", Value: "active"},
				After:       "point-50",
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
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, ok := node.(*ast.ScrollStmt)
			require.True(t, ok, "expected ScrollStmt, got %T", node)
			assert.Equal(t, tt.want.Collection, got.Collection)
			assert.Equal(t, tt.want.Limit, got.Limit)
			assert.Equal(t, tt.want.After, got.After)
			if tt.want.QueryFilter != nil {
				assertFilterExprEqual(t, tt.want.QueryFilter, got.QueryFilter)
			}
		})
	}
}

func TestParseSearchHybridWithFusion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFusion *string
	}{
		{
			name:       "rrf fusion",
			input:      `SEARCH docs SIMILAR TO 'query' LIMIT 5 USING HYBRID FUSION 'rrf'`,
			wantFusion: strPtr("rrf"),
		},
		{
			name:       "dbsf fusion",
			input:      `SEARCH docs SIMILAR TO 'query' LIMIT 5 USING HYBRID FUSION 'dbsf'`,
			wantFusion: strPtr("dbsf"),
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
			search, ok := node.(*ast.SearchStmt)
			require.True(t, ok, "expected SearchStmt, got %T", node)
			assert.True(t, search.Hybrid)
			if tt.wantFusion != nil {
				require.NotNil(t, search.Fusion)
				assert.Equal(t, *tt.wantFusion, *search.Fusion)
			} else {
				assert.Nil(t, search.Fusion)
			}
		})
	}

	t.Run("invalid fusion", func(t *testing.T) {
		l := &lexer.Lexer{}
		tokens, err := l.Tokenize(`SEARCH docs SIMILAR TO 'query' LIMIT 5 USING HYBRID FUSION 'invalid'`)
		require.NoError(t, err)
		_, err = NewParser().Parse(tokens)
		require.Error(t, err)
	})
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
			name:     "turbo without bits",
			input:    `CREATE COLLECTION docs QUANTIZE TURBO`,
			wantType: ast.QuantizationTypeTurbo,
		},
		{
			name:     "turbo bits 4",
			input:    `CREATE COLLECTION docs QUANTIZE TURBO BITS 4`,
			wantType: ast.QuantizationTypeTurbo,
			wantBits: float64Ptr(4.0),
		},
		{
			name:     "turbo bits 2 always ram",
			input:    `CREATE COLLECTION docs QUANTIZE TURBO BITS 2 ALWAYS RAM`,
			wantType: ast.QuantizationTypeTurbo,
			wantBits: float64Ptr(2.0),
			wantRAM:  true,
		},
		{
			name:     "turbo bits 1.5",
			input:    `CREATE COLLECTION docs QUANTIZE TURBO BITS 1.5`,
			wantType: ast.QuantizationTypeTurbo,
			wantBits: float64Ptr(1.5),
		},
		{
			name:     "turbo bits 1 always ram",
			input:    `CREATE COLLECTION docs QUANTIZE TURBO BITS 1 ALWAYS RAM`,
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
			require.NotNil(t, create.Quantization)
			assert.Equal(t, tt.wantType, create.Quantization.Type)
			assert.Equal(t, tt.wantRAM, create.Quantization.AlwaysRAM)
			if tt.wantBits != nil {
				require.NotNil(t, create.Quantization.TurboBits)
				assert.InDelta(t, *tt.wantBits, *create.Quantization.TurboBits, 0.0001)
			} else {
				assert.Nil(t, create.Quantization.TurboBits)
			}
		})
	}

	t.Run("invalid turbo bits", func(t *testing.T) {
		l := &lexer.Lexer{}
		tokens, err := l.Tokenize(`CREATE COLLECTION docs QUANTIZE TURBO BITS 3`)
		require.NoError(t, err)
		_, err = NewParser().Parse(tokens)
		require.Error(t, err)
		require.Contains(t, err.Error(), "BITS must be one of 1, 1.5, 2, or 4")
	})
}

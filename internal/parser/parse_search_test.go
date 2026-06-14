package parser

import (
	"testing"

	"github.com/srimon12/qql-go/internal/ast"
	"github.com/srimon12/qql-go/internal/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		{
			name:  "search with offset and score threshold",
			input: "SEARCH notes SIMILAR TO 'hi' LIMIT 5 OFFSET 10 SCORE THRESHOLD 0.8",
			want: &ast.SearchStmt{
				Collection:     "notes",
				QueryText:      "hi",
				Limit:          5,
				Offset:         10,
				ScoreThreshold: float64Ptr(0.8),
			},
		},
		{
			name:  "search with integer score threshold",
			input: "SEARCH notes SIMILAR TO 'hi' LIMIT 5 SCORE THRESHOLD 1",
			want: &ast.SearchStmt{
				Collection:     "notes",
				QueryText:      "hi",
				Limit:          5,
				ScoreThreshold: float64Ptr(1.0),
			},
		},
		{
			name:  "search with lookup from",
			input: "SEARCH notes SIMILAR TO 'hi' LIMIT 5 LOOKUP FROM other_coll",
			want: &ast.SearchStmt{
				Collection: "notes",
				QueryText:  "hi",
				Limit:      5,
				LookupFrom: "other_coll",
			},
		},
		{
			name:  "search with lookup from and vector",
			input: "SEARCH notes SIMILAR TO 'hi' LIMIT 5 LOOKUP FROM other_coll VECTOR 'my_vec'",
			want: &ast.SearchStmt{
				Collection:   "notes",
				QueryText:    "hi",
				Limit:        5,
				LookupFrom:   "other_coll",
				LookupVector: strPtr("my_vec"),
			},
		},
		{
			name:  "search with lookup before using model",
			input: "SEARCH notes SIMILAR TO 'hi' LIMIT 5 LOOKUP FROM other_coll USING MODEL 'dense-model'",
			want: &ast.SearchStmt{
				Collection: "notes",
				QueryText:  "hi",
				Limit:      5,
				LookupFrom: "other_coll",
				Model:      strPtr("dense-model"),
			},
		},
		{
			name:  "search with offset score threshold and lookup",
			input: "SEARCH notes SIMILAR TO 'hi' LIMIT 5 OFFSET 10 SCORE THRESHOLD 0.8 LOOKUP FROM other_coll",
			want: &ast.SearchStmt{
				Collection:     "notes",
				QueryText:      "hi",
				Limit:          5,
				Offset:         10,
				ScoreThreshold: float64Ptr(0.8),
				LookupFrom:     "other_coll",
			},
		},
		{
			name:    "search with negative offset raises error",
			input:   "SEARCH notes SIMILAR TO 'hi' LIMIT 5 OFFSET -1",
			wantErr: true,
		},
		{
			name:    "search group by with offset raises error",
			input:   "SEARCH notes SIMILAR TO 'hi' LIMIT 5 OFFSET 10 GROUP BY author",
			wantErr: true,
		},
		{
			name:    "search group by with zero offset raises error",
			input:   "SEARCH notes SIMILAR TO 'hi' LIMIT 5 OFFSET 0 GROUP BY author",
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
			assert.Equal(t, tt.want.Offset, stmt.Offset)
			if tt.want.ScoreThreshold != nil {
				require.NotNil(t, stmt.ScoreThreshold)
				assert.InDelta(t, *tt.want.ScoreThreshold, *stmt.ScoreThreshold, 0.0001)
			}
			assert.Equal(t, tt.want.LookupFrom, stmt.LookupFrom)
			if tt.want.LookupVector != nil {
				require.NotNil(t, stmt.LookupVector)
				assert.Equal(t, *tt.want.LookupVector, *stmt.LookupVector)
			}
		})
	}
}

func TestParseRecommend(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ast.RecommendStmt
		wantErr bool
	}{
		{
			name:  "basic recommend",
			input: "RECOMMEND FROM docs POSITIVE IDS ('seed-1', 'seed-2') NEGATIVE IDS ('seed-3') STRATEGY 'average' LIMIT 5 WHERE score > 0",
			want: &ast.RecommendStmt{
				Collection:  "docs",
				PositiveIDs: []any{"seed-1", "seed-2"},
				NegativeIDs: []any{"seed-3"},
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
				PositiveIDs: []any{"a"},
				Limit:       10,
				Offset:      5,
			},
		},
		{
			name:  "recommend with score threshold",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 SCORE THRESHOLD 0.5",
			want: &ast.RecommendStmt{
				Collection:     "docs",
				PositiveIDs:    []any{"a"},
				Limit:          10,
				ScoreThreshold: float64Ptr(0.5),
			},
		},
		{
			name:  "recommend with score threshold integer",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 SCORE THRESHOLD 1",
			want: &ast.RecommendStmt{
				Collection:     "docs",
				PositiveIDs:    []any{"a"},
				Limit:          10,
				ScoreThreshold: float64Ptr(1.0),
			},
		},
		{
			name:  "recommend with with clause",
			input: "RECOMMEND FROM docs POSITIVE IDS ('a') LIMIT 10 WITH {exact: true, hnsw_ef: 128, indexed_only: true, quantization: {rescore: true}}",
			want: &ast.RecommendStmt{
				Collection:  "docs",
				PositiveIDs: []any{"a"},
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
				PositiveIDs: []any{"a"},
				Limit:       5,
				LookupFrom:  "source_collection",
			},
		},
		{
			name:  "recommend with lookup from and vector",
			input: "RECOMMEND FROM target_collection POSITIVE IDS ('a') LOOKUP FROM source_collection VECTOR 'dense' LIMIT 5",
			want: &ast.RecommendStmt{
				Collection:   "target_collection",
				PositiveIDs:  []any{"a"},
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
				PositiveIDs: []any{"a"},
				Limit:       5,
				Using:       strPtr("sparse"),
			},
		},
		{
			name:  "recommend with lookup from and using",
			input: "RECOMMEND FROM target_collection POSITIVE IDS ('a') LOOKUP FROM source_collection VECTOR 'dense' USING 'sparse' LIMIT 5",
			want: &ast.RecommendStmt{
				Collection:   "target_collection",
				PositiveIDs:  []any{"a"},
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
				PositiveIDs:    []any{"a", "b"},
				NegativeIDs:    []any{"c"},
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
		{
			name:    "recommend with negative offset raises error",
			input:   "RECOMMEND FROM notes POSITIVE IDS ('a') LIMIT 10 OFFSET -1",
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

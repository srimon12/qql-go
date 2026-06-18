package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertUpsert(t *testing.T) {
	input := `{"points":[{"id":1,"payload":{"text":"hello","topic":"search"}},{"id":2,"payload":{"text":"world"}}]}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 2)
	assert.Contains(t, stmts[0], "INSERT INTO")
	assert.Contains(t, stmts[0], "'text': 'hello'")
	assert.Contains(t, stmts[0], "'id': 1")
	assert.Contains(t, stmts[1], "'id': 2")
}

func TestConvertSearchWithFilter(t *testing.T) {
	input := `{"vector":[0.1,0.2],"limit":10,"filter":{"must":[{"key":"status","match":{"value":"active"}}]},"score_threshold":0.5}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "QUERY [0.1, 0.2] FROM collection")
	assert.Contains(t, stmts[0], "WHERE status = 'active'")
	assert.Contains(t, stmts[0], "SCORE THRESHOLD 0.5")
}

func TestConvertRecommend(t *testing.T) {
	input := `{"positive":["id-1","id-2"],"negative":["id-3"],"limit":5,"strategy":"best_score"}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "QUERY RECOMMEND WITH (positive = ('id-1', 'id-2'), negative = ('id-3'))")
	assert.Contains(t, stmts[0], "LIMIT 5")
	assert.Contains(t, stmts[0], "STRATEGY 'best_score'")
}

func TestConvertDiscover(t *testing.T) {
	input := `{"target":"id-1","context":[{"positive":"id-2","negative":"id-3"}],"limit":5}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "QUERY DISCOVER TARGET 'id-1'")
	assert.Contains(t, stmts[0], "CONTEXT PAIRS ('id-2', 'id-3')")
	assert.Contains(t, stmts[0], "LIMIT 5")
}

func TestConvertScroll(t *testing.T) {
	input := `{"limit":10,"filter":{"must":[{"key":"year","range":{"gte":2024,"lte":2026}}]}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "SCROLL FROM collection")
	assert.Contains(t, stmts[0], "WHERE year BETWEEN 2024 AND 2026")
	assert.Contains(t, stmts[0], "LIMIT 10")
}

func TestConvertGetPoints(t *testing.T) {
	input := `{"ids":[1,"uuid-123"]}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 2)
	assert.Contains(t, stmts[0], "SELECT * FROM collection WHERE id = 1")
	assert.Contains(t, stmts[1], "SELECT * FROM collection WHERE id = 'uuid-123'")
}

func TestConvertDeletePoints(t *testing.T) {
	input := `{"points":[1,2,3]}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 3)
	assert.Contains(t, stmts[0], "DELETE FROM collection WHERE id = 1")
}

func TestConvertCreateCollection(t *testing.T) {
	input := `{"vectors":{"size":384,"distance":"Cosine"}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "CREATE COLLECTION collection")
	assert.Contains(t, stmts[0], "VECTOR(384, Cosine)")
}

func TestConvertCreateIndex(t *testing.T) {
	input := `{"field_name":"status","field_schema":"keyword"}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "CREATE INDEX ON collection FOR status TYPE keyword")
}

func TestConvertWrappedRequest(t *testing.T) {
	input := `{"method":"POST","path":"/collections/docs/points/search","body":{"vector":[0.1],"limit":5}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "FROM docs")
	assert.Contains(t, stmts[0], "LIMIT 5")
}

func TestConvertComplexFilter(t *testing.T) {
	input := `{"vector":[0.1],"limit":5,"filter":{"must":[{"key":"status","match":{"value":"active"}}],"should":[{"key":"priority","match":{"value":"high"}},{"key":"priority","match":{"value":"medium"}}],"must_not":[{"key":"archived","match":{"boolean":true}}]}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "status = 'active'")
	assert.Contains(t, stmts[0], "OR")
	assert.Contains(t, stmts[0], "NOT archived = true")
}

func TestConvertSetPayload(t *testing.T) {
	input := `{"payload":{"status":"reviewed"},"points":[1,2]}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 2)
	assert.Contains(t, stmts[0], "UPDATE collection SET PAYLOAD = {'status': 'reviewed'} WHERE id = 1")
}

func TestConvertSetPayloadByFilter(t *testing.T) {
	input := `{"payload":{"status":"archived"},"filter":{"must":[{"key":"status","match":{"value":"discharged"}}]}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "UPDATE collection SET PAYLOAD =")
	assert.Contains(t, stmts[0], "WHERE status = 'discharged'")
}

func TestConvertInvalidJSON(t *testing.T) {
	_, err := JSONToQQL("not json")
	require.Error(t, err)
}

func TestConvertEmptyJSON(t *testing.T) {
	_, err := JSONToQQL("{}")
	require.Error(t, err)
}

func TestConvertFormulaScoreBoost(t *testing.T) {
	input := `{"prefetch":{"query":[0.2,0.8],"limit":50},"query":{"formula":{"sum":["$score",{"mult":[0.5,{"key":"tag","match":{"any":["h1","h2"]}}]}]}},"limit":10}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "BOOST")
	assert.Contains(t, stmts[0], "$score")
	assert.Contains(t, stmts[0], "MATCH(tag, ['h1', 'h2'])")
	assert.Contains(t, stmts[0], "LIMIT 10")
}

func TestConvertFormulaGeoDecay(t *testing.T) {
	input := `{"prefetch":{"query":[0.2],"limit":50},"query":{"formula":{"sum":["$score",{"gauss_decay":{"x":{"geo_distance":{"origin":{"lat":52.5,"lon":13.3},"to":"geo.location"}},"scale":5000}}]}},"defaults":{"geo.location":{"lat":48.1,"lon":11.5}}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "GAUSS_DECAY")
	assert.Contains(t, stmts[0], "GEO_DISTANCE")
	assert.Contains(t, stmts[0], "scale: 5000")
	assert.Contains(t, stmts[0], "DEFAULTS")
	assert.Contains(t, stmts[0], "geo.location = {lat: 48.1, lon: 11.5}")
}

func TestConvertFormulaTimeDecay(t *testing.T) {
	input := `{"query":{"formula":{"sum":["$score",{"exp_decay":{"x":{"datetime_key":"updated"},"target":{"datetime":"2026-01-01"},"scale":86400,"midpoint":0.5}}]}}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "EXP_DECAY")
	assert.Contains(t, stmts[0], "datetime_key('updated')")
	assert.Contains(t, stmts[0], "datetime('2026-01-01')")
	assert.Contains(t, stmts[0], "scale: 86400")
}

func TestConvertMMRQuery(t *testing.T) {
	input := `{"query":{"nearest":[0.01,0.45],"mmr":{"diversity":0.5,"candidates_limit":100}},"limit":10}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "mmr_diversity = 0.5")
	assert.Contains(t, stmts[0], "mmr_candidates = 100")
	assert.Contains(t, stmts[0], "LIMIT 10")
}

func TestConvertRelevanceFeedback(t *testing.T) {
	input := `{"query":{"relevance_feedback":{"target":[0.1,0.9],"feedback":[{"example":111,"score":0.68},{"example":222,"score":0.72}],"strategy":{"naive":{"a":0.12,"b":0.43,"c":0.03}}}}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "QUERY RELEVANCE FEEDBACK")
	assert.Contains(t, stmts[0], "FEEDBACK ((111, 0.68), (222, 0.72))")
	assert.Contains(t, stmts[0], "STRATEGY NAIVE (a = 0.12, b = 0.43, c = 0.03)")
}

func TestConvertEndpointFormula(t *testing.T) {
	input := `{"method":"POST","path":"/collections/docs/points/query","body":{"query":{"formula":{"sum":["$score",{"mult":[2,{"key":"priority","match":{"value":"high"}}]}]}},"limit":5}}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "FROM docs")
	assert.Contains(t, stmts[0], "BOOST")
	assert.Contains(t, stmts[0], "LIMIT 5")
}

func TestConvertFormulaMultiMatch(t *testing.T) {
	input := `{
		"prefetch": {"query": [0.2, 0.8, 0.3], "limit": 50},
		"query": {
			"formula": {
				"sum": [
					"$score",
					{"mult": [0.5, {"key": "tag", "match": {"any": ["h1", "h2", "h3", "h4"]}}]},
					{"mult": [0.25, {"key": "tag", "match": {"any": ["p", "li"]}}]}
				]
			}
		},
		"limit": 10
	}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "BOOST")
	assert.Contains(t, stmts[0], "$score")
	assert.Contains(t, stmts[0], "0.5 * MATCH(tag, ['h1', 'h2', 'h3', 'h4'])")
	assert.Contains(t, stmts[0], "0.25 * MATCH(tag, ['p', 'li'])")
	assert.Contains(t, stmts[0], "PREFETCH")
	assert.Contains(t, stmts[0], "_pf0")
	assert.Contains(t, stmts[0], "LIMIT 10")
}

func TestConvertFormulaWithDocumentPrefetch(t *testing.T) {
	input := `{
		"prefetch": {"document": {"text": "machine learning basics", "model": "all-MiniLM-L6-v2"}, "limit": 50},
		"query": {"formula": {"mult": ["$score", 2.0]}},
		"limit": 10
	}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "machine learning basics")
	assert.Contains(t, stmts[0], "BOOST")
	assert.Contains(t, stmts[0], "PREFETCH")
}

func TestConvertFormulaRawVectorPrefetch(t *testing.T) {
	input := `{
		"prefetch": {"query": [0.2, 0.8, 0.3], "limit": 50},
		"query": {"formula": {"sum": ["$score", {"mult": [0.5, {"key": "tag", "match": {"any": ["h1", "h2"]}}]}]}},
		"limit": 10
	}`
	stmts, err := JSONToQQL(input)
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	assert.Contains(t, stmts[0], "QUERY [0.2, 0.8, 0.3]")
	assert.Contains(t, stmts[0], "BOOST")
	assert.Contains(t, stmts[0], "PREFETCH")
}

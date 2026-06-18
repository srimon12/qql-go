package convert

import (
	"encoding/json"
)

type RESTQueryRequest struct {
	Prefetch json.RawMessage        `json:"prefetch"`
	Query    RESTQuery              `json:"query"`
	Filter   *RESTFilter            `json:"filter"`
	Limit    *int                   `json:"limit"`
	Defaults map[string]interface{} `json:"defaults"`
}

type RESTPrefetch struct {
	Prefetch json.RawMessage `json:"prefetch"`
	Query    RESTQuery       `json:"query"`
	Document interface{}     `json:"document"`
	Vector   interface{}     `json:"vector"`
	Filter   *RESTFilter     `json:"filter"`
	Limit    *int            `json:"limit"`
}

type RESTQuery struct {
	Formula           json.RawMessage        `json:"formula"`
	Nearest           interface{}            `json:"nearest"`
	Document          interface{}            `json:"document"`
	Recommend         *RESTRecommend         `json:"recommend"`
	Discover          *RESTDiscover          `json:"discover"`
	Context           []RESTContextPair      `json:"context"`
	RelevanceFeedback *RESTRelevanceFeedback `json:"relevance_feedback"`
}

func (q *RESTQuery) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	
	// If it's an array, it's a raw nearest vector
	if len(data) > 0 && data[0] == '[' {
		var arr []interface{}
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		q.Nearest = arr
		return nil
	}
	
	// If it's a string, it could be a point ID or raw query text (if we allow that)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		q.Nearest = s
		return nil
	}

	// Try standard struct mapping
	type Alias RESTQuery
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*q = RESTQuery(alias)

	// If Nearest wasn't set inside the object but it IS an object (like `{"nearest": ...}`), alias unmarshal works.
	// But what if the object itself is the nearest vector? Qdrant sometimes allows named vectors `{"name": "vec", "vector": [0.1, 0.2]}` as query.
	// We handle standard RESTQuery fields here.
	return nil
}

type RESTRecommend struct {
	Positive []interface{} `json:"positive"`
	Negative []interface{} `json:"negative"`
	Strategy string        `json:"strategy"`
}

type RESTDiscover struct {
	Target  interface{}       `json:"target"`
	Context []RESTContextPair `json:"context"`
}

type RESTContextPair struct {
	Positive interface{} `json:"positive"`
	Negative interface{} `json:"negative"`
}

type RESTRelevanceFeedback struct {
	Target   interface{} `json:"target"`
	Feedback []struct {
		Example interface{} `json:"example"`
		Score   float64     `json:"score"`
	} `json:"feedback"`
	Strategy *RESTFeedbackStrategy `json:"strategy"`
}

type RESTFeedbackStrategy struct {
	Naive *struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
		C float64 `json:"c"`
	} `json:"naive"`
}

type RESTFilter struct {
	Must    []RESTCondition `json:"must"`
	Should  []RESTCondition `json:"should"`
	MustNot []RESTCondition `json:"must_not"`
}

type RESTCondition struct {
	HasID   []interface{}          `json:"has_id"`
	IsEmpty *RESTBoolVal           `json:"is_empty"`
	IsNull  *RESTBoolVal           `json:"is_null"`
	Key     string                 `json:"key"`
	Match   map[string]interface{} `json:"match"`
	Range   map[string]interface{} `json:"range"`
	Must    []RESTCondition        `json:"must"`
	Should  []RESTCondition        `json:"should"`
	MustNot []RESTCondition        `json:"must_not"`
}

type RESTBoolVal struct {
	Value bool `json:"value"`
}

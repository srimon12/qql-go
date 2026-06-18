package convert

import (
	"encoding/json"
)

type RESTQueryRequest struct {
	Prefetch    RESTPrefetchList `json:"prefetch"`
	Query       RESTQuery        `json:"query"`
	Filter      *RESTFilter      `json:"filter"`
	Limit       *int             `json:"limit"`
	Offset      *int             `json:"offset"`
	Defaults    map[string]any   `json:"defaults"`
	Using       string           `json:"using"`
	WithPayload any              `json:"with_payload"`
}

type RESTPrefetch struct {
	Prefetch       RESTPrefetchList `json:"prefetch"`
	Query          RESTQuery        `json:"query"`
	Document       any              `json:"document"`
	Vector         any              `json:"vector"`
	Filter         *RESTFilter      `json:"filter"`
	Limit          *int             `json:"limit"`
	Using          string           `json:"using"`
	ScoreThreshold *float64         `json:"score_threshold"`
}

type RESTPrefetchList []RESTPrefetch

func (l *RESTPrefetchList) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			var arr []RESTPrefetch
			if err := json.Unmarshal(data, &arr); err != nil {
				return err
			}
			*l = arr
			return nil
		default:
			var single RESTPrefetch
			if err := json.Unmarshal(data, &single); err != nil {
				return err
			}
			*l = []RESTPrefetch{single}
			return nil
		}
	}
	return nil
}

type RESTQuery struct {
	Formula           json.RawMessage        `json:"formula"`
	Nearest           interface{}            `json:"nearest"`
	Document          interface{}            `json:"document"`
	Text              string                 `json:"text"`
	Model             string                 `json:"model"`
	Recommend         *RESTRecommend         `json:"recommend"`
	Discover          *RESTDiscover          `json:"discover"`
	Context           []RESTContextPair      `json:"context"`
	RelevanceFeedback *RESTRelevanceFeedback `json:"relevance_feedback"`
}

func (q *RESTQuery) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			var arr []any
			if err := json.Unmarshal(data, &arr); err != nil {
				return err
			}
			q.Nearest = arr
			return nil
		case '"':
			var s string
			if err := json.Unmarshal(data, &s); err != nil {
				return err
			}
			q.Nearest = s
			return nil
		default:
			type Alias RESTQuery
			var alias Alias
			if err := json.Unmarshal(data, &alias); err != nil {
				return err
			}
			*q = RESTQuery(alias)
			return nil
		}
	}

	// If Nearest wasn't set inside the object but it IS an object (like `{"nearest": ...}`), alias unmarshal works.
	// But what if the object itself is the nearest vector? Qdrant sometimes allows named vectors `{"name": "vec", "vector": [0.1, 0.2]}` as query.
	// We handle standard RESTQuery fields here.
	return nil
}

type RESTRecommend struct {
	Positive []any  `json:"positive"`
	Negative []any  `json:"negative"`
	Strategy string `json:"strategy"`
}

type RESTDiscover struct {
	Target  any               `json:"target"`
	Context []RESTContextPair `json:"context"`
}

type RESTContextPair struct {
	Positive any `json:"positive"`
	Negative any `json:"negative"`
}

type RESTRelevanceFeedback struct {
	Target   any `json:"target"`
	Feedback []struct {
		Example any     `json:"example"`
		Score   float64 `json:"score"`
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
	Must    RESTConditionList `json:"must"`
	Should  RESTConditionList `json:"should"`
	MustNot RESTConditionList `json:"must_not"`
}

type RESTConditionList []RESTCondition

func (l *RESTConditionList) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '[':
			var arr []RESTCondition
			if err := json.Unmarshal(data, &arr); err != nil {
				return err
			}
			*l = arr
			return nil
		default:
			var single RESTCondition
			if err := json.Unmarshal(data, &single); err != nil {
				return err
			}
			*l = []RESTCondition{single}
			return nil
		}
	}
	return nil
}

type RESTCondition struct {
	HasID          []any                `json:"has_id"`
	IsEmpty        *RESTKeyCondition    `json:"is_empty"`
	IsNull         *RESTKeyCondition    `json:"is_null"`
	Key            string               `json:"key"`
	Match          map[string]any       `json:"match"`
	Range          map[string]any       `json:"range"`
	Must           []RESTCondition      `json:"must"`
	Should         []RESTCondition      `json:"should"`
	MustNot        []RESTCondition      `json:"must_not"`
	Nested         *RESTNestedCondition `json:"nested"`
	GeoBoundingBox *RESTGeoBox          `json:"geo_bounding_box"`
	GeoRadius      *RESTGeoRadius       `json:"geo_radius"`
	ValuesCount    map[string]any       `json:"values_count"`
	HasVector      *RESTHasVector       `json:"has_vector"`
}

type RESTNestedCondition struct {
	Key    json.RawMessage `json:"key"`
	Filter json.RawMessage `json:"filter"`
}

type RESTGeoBox struct {
	TopLeft     RESTGeoPoint `json:"top_left"`
	BottomRight RESTGeoPoint `json:"bottom_right"`
}

type RESTGeoRadius struct {
	Center RESTGeoPoint `json:"center"`
	Radius float64      `json:"radius"`
}

type RESTGeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type RESTHasVector struct {
	Vector string `json:"vector"` // or just a plain string
}

func (h *RESTHasVector) UnmarshalJSON(data []byte) error {
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '"':
			var s string
			json.Unmarshal(data, &s)
			h.Vector = s
			return nil
		default:
			var obj struct {
				Vector string `json:"vector"`
			}
			json.Unmarshal(data, &obj)
			h.Vector = obj.Vector
			return nil
		}
	}
	return nil
}

type RESTKeyCondition struct {
	Key string `json:"key"`
}

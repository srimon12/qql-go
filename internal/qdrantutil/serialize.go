package qdrantutil

import (
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

// SerializeKeywordBoolFields builds a map from optional bool fields,
// only including keys where the pointer is non-nil.
func SerializeKeywordBoolFields(isTenant, onDisk, enableHnsw *bool) map[string]any {
	data := map[string]any{}
	if isTenant != nil {
		data["is_tenant"] = *isTenant
	}
	if onDisk != nil {
		data["on_disk"] = *onDisk
	}
	if enableHnsw != nil {
		data["enable_hnsw"] = *enableHnsw
	}
	return data
}

func SerializePayloadSchemaInfo(idxInfo *qdrant.PayloadSchemaInfo) map[string]any {
	data := map[string]any{
		"type": strings.ToLower(strings.TrimPrefix(idxInfo.GetDataType().String(), "PayloadSchemaType_")),
	}
	if params := idxInfo.GetParams(); params != nil {
		if serialized := SerializePayloadIndexParams(params); len(serialized) > 0 {
			data["params"] = serialized
		}
	}
	return data
}

func SerializePayloadIndexParams(params *qdrant.PayloadIndexParams) map[string]any {
	switch typed := params.GetIndexParams().(type) {
	case *qdrant.PayloadIndexParams_KeywordIndexParams:
		p := typed.KeywordIndexParams
		return SerializeKeywordBoolFields(p.IsTenant, p.OnDisk, p.EnableHnsw)
	case *qdrant.PayloadIndexParams_TextIndexParams:
		return SerializeTextIndexParams(typed.TextIndexParams)
	case *qdrant.PayloadIndexParams_UuidIndexParams:
		p := typed.UuidIndexParams
		return SerializeKeywordBoolFields(p.IsTenant, p.OnDisk, p.EnableHnsw)
	default:
		return nil
	}
}

func SerializeTextIndexParams(params *qdrant.TextIndexParams) map[string]any {
	if params == nil {
		return nil
	}
	data := map[string]any{}
	if params.Tokenizer != qdrant.TokenizerType_Unknown {
		data["tokenizer"] = strings.ToLower(strings.TrimPrefix(params.Tokenizer.String(), "TokenizerType_"))
	}
	if params.MinTokenLen != nil {
		data["min_token_len"] = params.GetMinTokenLen()
	}
	if params.MaxTokenLen != nil {
		data["max_token_len"] = params.GetMaxTokenLen()
	}
	if params.Lowercase != nil {
		data["lowercase"] = params.GetLowercase()
	}
	if params.AsciiFolding != nil {
		data["ascii_folding"] = params.GetAsciiFolding()
	}
	if params.PhraseMatching != nil {
		data["phrase_matching"] = params.GetPhraseMatching()
	}
	if params.OnDisk != nil {
		data["on_disk"] = params.GetOnDisk()
	}
	if params.EnableHnsw != nil {
		data["enable_hnsw"] = params.GetEnableHnsw()
	}
	return data
}

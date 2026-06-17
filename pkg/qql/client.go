package qql

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

// QdrantClient is the interface pkg/qql needs from a Qdrant connection.
// Implemented by *qdrant.Client from github.com/qdrant/go-client.
type QdrantClient interface {
	ListCollections(context.Context) ([]string, error)
	CollectionExists(context.Context, string) (bool, error)
	GetCollectionInfo(context.Context, string) (*qdrant.CollectionInfo, error)
	CreateCollection(context.Context, *qdrant.CreateCollection) error
	UpdateCollection(context.Context, *qdrant.UpdateCollection) error
	DeleteCollection(context.Context, string) error
	Upsert(context.Context, *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
	Query(context.Context, *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
	QueryBatch(context.Context, *qdrant.QueryBatchPoints) ([]*qdrant.BatchResult, error)
	QueryGroups(context.Context, *qdrant.QueryPointGroups) ([]*qdrant.PointGroup, error)
	Delete(context.Context, *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
	UpdateVectors(context.Context, *qdrant.UpdatePointVectors) (*qdrant.UpdateResult, error)
	SetPayload(context.Context, *qdrant.SetPayloadPoints) (*qdrant.UpdateResult, error)
	CreateFieldIndex(context.Context, *qdrant.CreateFieldIndexCollection) (*qdrant.UpdateResult, error)
	Count(context.Context, *qdrant.CountPoints) (uint64, error)
	ScrollAndOffset(context.Context, *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId, error)
	Get(context.Context, *qdrant.GetPoints) ([]*qdrant.RetrievedPoint, error)
}

// ClientConfig holds connection parameters for creating a Qdrant client.
type ClientConfig struct {
	// URL is the Qdrant endpoint, e.g. "http://localhost:6334" or "localhost:6334".
	URL string
	// Secret is the API key for authentication.
	Secret string
	// NoVerify disables TLS certificate verification.
	NoVerify bool
}

// NewQdrantClient creates a Qdrant client from the given config.
func NewQdrantClient(cfg ClientConfig) (QdrantClient, error) {
	qcfg, err := buildQdrantConfig(cfg.URL, cfg.Secret, cfg.NoVerify)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return qdrant.NewClient(qcfg)
}

func buildQdrantConfig(rawURL, apiKey string, noVerify bool) (*qdrant.Config, error) {
	normalized := strings.TrimSpace(rawURL)
	if normalized == "" {
		return nil, fmt.Errorf("empty URL")
	}
	if !strings.Contains(normalized, "://") {
		normalized = "http://" + normalized
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, err
	}

	host := parsed.Hostname()
	if host == "" {
		host = parsed.Host
	}
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	port := 6333
	if parsedPort := parsed.Port(); parsedPort != "" {
		port, err = strconv.Atoi(parsedPort)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q", parsedPort)
		}
	}
	if port == 6333 {
		port = 6334
	}

	var tlsConf *tls.Config
	if strings.EqualFold(parsed.Scheme, "https") {
		tlsConf = &tls.Config{MinVersion: tls.VersionTLS13}
		if noVerify {
			tlsConf.InsecureSkipVerify = true
		}
	}

	return &qdrant.Config{
		Host:                   host,
		Port:                   port,
		APIKey:                 apiKey,
		UseTLS:                 tlsConf != nil,
		TLSConfig:              tlsConf,
		SkipCompatibilityCheck: true,
	}, nil
}

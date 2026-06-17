package commands

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	"github.com/srimon12/qql-go/internal/config"
)

func newClientFromURL(rawURL, apiKey string, noVerify bool, caCert string) (*qdrant.Client, error) {
	cfg, err := buildClientConfig(rawURL, apiKey, noVerify, caCert)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	return qdrant.NewClient(cfg)
}

func buildClientConfig(rawURL, apiKey string, noVerify bool, caCert string) (*qdrant.Config, error) {
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

	port := 6334
	if parsedPort := parsed.Port(); parsedPort != "" {
		port, err = strconv.Atoi(parsedPort)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q", parsedPort)
		}
	}
	// Qdrant exposes port 6333 for the REST API and port 6334 for gRPC.
	// QQL communicates exclusively via gRPC (go-client).
	if port == 6333 {
		return nil, fmt.Errorf("port 6333 is the Qdrant REST API port; QQL uses gRPC on port 6334 — use %s:6334 or omit the port", host)
	}

	var tlsConf *tls.Config
	if strings.EqualFold(parsed.Scheme, "https") {
		tlsConf = &tls.Config{MinVersion: tls.VersionTLS13}
		if noVerify {
			tlsConf.InsecureSkipVerify = true
		}
		if caCert != "" {
			certPEM, err := os.ReadFile(caCert)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(certPEM) {
				return nil, fmt.Errorf("failed to parse CA cert from %s", caCert)
			}
			tlsConf.RootCAs = certPool
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

func loadSavedConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg == nil || cfg.URL == "" {
		return nil, fmt.Errorf("not connected. Run: qql-go connect --url <url>")
	}
	return cfg, nil
}

func loadSavedConfigAndClient() (*config.Config, *qdrant.Client, error) {
	cfg, err := loadSavedConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := newClientFromURL(cfg.URL, cfg.Secret, cfg.NoVerify, cfg.CACert)
	if err != nil {
		return nil, nil, fmt.Errorf("connection failed: %w", err)
	}
	return cfg, client, nil
}

func savedConfigMessage() string {
	path, err := config.ConfigPath()
	if err != nil {
		return "Connected. Config saved."
	}
	return fmt.Sprintf("Connected. Config saved to %s", path)
}

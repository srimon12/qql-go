package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	URL                  string            `json:"url"`
	Secret               string            `json:"secret"`
	ActiveProfile        string            `json:"active_profile"`
	InferenceModel       string            `json:"inference_model"`
	SparseInferenceModel string            `json:"sparse_inference_model"`
	InferenceMode        string            `json:"inference_mode"`
	CloudModelOptions    map[string]string `json:"cloud_model_options,omitempty"` // e.g. {"openai-api-key": "sk-...", "openrouter-api-key": "..."}
	EmbeddingEndpoint    string            `json:"embedding_endpoint"`
	EmbeddingAPIKey      string            `json:"embedding_api_key"`
	EmbeddingModel       string            `json:"embedding_model"`
	EmbeddingDimension   int               `json:"embedding_dimension"`
	NoVerify             bool              `json:"no_verify"`
	CACert               string            `json:"ca_cert"`
	RequestTimeout       int               `json:"request_timeout,omitempty"` // per-request timeout in seconds for Qdrant operations; 0 = no server-side timeout
	BM25K1               *float64          `json:"bm25_k1,omitempty"`
	BM25B                *float64          `json:"bm25_b,omitempty"`
	BM25AvgDL            *float64          `json:"bm25_avg_dl,omitempty"`
}

type Profile struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Secret   string `json:"secret"`
	NoVerify bool   `json:"no_verify"`
	CACert   string `json:"ca_cert"`
}

var (
	cfg      *Config
	profiles map[string]*Profile
	mu       sync.RWMutex
)

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	dir := filepath.Join(home, ".qql")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}
	return dir, nil
}

func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func profilesPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	var loaded Config
	if err := readJSONFile(path, &loaded); err != nil {
		if os.IsNotExist(err) {
			cfg = nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	mu.Lock()
	cfg = cloneConfig(&loaded)
	mu.Unlock()
	return cfg, nil
}

func SaveConfig(c *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	stored := cloneConfig(c)
	if stored == nil {
		stored = &Config{}
	}

	if err := writeJSONFile(path, stored); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	mu.Lock()
	cfg = stored
	mu.Unlock()
	return nil
}

func DeleteConfig() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete config: %w", err)
	}

	mu.Lock()
	cfg = nil
	mu.Unlock()
	return nil
}

func GetConfig() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}

func LoadProfiles() (map[string]*Profile, error) {
	path, err := profilesPath()
	if err != nil {
		return nil, err
	}

	loaded := map[string]*Profile{}
	if err := readJSONFile(path, &loaded); err != nil {
		if os.IsNotExist(err) {
			profiles = map[string]*Profile{}
			return profiles, nil
		}
		return nil, fmt.Errorf("failed to read profiles: %w", err)
	}

	mu.Lock()
	profiles = normalizeProfiles(loaded)
	mu.Unlock()
	return profiles, nil
}

func SaveProfiles(p map[string]*Profile) error {
	path, err := profilesPath()
	if err != nil {
		return err
	}

	stored := normalizeProfiles(p)
	if err := writeJSONFile(path, stored); err != nil {
		return fmt.Errorf("failed to write profiles: %w", err)
	}

	mu.Lock()
	profiles = stored
	mu.Unlock()
	return nil
}

func GetProfile(name string) (*Profile, error) {
	loaded, err := ensureProfiles()
	if err != nil {
		return nil, err
	}
	return loaded[name], nil
}

func SaveProfile(profile *Profile) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	loaded, err := ensureProfiles()
	if err != nil {
		return err
	}

	clone := *profile
	loaded[clone.Name] = &clone
	return SaveProfiles(loaded)
}

func DeleteProfile(name string) error {
	loaded, err := ensureProfiles()
	if err != nil {
		return err
	}
	delete(loaded, name)
	return SaveProfiles(loaded)
}

func HasConfig() bool {
	return cfg != nil && cfg.URL != ""
}

func ensureProfiles() (map[string]*Profile, error) {
	mu.RLock()
	if profiles != nil {
		defer mu.RUnlock()
		return profiles, nil
	}
	mu.RUnlock()
	return LoadProfiles()
}

func cloneConfig(c *Config) *Config {
	if c == nil {
		return nil
	}
	clone := *c
	if c.CloudModelOptions != nil {
		clone.CloudModelOptions = make(map[string]string, len(c.CloudModelOptions))
		maps.Copy(clone.CloudModelOptions, c.CloudModelOptions)
	}
	return &clone
}

func normalizeProfiles(input map[string]*Profile) map[string]*Profile {
	if len(input) == 0 {
		return map[string]*Profile{}
	}

	normalized := make(map[string]*Profile, len(input))
	for name, profile := range input {
		if profile == nil {
			profile = &Profile{}
		}
		clone := *profile
		clone.Name = name
		normalized[name] = &clone
	}
	return normalized
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

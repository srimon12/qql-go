package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func resetTestConfigState(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	cfg = nil
	profiles = nil

	t.Cleanup(func() {
		cfg = nil
		profiles = nil
	})
}

func TestConfigPathUsesHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	cfg = nil
	profiles = nil

	path, err := ConfigPath()
	require.NoError(t, err)
	require.Contains(t, path, home)
	require.Contains(t, path, ".qql")
	require.Contains(t, path, "config.json")
}

func TestLoadConfigMissingReturnsNil(t *testing.T) {
	resetTestConfigState(t)

	loaded, err := LoadConfig()
	require.NoError(t, err)
	require.Nil(t, loaded)
	require.Nil(t, GetConfig())
}

func TestSaveLoadAndDeleteConfigRoundTrip(t *testing.T) {
	resetTestConfigState(t)

	original := &Config{
		URL:                "https://qdrant.example.com",
		Secret:             "secret",
		ActiveProfile:      "prod",
		InferenceModel:     "legacy-model",
		InferenceMode:      "external",
		EmbeddingEndpoint:  "https://api.example.com/v1/embeddings",
		EmbeddingAPIKey:    "embed-secret",
		EmbeddingModel:     "text-embedding-3-small",
		EmbeddingDimension: 1536,
	}

	require.NoError(t, SaveConfig(original))

	path, err := ConfigPath()
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.NoError(t, err)

	cfg = nil

	loaded, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, "https://qdrant.example.com", loaded.URL)
	require.Equal(t, "secret", loaded.Secret)
	require.Equal(t, "prod", loaded.ActiveProfile)
	require.Equal(t, "legacy-model", loaded.InferenceModel)
	require.Equal(t, "external", loaded.InferenceMode)
	require.Equal(t, "https://api.example.com/v1/embeddings", loaded.EmbeddingEndpoint)
	require.Equal(t, "embed-secret", loaded.EmbeddingAPIKey)
	require.Equal(t, "text-embedding-3-small", loaded.EmbeddingModel)
	require.Equal(t, 1536, loaded.EmbeddingDimension)

	require.NoError(t, DeleteConfig())
	_, err = os.Stat(path)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Nil(t, GetConfig())
}

func TestLoadConfigRejectsMalformedJSON(t *testing.T) {
	resetTestConfigState(t)

	path, err := ConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("{broken"), 0o644))

	loaded, err := LoadConfig()
	require.Nil(t, loaded)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read config")
}

func TestProfileRoundTrip(t *testing.T) {
	resetTestConfigState(t)

	require.NoError(t, SaveProfile(&Profile{
		Name:   "prod",
		URL:    "https://qdrant.example.com",
		Secret: "top-secret",
	}))

	profile, err := GetProfile("prod")
	require.NoError(t, err)
	require.NotNil(t, profile)
	require.Equal(t, "prod", profile.Name)
	require.Equal(t, "https://qdrant.example.com", profile.URL)

	require.NoError(t, DeleteProfile("prod"))

	profile, err = GetProfile("prod")
	require.NoError(t, err)
	require.Nil(t, profile)
}

func TestLoadProfilesMissingReturnsEmptyMap(t *testing.T) {
	resetTestConfigState(t)

	loaded, err := LoadProfiles()
	require.NoError(t, err)
	require.Empty(t, loaded)
}

func TestSaveProfileValidatesInput(t *testing.T) {
	resetTestConfigState(t)

	require.EqualError(t, SaveProfile(nil), "profile is nil")
	require.EqualError(t, SaveProfile(&Profile{}), "profile name is required")
}

func TestConfigPathReturnsDotQQL(t *testing.T) {
	resetTestConfigState(t)

	path, err := ConfigPath()
	require.NoError(t, err)
	require.Contains(t, path, ".qql")
}

package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func resetTestConfigState(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	viper.Reset()
	cfg = &Config{}
	profiles = make(map[string]*Profile)

	t.Cleanup(func() {
		viper.Reset()
		cfg = &Config{}
		profiles = make(map[string]*Profile)
	})
}

func TestLoadConfigMissingReturnsNil(t *testing.T) {
	resetTestConfigState(t)

	loaded, err := LoadConfig()
	require.NoError(t, err)
	require.Nil(t, loaded)
}

func TestSaveLoadAndDeleteConfigRoundTrip(t *testing.T) {
	resetTestConfigState(t)

	original := &Config{
		URL:            "https://qdrant.example.com",
		Secret:         "secret",
		ActiveProfile:  "prod",
		InferenceModel: "legacy-model",
	}

	require.NoError(t, SaveConfig(original))

	path, err := configPath()
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.NoError(t, err)

	viper.Reset()
	cfg = nil

	loaded, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, "https://qdrant.example.com", loaded.URL)
	require.Equal(t, "secret", loaded.Secret)
	require.Equal(t, "prod", loaded.ActiveProfile)
	require.Equal(t, "legacy-model", loaded.InferenceModel)

	require.NoError(t, DeleteConfig())
	_, err = os.Stat(path)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Equal(t, "", GetConfig().URL)
}

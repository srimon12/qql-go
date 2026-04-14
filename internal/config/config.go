package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	URL            string `mapstructure:"url"`
	Secret         string `mapstructure:"secret"`
	ActiveProfile  string `mapstructure:"active_profile"`
	InferenceModel string `mapstructure:"inference_model"`
}

type Profile struct {
	Name   string `mapstructure:"name"`
	URL    string `mapstructure:"url"`
	Secret string `mapstructure:"secret"`
}

var cfg *Config
var profiles map[string]*Profile

func init() {
	cfg = &Config{}
	profiles = make(map[string]*Profile)
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	dir := filepath.Join(home, ".qql")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}
	return dir, nil
}

func configPath() (string, error) {
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
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("json")

	if err := viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var loaded Config
	if err := viper.Unmarshal(&loaded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg = &loaded

	return cfg, nil
}

func SaveConfig(c *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if c == nil {
		c = &Config{}
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("json")

	viper.Set("url", c.URL)
	viper.Set("secret", c.Secret)
	viper.Set("active_profile", c.ActiveProfile)
	viper.Set("inference_model", c.InferenceModel)

	if err := viper.SafeWriteConfig(); err != nil {
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	cfg = c
	return nil
}

func DeleteConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete config: %w", err)
	}

	cfg = &Config{}
	return nil
}

func GetConfig() *Config {
	return cfg
}

func LoadProfiles() (map[string]*Profile, error) {
	path, err := profilesPath()
	if err != nil {
		return nil, err
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("json")

	if err := viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return profiles, nil
		}
		return nil, fmt.Errorf("failed to read profiles: %w", err)
	}

	if err := viper.Unmarshal(&profiles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profiles: %w", err)
	}

	return profiles, nil
}

func SaveProfiles(p map[string]*Profile) error {
	path, err := profilesPath()
	if err != nil {
		return err
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("json")

	for name, profile := range p {
		profile.Name = name
		viper.Set(name, profile)
	}

	if err := viper.SafeWriteConfig(); err != nil {
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to write profiles: %w", err)
		}
	}

	profiles = p
	return nil
}

func GetProfile(name string) (*Profile, error) {
	if profiles == nil {
		var err error
		profiles, err = LoadProfiles()
		if err != nil {
			return nil, err
		}
	}
	return profiles[name], nil
}

func SaveProfile(profile *Profile) error {
	if profiles == nil {
		var err error
		profiles, err = LoadProfiles()
		if err != nil {
			return err
		}
	}
	profiles[profile.Name] = profile
	return SaveProfiles(profiles)
}

func DeleteProfile(name string) error {
	if profiles == nil {
		var err error
		profiles, err = LoadProfiles()
		if err != nil {
			return err
		}
	}
	delete(profiles, name)
	return SaveProfiles(profiles)
}

func HasConfig() bool {
	return cfg != nil && cfg.URL != ""
}

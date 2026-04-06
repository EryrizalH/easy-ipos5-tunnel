package appcore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	defaultServiceName = "EasyRatholeClient"
	taskName           = "EasyRatholeClientGUI"
)

type AppConfig struct {
	ConfigPath       string `json:"configPath"`
	AutoStartEnabled bool   `json:"autoStartEnabled"`
}

type ConfigStore struct {
	path string
}

func NewConfigStore() (*ConfigStore, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(cfgDir, "easy-rathole-client-gui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	return &ConfigStore{
		path: filepath.Join(dir, "config.json"),
	}, nil
}

func defaultConfig() AppConfig {
	exePath, err := os.Executable()
	if err != nil {
		return AppConfig{
			ConfigPath:       "client.toml",
			AutoStartEnabled: false,
		}
	}

	return AppConfig{
		ConfigPath:       filepath.Join(filepath.Dir(exePath), "client.toml"),
		AutoStartEnabled: false,
	}
}

func (c *ConfigStore) Load() (AppConfig, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), nil
	}

	if cfg.ConfigPath == "" {
		cfg.ConfigPath = defaultConfig().ConfigPath
	}

	return cfg, nil
}

func (c *ConfigStore) Save(cfg AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o644)
}

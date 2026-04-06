package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	configDirName       = "notion-cli"
	configFileName      = "config.json"
	defaultAPIBaseURL   = "https://api.notion.com/v1"
	defaultNotionAPIVer = "2026-03-11"
)

type Config struct {
	API APIConfig `json:"api,omitempty"`
}

type APIConfig struct {
	BaseURL       string `json:"base_url,omitempty"`
	NotionVersion string `json:"notion_version,omitempty"`
	Token         string `json:"token,omitempty"`
}

type LoadedConfig struct {
	Config         Config
	Path           string
	APITokenSource string
	HasConfigToken bool
}

type APIOverrides struct {
	BaseURL       string
	NotionVersion string
	Token         string
}

const (
	APITokenSourceNone   = "none"
	APITokenSourceConfig = "config"
	APITokenSourceEnv    = "env"
)

func Default() Config {
	return Config{
		API: APIConfig{
			BaseURL:       defaultAPIBaseURL,
			NotionVersion: defaultNotionAPIVer,
		},
	}
}

func Path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, configDirName, configFileName), nil
}

func Load() (Config, error) {
	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		return Config{}, err
	}
	return loaded.Config, nil
}

func LoadWithMeta(overrides APIOverrides) (LoadedConfig, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return LoadedConfig{}, err
	}

	fileCfg, err := loadFile(path)
	if err != nil {
		return LoadedConfig{}, err
	}
	cfg = merge(cfg, fileCfg)
	source := APITokenSourceNone
	if strings.TrimSpace(fileCfg.API.Token) != "" {
		source = APITokenSourceConfig
	}

	applyOverrides(&cfg, overrides)
	if strings.TrimSpace(overrides.Token) != "" {
		source = APITokenSourceEnv
	}

	normalize(&cfg)
	return LoadedConfig{
		Config:         cfg,
		Path:           path,
		APITokenSource: source,
		HasConfigToken: strings.TrimSpace(fileCfg.API.Token) != "",
	}, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	normalize(&cfg)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("secure config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, configFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}

	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temp config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("replace config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure config file: %w", err)
	}
	return nil
}

func SetAPIToken(token string) error {
	cfg, err := loadForMutation()
	if err != nil {
		return err
	}
	cfg.API.Token = strings.TrimSpace(token)
	return Save(cfg)
}

func UnsetAPIToken() error {
	cfg, err := loadForMutation()
	if err != nil {
		return err
	}
	cfg.API.Token = ""
	return Save(cfg)
}

func loadForMutation() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	fileCfg, err := loadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg = merge(cfg, fileCfg)
	normalize(&cfg)
	return cfg, nil
}

func loadFile(path string) (Config, error) {
	cfg := Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func merge(base, overlay Config) Config {
	if strings.TrimSpace(overlay.API.BaseURL) != "" {
		base.API.BaseURL = overlay.API.BaseURL
	}
	if strings.TrimSpace(overlay.API.NotionVersion) != "" {
		base.API.NotionVersion = overlay.API.NotionVersion
	}
	if strings.TrimSpace(overlay.API.Token) != "" {
		base.API.Token = overlay.API.Token
	}
	return base
}

func applyOverrides(cfg *Config, overrides APIOverrides) {
	if cfg == nil {
		return
	}

	if s := strings.TrimSpace(overrides.BaseURL); s != "" {
		cfg.API.BaseURL = s
	}
	if s := strings.TrimSpace(overrides.NotionVersion); s != "" {
		cfg.API.NotionVersion = s
	}
	if s := strings.TrimSpace(overrides.Token); s != "" {
		cfg.API.Token = s
	}
}

func normalize(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.API.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.API.BaseURL), "/")
	if cfg.API.BaseURL == "" {
		cfg.API.BaseURL = defaultAPIBaseURL
	}
	cfg.API.NotionVersion = strings.TrimSpace(cfg.API.NotionVersion)
	if cfg.API.NotionVersion == "" {
		cfg.API.NotionVersion = defaultNotionAPIVer
	}
	cfg.API.Token = strings.TrimSpace(cfg.API.Token)
}

package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	configDirName  = ".config/notion-cli"
	configFileName = "config.json"
)

type Config struct {
	ActiveAccount string    `json:"active_account,omitempty"`
	API           APIConfig `json:"api,omitempty"`
}

type APIConfig struct {
	BaseURL       string `json:"base_url,omitempty"`
	NotionVersion string `json:"notion_version,omitempty"`
	Token         string `json:"token,omitempty"`
}

func Default() Config {
	return Config{
		API: APIConfig{
			BaseURL:       "https://api.notion.com/v1",
			NotionVersion: "2022-06-28",
		},
	}
}

func Load() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}

	applyEnvOverrides(&cfg)
	normalize(&cfg)
	return cfg, nil
}

func LoadFile() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}

	normalize(&cfg)
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	normalize(&cfg)

	merged := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil {
		if len(existing) > 0 {
			if err := json.Unmarshal(existing, &merged); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if cfg.ActiveAccount != "" {
		merged["active_account"] = cfg.ActiveAccount
	}

	apiMap := map[string]any{}
	if existingAPI, ok := merged["api"].(map[string]any); ok {
		for k, v := range existingAPI {
			apiMap[k] = v
		}
	}
	apiMap["base_url"] = cfg.API.BaseURL
	apiMap["notion_version"] = cfg.API.NotionVersion
	if cfg.API.Token == "" {
		delete(apiMap, "token")
	} else {
		apiMap["token"] = cfg.API.Token
	}
	merged["api"] = apiMap

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName, configFileName), nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if s := os.Getenv("NOTION_API_BASE_URL"); s != "" {
		cfg.API.BaseURL = s
	}
	if s := os.Getenv("NOTION_API_NOTION_VERSION"); s != "" {
		cfg.API.NotionVersion = s
	}
	if s := os.Getenv("NOTION_API_TOKEN"); s != "" {
		cfg.API.Token = s
	}
}

func normalize(cfg *Config) {
	if cfg == nil {
		return
	}

	cfg.API.BaseURL = strings.TrimSpace(cfg.API.BaseURL)
	if cfg.API.BaseURL == "" {
		cfg.API.BaseURL = "https://api.notion.com/v1"
	}
	cfg.API.BaseURL = strings.TrimRight(cfg.API.BaseURL, "/")
	cfg.API.NotionVersion = strings.TrimSpace(cfg.API.NotionVersion)
	if cfg.API.NotionVersion == "" {
		cfg.API.NotionVersion = "2022-06-28"
	}
	cfg.API.Token = strings.TrimSpace(cfg.API.Token)
}

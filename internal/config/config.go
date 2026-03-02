package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	configDirName  = ".config/notion-cli"
	configFileName = "config.json"
)

type Config struct {
	ActiveAccount string           `json:"active_account,omitempty"`
	API           APIConfig        `json:"api,omitempty"`
	PrivateAPI    PrivateAPIConfig `json:"private_api,omitempty"`
}

type APIConfig struct {
	BaseURL       string `json:"base_url,omitempty"`
	NotionVersion string `json:"notion_version,omitempty"`
	Token         string `json:"token,omitempty"`
}

type PrivateAPIConfig struct {
	Enabled      bool   `json:"enabled"`
	BaseURL      string `json:"base_url,omitempty"`
	TokenV2      string `json:"token_v2,omitempty"`
	NotionUserID string `json:"notion_user_id,omitempty"`
	NotionUsers  string `json:"notion_users,omitempty"`
	DeviceID     string `json:"device_id,omitempty"`
	CSRF         string `json:"csrf,omitempty"`
	ActiveUserID string `json:"active_user_id,omitempty"`
	SpaceID      string `json:"space_id,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
}

func Default() Config {
	return Config{
		API: APIConfig{
			BaseURL:       "https://api.notion.com/v1",
			NotionVersion: "2022-06-28",
		},
		PrivateAPI: PrivateAPIConfig{
			BaseURL: "https://www.notion.so",
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

	privateMap := map[string]any{}
	if existingPrivate, ok := merged["private_api"].(map[string]any); ok {
		for k, v := range existingPrivate {
			privateMap[k] = v
		}
	}
	privateMap["enabled"] = cfg.PrivateAPI.Enabled
	privateMap["base_url"] = cfg.PrivateAPI.BaseURL
	setOrDelete(privateMap, "token_v2", cfg.PrivateAPI.TokenV2)
	setOrDelete(privateMap, "notion_user_id", cfg.PrivateAPI.NotionUserID)
	setOrDelete(privateMap, "notion_users", cfg.PrivateAPI.NotionUsers)
	setOrDelete(privateMap, "device_id", cfg.PrivateAPI.DeviceID)
	setOrDelete(privateMap, "csrf", cfg.PrivateAPI.CSRF)
	setOrDelete(privateMap, "active_user_id", cfg.PrivateAPI.ActiveUserID)
	setOrDelete(privateMap, "space_id", cfg.PrivateAPI.SpaceID)
	setOrDelete(privateMap, "user_agent", cfg.PrivateAPI.UserAgent)
	merged["private_api"] = privateMap

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func setOrDelete(target map[string]any, key, value string) {
	if strings.TrimSpace(value) == "" {
		delete(target, key)
		return
	}
	target[key] = value
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

	if s := os.Getenv("NOTION_PRIVATE_API_ENABLED"); s != "" {
		if v, err := strconv.ParseBool(s); err == nil {
			cfg.PrivateAPI.Enabled = v
		}
	}
	if s := os.Getenv("NOTION_PRIVATE_API_BASE_URL"); s != "" {
		cfg.PrivateAPI.BaseURL = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_TOKEN_V2"); s != "" {
		cfg.PrivateAPI.TokenV2 = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_NOTION_USER_ID"); s != "" {
		cfg.PrivateAPI.NotionUserID = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_NOTION_USERS"); s != "" {
		cfg.PrivateAPI.NotionUsers = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_DEVICE_ID"); s != "" {
		cfg.PrivateAPI.DeviceID = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_CSRF"); s != "" {
		cfg.PrivateAPI.CSRF = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_ACTIVE_USER_ID"); s != "" {
		cfg.PrivateAPI.ActiveUserID = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_SPACE_ID"); s != "" {
		cfg.PrivateAPI.SpaceID = s
	}
	if s := os.Getenv("NOTION_PRIVATE_API_USER_AGENT"); s != "" {
		cfg.PrivateAPI.UserAgent = s
	}
}

func normalize(cfg *Config) {
	if cfg == nil {
		return
	}

	cfg.ActiveAccount = strings.TrimSpace(cfg.ActiveAccount)

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

	cfg.PrivateAPI.BaseURL = strings.TrimSpace(cfg.PrivateAPI.BaseURL)
	if cfg.PrivateAPI.BaseURL == "" {
		cfg.PrivateAPI.BaseURL = "https://www.notion.so"
	}
	cfg.PrivateAPI.BaseURL = strings.TrimRight(cfg.PrivateAPI.BaseURL, "/")
	cfg.PrivateAPI.TokenV2 = strings.TrimSpace(cfg.PrivateAPI.TokenV2)
	cfg.PrivateAPI.NotionUserID = strings.TrimSpace(cfg.PrivateAPI.NotionUserID)
	cfg.PrivateAPI.NotionUsers = strings.TrimSpace(cfg.PrivateAPI.NotionUsers)
	cfg.PrivateAPI.DeviceID = strings.TrimSpace(cfg.PrivateAPI.DeviceID)
	cfg.PrivateAPI.CSRF = strings.TrimSpace(cfg.PrivateAPI.CSRF)
	cfg.PrivateAPI.ActiveUserID = strings.TrimSpace(cfg.PrivateAPI.ActiveUserID)
	cfg.PrivateAPI.SpaceID = strings.TrimSpace(cfg.PrivateAPI.SpaceID)
	cfg.PrivateAPI.UserAgent = strings.TrimSpace(cfg.PrivateAPI.UserAgent)
}

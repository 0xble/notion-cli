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
	PrivateAPI PrivateAPIConfig `json:"private_api"`
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
	cfg.PrivateAPI.BaseURL = strings.TrimSpace(cfg.PrivateAPI.BaseURL)
	if cfg.PrivateAPI.BaseURL == "" {
		cfg.PrivateAPI.BaseURL = "https://www.notion.so"
	}
	cfg.PrivateAPI.BaseURL = strings.TrimRight(cfg.PrivateAPI.BaseURL, "/")
}

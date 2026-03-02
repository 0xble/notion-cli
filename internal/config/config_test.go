package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyEnvOverridesAPI(t *testing.T) {
	t.Setenv("NOTION_API_BASE_URL", "https://api.example.com/v1/")
	t.Setenv("NOTION_API_NOTION_VERSION", "2022-06-28")
	t.Setenv("NOTION_API_TOKEN", "api-token")

	cfg := Default()
	applyEnvOverrides(&cfg)
	normalize(&cfg)

	if cfg.API.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected api.base_url normalization: %q", cfg.API.BaseURL)
	}
	if cfg.API.NotionVersion != "2022-06-28" {
		t.Fatalf("unexpected api.notion_version: %q", cfg.API.NotionVersion)
	}
	if cfg.API.Token != "api-token" {
		t.Fatalf("unexpected api.token: %q", cfg.API.Token)
	}
}

func TestApplyEnvOverridesPrivateAPI(t *testing.T) {
	t.Setenv("NOTION_PRIVATE_API_ENABLED", "true")
	t.Setenv("NOTION_PRIVATE_API_TOKEN_V2", "tok")
	t.Setenv("NOTION_PRIVATE_API_NOTION_USER_ID", "user")
	t.Setenv("NOTION_PRIVATE_API_BASE_URL", "https://example.com/")

	cfg := Default()
	applyEnvOverrides(&cfg)
	normalize(&cfg)

	if !cfg.PrivateAPI.Enabled {
		t.Fatal("expected private_api.enabled=true from env")
	}
	if cfg.PrivateAPI.TokenV2 != "tok" {
		t.Fatalf("unexpected token_v2: %q", cfg.PrivateAPI.TokenV2)
	}
	if cfg.PrivateAPI.NotionUserID != "user" {
		t.Fatalf("unexpected notion_user_id: %q", cfg.PrivateAPI.NotionUserID)
	}
	if cfg.PrivateAPI.BaseURL != "https://example.com" {
		t.Fatalf("unexpected base_url normalization: %q", cfg.PrivateAPI.BaseURL)
	}
}

func TestNormalizeAppliesAPIDefaults(t *testing.T) {
	cfg := Config{}
	normalize(&cfg)

	if cfg.API.BaseURL != "https://api.notion.com/v1" {
		t.Fatalf("unexpected api.base_url default: %q", cfg.API.BaseURL)
	}
	if cfg.API.NotionVersion != "2022-06-28" {
		t.Fatalf("unexpected api.notion_version default: %q", cfg.API.NotionVersion)
	}
	if cfg.PrivateAPI.BaseURL != "https://www.notion.so" {
		t.Fatalf("unexpected private_api.base_url default: %q", cfg.PrivateAPI.BaseURL)
	}
}

func TestPathUsesHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/example-home")

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/example-home/.config/notion-cli/config.json" {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestLoadFileIgnoresEnvOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("NOTION_API_TOKEN", "env-token")

	cfg := Default()
	cfg.API.Token = "file-token"
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFile()
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	if loaded.API.Token != "file-token" {
		t.Fatalf("unexpected file token: %q", loaded.API.Token)
	}

	withEnv, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if withEnv.API.Token != "env-token" {
		t.Fatalf("unexpected env override token: %q", withEnv.API.Token)
	}
}

func TestSavePreservesUnknownFieldsAndCanUnsetToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := Path()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	initial := map[string]any{
		"active_account": "default",
		"api": map[string]any{
			"base_url":       "https://api.notion.com/v1",
			"notion_version": "2022-06-28",
			"token":          "old-token",
			"require_icon":   true,
		},
		"custom": map[string]any{
			"keep": "value",
		},
	}
	raw, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		t.Fatalf("marshal initial: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	cfg, err := LoadFile()
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	cfg.API.Token = ""
	cfg.API.BaseURL = "https://api.example.com/v1/"
	cfg.PrivateAPI.Enabled = true
	cfg.PrivateAPI.TokenV2 = "private-token"
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	updatedBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated: %v", err)
	}
	updated := map[string]any{}
	if err := json.Unmarshal(updatedBytes, &updated); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}

	custom, ok := updated["custom"].(map[string]any)
	if !ok || custom["keep"] != "value" {
		t.Fatalf("custom field not preserved: %#v", updated["custom"])
	}

	apiMap, ok := updated["api"].(map[string]any)
	if !ok {
		t.Fatalf("api field missing")
	}
	if _, ok := apiMap["token"]; ok {
		t.Fatalf("expected token key removed, got %#v", apiMap["token"])
	}
	if apiMap["require_icon"] != true {
		t.Fatalf("expected require_icon preserved, got %#v", apiMap["require_icon"])
	}
	if apiMap["base_url"] != "https://api.example.com/v1" {
		t.Fatalf("base_url not normalized: %#v", apiMap["base_url"])
	}

	privateMap, ok := updated["private_api"].(map[string]any)
	if !ok {
		t.Fatalf("private_api field missing: %#v", updated["private_api"])
	}
	if enabled, ok := privateMap["enabled"].(bool); !ok || !enabled {
		t.Fatalf("private_api.enabled mismatch: %#v", privateMap["enabled"])
	}
	if privateMap["token_v2"] != "private-token" {
		t.Fatalf("private_api.token_v2 mismatch: %#v", privateMap["token_v2"])
	}
}

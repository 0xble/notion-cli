package config

import "testing"

func TestApplyEnvOverrides(t *testing.T) {
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

func TestNormalizeAppliesAPIDefaults(t *testing.T) {
	cfg := Config{}
	normalize(&cfg)

	if cfg.API.BaseURL != "https://api.notion.com/v1" {
		t.Fatalf("unexpected api.base_url default: %q", cfg.API.BaseURL)
	}
	if cfg.API.NotionVersion != "2022-06-28" {
		t.Fatalf("unexpected api.notion_version default: %q", cfg.API.NotionVersion)
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

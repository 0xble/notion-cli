package config

import "testing"

func TestApplyEnvOverrides(t *testing.T) {
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

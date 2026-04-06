package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithMetaDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.BaseURL != defaultAPIBaseURL {
		t.Fatalf("BaseURL = %q, want %q", loaded.Config.API.BaseURL, defaultAPIBaseURL)
	}
	if loaded.Config.API.NotionVersion != defaultNotionAPIVer {
		t.Fatalf("NotionVersion = %q, want %q", loaded.Config.API.NotionVersion, defaultNotionAPIVer)
	}
	if loaded.APITokenSource != APITokenSourceNone {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceNone)
	}
}

func TestLoadWithMetaReportsConfigTokenSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetAPIToken("secret-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "secret-token" {
		t.Fatalf("Token = %q, want secret-token", loaded.Config.API.Token)
	}
	if loaded.APITokenSource != APITokenSourceConfig {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceConfig)
	}
}

func TestLoadWithMetaEnvOverrideWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetAPIToken("config-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}
	loaded, err := LoadWithMeta(APIOverrides{
		Token:         "env-token",
		BaseURL:       "https://example.test/v1/",
		NotionVersion: "2026-04-01",
	})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "env-token" {
		t.Fatalf("Token = %q, want env-token", loaded.Config.API.Token)
	}
	if loaded.Config.API.BaseURL != "https://example.test/v1" {
		t.Fatalf("BaseURL = %q", loaded.Config.API.BaseURL)
	}
	if loaded.Config.API.NotionVersion != "2026-04-01" {
		t.Fatalf("NotionVersion = %q", loaded.Config.API.NotionVersion)
	}
	if loaded.APITokenSource != APITokenSourceEnv {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceEnv)
	}
}

func TestUnsetAPITokenClearsStoredToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetAPIToken("secret-token"); err != nil {
		t.Fatalf("SetAPIToken: %v", err)
	}
	if err := UnsetAPIToken(); err != nil {
		t.Fatalf("UnsetAPIToken: %v", err)
	}

	loaded, err := LoadWithMeta(APIOverrides{})
	if err != nil {
		t.Fatalf("LoadWithMeta: %v", err)
	}
	if loaded.Config.API.Token != "" {
		t.Fatalf("Token = %q, want empty", loaded.Config.API.Token)
	}
	if loaded.APITokenSource != APITokenSourceNone {
		t.Fatalf("APITokenSource = %q, want %q", loaded.APITokenSource, APITokenSourceNone)
	}
}

func TestSaveSecuresConfigFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := Default()
	cfg.API.Token = "secret-token"
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config perm = %o, want 600", perm)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir perm = %o, want 700", perm)
	}
}

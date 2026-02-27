package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestResolveAccountName_DefaultAndConfiguredActive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	account, err := ResolveAccountName("")
	if err != nil {
		t.Fatalf("ResolveAccountName() error = %v", err)
	}
	if account != defaultAccount {
		t.Fatalf("ResolveAccountName() = %q, want %q", account, defaultAccount)
	}

	if err := SetActiveAccount("work"); err != nil {
		t.Fatalf("SetActiveAccount() error = %v", err)
	}

	account, err = ResolveAccountName("")
	if err != nil {
		t.Fatalf("ResolveAccountName() error = %v", err)
	}
	if account != "work" {
		t.Fatalf("ResolveAccountName() = %q, want %q", account, "work")
	}

	explicit, err := ResolveAccountName("personal")
	if err != nil {
		t.Fatalf("ResolveAccountName(explicit) error = %v", err)
	}
	if explicit != "personal" {
		t.Fatalf("ResolveAccountName(explicit) = %q, want %q", explicit, "personal")
	}
}

func TestValidateAccountName(t *testing.T) {
	if err := ValidateAccountName("work.prod_1"); err != nil {
		t.Fatalf("ValidateAccountName(valid) error = %v", err)
	}
	if err := ValidateAccountName("brian@brianle.xyz"); err != nil {
		t.Fatalf("ValidateAccountName(email) error = %v", err)
	}
	if err := ValidateAccountName("admin+ops@meridianoperations.co"); err != nil {
		t.Fatalf("ValidateAccountName(email+plus) error = %v", err)
	}

	for _, invalid := range []string{"", " bad", "bad/name", "bad*name", ".startdot"} {
		if err := ValidateAccountName(invalid); err == nil {
			t.Fatalf("ValidateAccountName(%q) expected error, got nil", invalid)
		}
	}
}

func TestLegacyMigrationAndGetToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	expires := time.Now().Add(2 * time.Hour).UTC().Round(0)
	legacyPath := filepath.Join(home, configDir, legacyTokenFile)
	writeTokenFile(t, legacyPath, map[string]any{
		"access_token":  "legacy-access",
		"token_type":    "Bearer",
		"refresh_token": "legacy-refresh",
		"expires_at":    expires,
		"client_id":     "legacy-client",
	})

	store, err := NewFileTokenStoreForAccount("")
	if err != nil {
		t.Fatalf("NewFileTokenStoreForAccount() error = %v", err)
	}

	wantPath := filepath.Join(home, configDir, accountsDir, "default.json")
	if store.Path() != wantPath {
		t.Fatalf("store.Path() = %q, want %q", store.Path(), wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected migrated file at %q: %v", wantPath, err)
	}

	token, err := store.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if token.AccessToken != "legacy-access" {
		t.Fatalf("GetToken().AccessToken = %q, want %q", token.AccessToken, "legacy-access")
	}
	if !token.ExpiresAt.Equal(expires) {
		t.Fatalf("GetToken().ExpiresAt = %v, want %v", token.ExpiresAt, expires)
	}

	clientID, err := store.GetClientID(context.Background())
	if err != nil {
		t.Fatalf("GetClientID() error = %v", err)
	}
	if clientID != "legacy-client" {
		t.Fatalf("GetClientID() = %q, want %q", clientID, "legacy-client")
	}
}

func TestListAccountsIncludesConfiguredAndLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := SetActiveAccount("team"); err != nil {
		t.Fatalf("SetActiveAccount() error = %v", err)
	}

	writeTokenFile(t, filepath.Join(home, configDir, accountsDir, "work.json"), map[string]any{"access_token": "a", "token_type": "Bearer"})
	writeTokenFile(t, filepath.Join(home, configDir, accountsDir, "personal.json"), map[string]any{"access_token": "b", "token_type": "Bearer"})
	writeTokenFile(t, filepath.Join(home, configDir, legacyTokenFile), map[string]any{"access_token": "legacy", "token_type": "Bearer"})

	accounts, err := ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}

	want := []string{"default", "personal", "team", "work"}
	if !reflect.DeepEqual(accounts, want) {
		t.Fatalf("ListAccounts() = %#v, want %#v", accounts, want)
	}
}

func TestClearDefaultAccountRemovesLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeTokenFile(t, filepath.Join(home, configDir, legacyTokenFile), map[string]any{"access_token": "legacy", "token_type": "Bearer"})

	store, err := NewFileTokenStoreForAccount("default")
	if err != nil {
		t.Fatalf("NewFileTokenStoreForAccount(default) error = %v", err)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, configDir, legacyTokenFile)); !os.IsNotExist(err) {
		t.Fatalf("legacy token should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("account token should be removed, stat err = %v", err)
	}
}

func TestClearAllTokensRemovesAccountsAndLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeTokenFile(t, filepath.Join(home, configDir, accountsDir, "work.json"), map[string]any{"access_token": "a", "token_type": "Bearer"})
	writeTokenFile(t, filepath.Join(home, configDir, legacyTokenFile), map[string]any{"access_token": "legacy", "token_type": "Bearer"})

	if err := ClearAllTokens(); err != nil {
		t.Fatalf("ClearAllTokens() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, configDir, accountsDir)); !os.IsNotExist(err) {
		t.Fatalf("accounts directory should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, configDir, legacyTokenFile)); !os.IsNotExist(err) {
		t.Fatalf("legacy token should be removed, stat err = %v", err)
	}
}

func TestSetActiveAccountPreservesUnknownConfigFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, configDir, configFile)
	writeTokenFile(t, configPath, map[string]any{
		"active_account": "default",
		"api": map[string]any{
			"base_url":     "https://api.notion.com/v1",
			"require_icon": true,
		},
	})

	if err := SetActiveAccount("work"); err != nil {
		t.Fatalf("SetActiveAccount() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(configPath) error = %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal config error = %v", err)
	}

	if got := cfg["active_account"]; got != "work" {
		t.Fatalf("active_account = %v, want %q", got, "work")
	}

	api, ok := cfg["api"].(map[string]any)
	if !ok {
		t.Fatalf("expected api object to be preserved, got %T", cfg["api"])
	}
	if got := api["base_url"]; got != "https://api.notion.com/v1" {
		t.Fatalf("api.base_url = %v, want %q", got, "https://api.notion.com/v1")
	}
	if got := api["require_icon"]; got != true {
		t.Fatalf("api.require_icon = %v, want true", got)
	}
}

func writeTokenFile(t *testing.T, path string, payload map[string]any) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal payload error = %v", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

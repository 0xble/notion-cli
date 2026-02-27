package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/config"
)

func TestRunAuthAPISetupVerifiesAndSavesToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotVersion string
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Notion-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"user","id":"user-id"}`))
	}))
	defer srv.Close()

	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_NOTION_VERSION", "2022-06-28")

	if err := runAuthAPISetup(authAPISetupOptions{Token: "secret-token"}); err != nil {
		t.Fatalf("run auth api setup: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("verify call count mismatch: got %d", callCount)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method mismatch: got %s", gotMethod)
	}
	if gotPath != "/v1/users/me" {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth mismatch: got %q", gotAuth)
	}
	if gotVersion != "2022-06-28" {
		t.Fatalf("version mismatch: got %q", gotVersion)
	}

	cfg, err := config.LoadFile()
	if err != nil {
		t.Fatalf("load file config: %v", err)
	}
	if cfg.API.Token != "secret-token" {
		t.Fatalf("saved token mismatch: got %q", cfg.API.Token)
	}
	if cfg.API.BaseURL != srv.URL+"/v1" {
		t.Fatalf("saved base URL mismatch: got %q", cfg.API.BaseURL)
	}
}

func TestRunAuthAPISetupNoVerifySkipsNetwork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Point to a dead endpoint; should still pass because --no-verify is set.
	t.Setenv("NOTION_API_BASE_URL", "http://127.0.0.1:65535/v1")

	if err := runAuthAPISetup(authAPISetupOptions{
		Token:    "secret-token",
		NoVerify: true,
	}); err != nil {
		t.Fatalf("run auth api setup: %v", err)
	}
}

func TestAuthAPIUnsetRemovesSavedTokenAndPreservesUnknownFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	initial := map[string]any{
		"active_account": "default",
		"api": map[string]any{
			"base_url":       "https://api.notion.com/v1",
			"notion_version": "2022-06-28",
			"token":          "secret-token",
			"require_icon":   true,
		},
	}
	raw, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		t.Fatalf("marshal initial: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	cmd := AuthAPIUnsetCmd{}
	if err := cmd.Run(&Context{}); err != nil {
		t.Fatalf("unset run: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var updated map[string]any
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	apiMap, ok := updated["api"].(map[string]any)
	if !ok {
		t.Fatalf("api map missing")
	}
	if _, ok := apiMap["token"]; ok {
		t.Fatalf("token key should be removed")
	}
	if apiMap["require_icon"] != true {
		t.Fatalf("require_icon not preserved: %#v", apiMap["require_icon"])
	}
}

func TestAuthAPIVerifyRequiresConfiguredToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := AuthAPIVerifyCmd{}
	err := cmd.Run(&Context{})
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAuthAPISetupNonInteractiveErrorMentionsAPITokenFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := runAuthAPISetup(authAPISetupOptions{})
	if err == nil {
		t.Fatal("expected non-interactive token error")
	}
	if !strings.Contains(err.Error(), "--api-token") {
		t.Fatalf("error should mention --api-token: %v", err)
	}
}

func TestAuthAPISetupWizardIntroMentionsInternalIntegrations(t *testing.T) {
	model := newAuthAPISetupWizardModel()
	view := model.View()

	if !strings.Contains(view, "Internal integration token") {
		t.Fatalf("intro should mention internal integration token: %q", view)
	}
	if !strings.Contains(view, "not Public OAuth app credentials") {
		t.Fatalf("intro should clarify public apps are not for this flow: %q", view)
	}
	if !strings.Contains(view, apiSetupInternalIntegrationsURL) {
		t.Fatalf("intro should include internal integrations URL: %q", view)
	}
}

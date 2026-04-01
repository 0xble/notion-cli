package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOfficialAPITokenFromReader(t *testing.T) {
	token, err := readOfficialAPIToken(strings.NewReader(" secret-token \n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("readOfficialAPIToken: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("token = %q, want secret-token", token)
	}
}

func TestAuthAPIStatusJSONUsesLoadedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_TOKEN", "env-token")

	var out bytes.Buffer
	oldOut := authAPIOutput
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIOutput = oldOut
	})

	cmd := &AuthAPIStatusCmd{JSON: true}
	if err := cmd.Run(&Context{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), `"configured": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"token_source": "env"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestAuthAPIVerifyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/users/me" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"object":"user","id":"user_123","type":"bot","name":"Notion CLI","bot":{"workspace_name":"Workspace"}}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_TOKEN", "env-token")
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")

	var out bytes.Buffer
	oldOut := authAPIOutput
	authAPIOutput = &out
	t.Cleanup(func() {
		authAPIOutput = oldOut
	})

	cmd := &AuthAPIVerifyCmd{JSON: true}
	if err := cmd.Run(&Context{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), `"verified": true`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"workspace_name": "Workspace"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

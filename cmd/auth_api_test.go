package cmd

import (
	"bytes"
	"errors"
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

func TestPrintOfficialAPITokenSetupHintIncludesURL(t *testing.T) {
	var out bytes.Buffer

	printOfficialAPITokenSetupHint(&out, false)

	text := out.String()
	if !strings.Contains(text, officialAPIIntegrationsURL) {
		t.Fatalf("hint should include integrations URL: %q", text)
	}
	if !strings.Contains(text, "copy the token from Configuration") {
		t.Fatalf("hint should explain where to find the token: %q", text)
	}
	if !strings.Contains(text, "Paste is hidden. Press Enter when done.") {
		t.Fatalf("hint should explain hidden paste behavior: %q", text)
	}
}

func TestPrintOfficialAPITokenSetupHintOpensBrowserWhenRequested(t *testing.T) {
	var out bytes.Buffer
	var openedURL string

	oldOpenBrowser := openOfficialAPIBrowser
	openOfficialAPIBrowser = func(url string) error {
		openedURL = url
		return nil
	}
	t.Cleanup(func() {
		openOfficialAPIBrowser = oldOpenBrowser
	})

	printOfficialAPITokenSetupHint(&out, true)

	if openedURL != officialAPIIntegrationsURL {
		t.Fatalf("opened URL = %q, want %q", openedURL, officialAPIIntegrationsURL)
	}
	if !strings.Contains(out.String(), "Opening that page in your browser") {
		t.Fatalf("expected browser open message in output: %q", out.String())
	}
}

func TestPrintOfficialAPITokenSetupHintReportsBrowserOpenFailure(t *testing.T) {
	var out bytes.Buffer

	oldOpenBrowser := openOfficialAPIBrowser
	openOfficialAPIBrowser = func(string) error {
		return errors.New("boom")
	}
	t.Cleanup(func() {
		openOfficialAPIBrowser = oldOpenBrowser
	})

	printOfficialAPITokenSetupHint(&out, true)

	if !strings.Contains(out.String(), "Could not open browser automatically: boom") {
		t.Fatalf("expected browser open failure in output: %q", out.String())
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

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunPageArchiveUsesOfficialAPI(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pageID + `","object":"page","archived":true}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	if err := runPageArchive(&Context{}, pageID); err != nil {
		t.Fatalf("runPageArchive: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Fatalf("method mismatch: got %s", gotMethod)
	}
	if gotPath != "/v1/pages/"+pageID {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("auth mismatch: got %q", gotAuth)
	}
	if gotBody["archived"] != true {
		t.Fatalf("archived payload mismatch: %#v", gotBody["archived"])
	}
}

func TestRunPageArchiveSupportsURLInputWithEmbeddedID(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	pageURL := "https://www.notion.so/My-Page-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pageID + `","object":"page","archived":true}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	if err := runPageArchive(&Context{}, pageURL); err != nil {
		t.Fatalf("runPageArchive: %v", err)
	}
	if gotPath != "/v1/pages/"+pageID {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
}

func TestRunPageArchiveRequiresOfficialAPIToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", "http://127.0.0.1:65535/v1")

	err := runPageArchive(&Context{}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	if err == nil {
		t.Fatal("expected missing official API token error")
	}
	if !strings.Contains(err.Error(), "official API token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

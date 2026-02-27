package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunPageEditIconOnlyUsesOfficialAPIEmoji(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotVersion string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Notion-Version")

		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pageID + `","object":"page"}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_NOTION_VERSION", "2022-06-28")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := runPageEdit(&Context{}, pageID, "", "", "", "", "✅")
	if err != nil {
		t.Fatalf("runPageEdit: %v", err)
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
	if gotVersion != "2022-06-28" {
		t.Fatalf("notion version mismatch: got %q", gotVersion)
	}
	icon, ok := gotBody["icon"].(map[string]any)
	if !ok {
		t.Fatalf("icon payload missing or wrong type: %#v", gotBody["icon"])
	}
	if icon["type"] != "emoji" || icon["emoji"] != "✅" {
		t.Fatalf("emoji payload mismatch: %#v", icon)
	}
}

func TestRunPageEditIconOnlyUsesOfficialAPIExternalURL(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pageID + `","object":"page"}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	iconURL := "https://cdn.example.com/icon.png"
	err := runPageEdit(&Context{}, pageID, "", "", "", "", iconURL)
	if err != nil {
		t.Fatalf("runPageEdit: %v", err)
	}

	icon, ok := gotBody["icon"].(map[string]any)
	if !ok {
		t.Fatalf("icon payload missing or wrong type: %#v", gotBody["icon"])
	}
	if icon["type"] != "external" {
		t.Fatalf("icon type mismatch: %#v", icon)
	}
	external, ok := icon["external"].(map[string]any)
	if !ok {
		t.Fatalf("external payload missing or wrong type: %#v", icon["external"])
	}
	if external["url"] != iconURL {
		t.Fatalf("icon external URL mismatch: got %v", external["url"])
	}
}

func TestRunPageEditIconOnlyUsesOfficialAPIClear(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pageID + `","object":"page"}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := runPageEdit(&Context{}, pageID, "", "", "", "", "none")
	if err != nil {
		t.Fatalf("runPageEdit: %v", err)
	}
	if _, ok := gotBody["icon"]; !ok {
		t.Fatalf("icon key missing in payload: %#v", gotBody)
	}
	if gotBody["icon"] != nil {
		t.Fatalf("expected icon to be null, got %#v", gotBody["icon"])
	}
}

func TestRunPageEditIconOnlyRequiresValidIcon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := runPageEdit(&Context{}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "", "", "", "", "not-an-icon")
	if err == nil {
		t.Fatal("expected invalid icon error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "icon") {
		t.Fatalf("expected icon error, got %v", err)
	}
}

func TestRunPageEditIconOnlyRequiresExtractablePageIDFromURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := runPageEdit(&Context{}, "https://example.com/page-without-id", "", "", "", "", "✅")
	if err == nil {
		t.Fatal("expected page ID extraction error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "could not extract page id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPageEditIconOnlyRequiresOfficialAPIToken(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	// Isolate config by using a temp HOME and no API token.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", "http://127.0.0.1:65535/v1")

	err := runPageEdit(&Context{}, pageID, "", "", "", "", "✅")
	if err == nil {
		t.Fatal("expected missing official API token error")
	}
	if !strings.Contains(err.Error(), "official API token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPageEditIconOnlySupportsURLInputWithEmbeddedID(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	pageURL := "https://www.notion.so/My-Page-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		defer func() { _ = r.Body.Close() }()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + pageID + `","object":"page"}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := runPageEdit(&Context{}, pageURL, "", "", "", "", "✅")
	if err != nil {
		t.Fatalf("runPageEdit: %v", err)
	}
	if gotPath != "/v1/pages/"+pageID {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
}

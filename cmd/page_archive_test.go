package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunPageArchiveAcceptsCanonicalIDAndArchives(t *testing.T) {
	var sawAuthHeader string
	var sawPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuthHeader = r.Header.Get("Authorization")
		sawPath = r.URL.Path

		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := runPageArchive(&Context{
		APIToken:   "secret-token",
		APIBaseURL: srv.URL + "/v1",
	}, "https://www.notion.so/workspace/My-Page-12345678abcdef1234567890abcdef12")
	if err != nil {
		t.Fatalf("runPageArchive: %v", err)
	}

	if sawPath != "/v1/pages/12345678-abcd-ef12-3456-7890abcdef12" {
		t.Fatalf("path = %q", sawPath)
	}
	if sawAuthHeader != "Bearer secret-token" {
		t.Fatalf("authorization = %q", sawAuthHeader)
	}
}

func TestRunPageArchiveRejectsPageName(t *testing.T) {
	err := runPageArchive(&Context{}, "Meeting Notes")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "page URL or page ID") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPageArchiveSurfacesMissingOfficialAPIToken(t *testing.T) {
	err := runPageArchive(&Context{}, "12345678-abcd-ef12-3456-7890abcdef12")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "run 'notion-cli auth api setup' or set NOTION_API_TOKEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

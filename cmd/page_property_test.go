package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lox/notion-cli/internal/api"
)

func TestFindPropertyIDByName(t *testing.T) {
	props := map[string]api.PagePropertyMeta{
		"People": {ID: "prop_people", Type: "people"},
	}

	if id, ok := findPropertyIDByName(props, "People"); !ok || id != "prop_people" {
		t.Fatalf("exact lookup failed: id=%q ok=%v", id, ok)
	}
	if id, ok := findPropertyIDByName(props, "people"); !ok || id != "prop_people" {
		t.Fatalf("case-insensitive lookup failed: id=%q ok=%v", id, ok)
	}
}

func TestRunPagePropertyGetByPropertyID(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	propertyID := "prop_people"

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object":"list",
			"results":[{"id":"item_1"},{"id":"item_2"}],
			"has_more":false,
			"next_cursor":null
		}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := runPagePropertyGet(&Context{JSON: true}, pageID, "", propertyID)
	if err != nil {
		t.Fatalf("runPagePropertyGet: %v", err)
	}
	if gotPath != "/v1/pages/"+pageID+"/properties/"+propertyID {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
}

func TestRunPagePropertyGetByName(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	requestCount := 0
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			_, _ = w.Write([]byte(`{
				"object":"page",
				"properties":{
					"People":{"id":"prop_people","type":"people"}
				}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"object":"list",
			"results":[{"id":"item_1"}],
			"has_more":false,
			"next_cursor":null
		}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := runPagePropertyGet(&Context{JSON: true}, pageID, "People", "")
	if err != nil {
		t.Fatalf("runPagePropertyGet: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("unexpected request count: %#v", paths)
	}
	if paths[0] != "/v1/pages/"+pageID {
		t.Fatalf("page fetch path mismatch: %s", paths[0])
	}
	if paths[1] != "/v1/pages/"+pageID+"/properties/prop_people" {
		t.Fatalf("property fetch path mismatch: %s", paths[1])
	}
}

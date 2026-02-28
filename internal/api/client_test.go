package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/config"
)

func TestNewClientRequiresToken(t *testing.T) {
	t.Parallel()

	_, err := NewClient(config.APIConfig{}, "")
	if err == nil {
		t.Fatal("expected token error")
	}
}

func TestPatchPageSendsPatchRequest(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotVersion string
	var gotContentType string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Notion-Version")
		gotContentType = r.Header.Get("Content-Type")

		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"page-id","object":"page"}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{
		BaseURL:       srv.URL,
		NotionVersion: "2022-06-28",
	}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	patch := map[string]any{
		"archived": true,
	}

	if err := client.PatchPage(context.Background(), "page-id", patch); err != nil {
		t.Fatalf("patch page: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Fatalf("method mismatch: got %s", gotMethod)
	}
	if gotPath != "/pages/page-id" {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth mismatch: got %s", gotAuth)
	}
	if gotVersion != "2022-06-28" {
		t.Fatalf("notion-version mismatch: got %s", gotVersion)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type mismatch: got %s", gotContentType)
	}

	if gotBody["archived"] != true {
		t.Fatalf("archived mismatch: %v", gotBody["archived"])
	}
}

func TestPatchPageReturnsAPIErrorMessage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"object":"error","message":"unauthorized"}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = client.PatchPage(context.Background(), "page-id", map[string]any{"archived": true})
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized message, got: %v", err)
	}
}

func TestRetrievePageProperties(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object":"page",
			"properties":{
				"People":{"id":"prop_people","type":"people"},
				"Tags":{"id":"prop_tags","type":"multi_select"}
			}
		}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	props, err := client.RetrievePageProperties(context.Background(), "page-id")
	if err != nil {
		t.Fatalf("retrieve page properties: %v", err)
	}
	if gotPath != "/pages/page-id" {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
	if props["People"].ID != "prop_people" {
		t.Fatalf("people id mismatch: %#v", props["People"])
	}
}

func TestRetrievePagePropertyItemsPaginated(t *testing.T) {
	t.Parallel()

	requests := 0
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{
				"object":"list",
				"results":[{"id":"a"},{"id":"b"}],
				"has_more":true,
				"next_cursor":"cursor_2"
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"object":"list",
			"results":[{"id":"c"}],
			"has_more":false,
			"next_cursor":null
		}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	items, err := client.RetrievePagePropertyItems(context.Background(), "page-id", "prop-id")
	if err != nil {
		t.Fatalf("retrieve page property items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items len mismatch: got %d", len(items))
	}
	if len(paths) != 2 || paths[0] != "/pages/page-id/properties/prop-id" || paths[1] != "/pages/page-id/properties/prop-id?start_cursor=cursor_2" {
		t.Fatalf("unexpected request paths: %#v", paths)
	}
}

func TestRetrievePagePropertyItemsSingleResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object":"property_item",
			"id":"x",
			"type":"number",
			"number":42
		}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	items, err := client.RetrievePagePropertyItems(context.Background(), "page-id", "prop-id")
	if err != nil {
		t.Fatalf("retrieve page property items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items len mismatch: got %d", len(items))
	}
}

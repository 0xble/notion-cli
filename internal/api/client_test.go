package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
	var gotAccept string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Notion-Version")
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")

		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"page-id","object":"page","archived":true}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{
		BaseURL:       srv.URL,
		NotionVersion: "2022-06-28",
	}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	archived := true
	page, err := client.PatchPage(context.Background(), "page-id", PageUpdate{Archived: &archived})
	if err != nil {
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
	if gotAccept != "application/json" {
		t.Fatalf("accept mismatch: got %s", gotAccept)
	}

	if gotBody["archived"] != true {
		t.Fatalf("archived mismatch: %v", gotBody["archived"])
	}
	if page == nil || page.ID != "page-id" || !page.Archived {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestPatchPageEscapesPageID(t *testing.T) {
	t.Parallel()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"page id"}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	archived := true
	if _, err := client.PatchPage(context.Background(), "page id/with slash", PageUpdate{Archived: &archived}); err != nil {
		t.Fatalf("patch page: %v", err)
	}
	if gotPath != "/pages/page%20id%2Fwith%20slash" {
		t.Fatalf("unexpected escaped path: %s", gotPath)
	}
}

func TestPatchPageReturnsTypedAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Notion-Request-Id", "req-123")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"object":"error","code":"unauthorized","message":"invalid token"}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	archived := true
	_, err = client.PatchPage(context.Background(), "page-id", PageUpdate{Archived: &archived})
	if err == nil {
		t.Fatal("expected API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status: got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "unauthorized" {
		t.Fatalf("code: got %q", apiErr.Code)
	}
	if apiErr.Message != "invalid token" {
		t.Fatalf("message: got %q", apiErr.Message)
	}
	if apiErr.RequestID != "req-123" {
		t.Fatalf("request id: got %q", apiErr.RequestID)
	}
}

func TestPatchPageRetriesOn429(t *testing.T) {
	t.Parallel()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"rate_limited","message":"slow down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"page-id","archived":true}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	archived := true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	page, err := client.PatchPage(ctx, "page-id", PageUpdate{Archived: &archived})
	if err != nil {
		t.Fatalf("patch page: %v", err)
	}
	if page.ID != "page-id" {
		t.Fatalf("unexpected page: %+v", page)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected exactly one retry (2 calls), got %d", got)
	}
}

func TestPatchPageEmptyUpdateIsRejected(t *testing.T) {
	t.Parallel()

	client, err := NewClient(config.APIConfig{BaseURL: "https://example.invalid"}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.PatchPage(context.Background(), "page-id", PageUpdate{}); err == nil {
		t.Fatal("expected empty-update error")
	}
}

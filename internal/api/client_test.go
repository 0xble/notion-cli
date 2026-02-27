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

func TestParsePageIcon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		want     PageIcon
		wantErr  bool
		errMatch string
	}{
		{
			name:  "emoji",
			input: "✅",
			want:  PageIcon{Emoji: "✅"},
		},
		{
			name:  "external URL",
			input: "https://cdn.example.com/icon.png",
			want:  PageIcon{ExternalURL: "https://cdn.example.com/icon.png"},
		},
		{
			name:  "clear none",
			input: "none",
			want:  PageIcon{Clear: true},
		},
		{
			name:  "clear uppercase",
			input: "CLEAR",
			want:  PageIcon{Clear: true},
		},
		{
			name:     "invalid URL format",
			input:    "https://",
			wantErr:  true,
			errMatch: "icon URL",
		},
		{
			name:     "plain text is rejected",
			input:    "hello",
			wantErr:  true,
			errMatch: "emoji",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParsePageIcon(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				if tt.errMatch != "" && !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMatch)) {
					t.Fatalf("expected error containing %q, got %q", tt.errMatch, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("parse icon: %v", err)
			}
			if got != tt.want {
				t.Fatalf("icon mismatch: got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSetPageIconBuildsPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		icon       PageIcon
		wantIcon   any
		expectFail bool
	}{
		{
			name: "emoji",
			icon: PageIcon{Emoji: "🔥"},
			wantIcon: map[string]any{
				"type":  "emoji",
				"emoji": "🔥",
			},
		},
		{
			name: "external URL",
			icon: PageIcon{ExternalURL: "https://cdn.example.com/icon.png"},
			wantIcon: map[string]any{
				"type": "external",
				"external": map[string]any{
					"url": "https://cdn.example.com/icon.png",
				},
			},
		},
		{
			name:     "clear",
			icon:     PageIcon{Clear: true},
			wantIcon: nil,
		},
		{
			name:       "invalid icon shape",
			icon:       PageIcon{},
			expectFail: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() { _ = r.Body.Close() }()
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"page-id","object":"page"}`))
			}))
			defer srv.Close()

			client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
			if err != nil {
				t.Fatalf("new client: %v", err)
			}

			err = client.SetPageIcon(context.Background(), "page-id", tt.icon)
			if tt.expectFail {
				if err == nil {
					t.Fatalf("expected failure for %+v", tt.icon)
				}
				return
			}
			if err != nil {
				t.Fatalf("set page icon: %v", err)
			}

			if gotBody["icon"] == nil {
				if tt.wantIcon != nil {
					t.Fatalf("icon payload mismatch: got nil, want %v", tt.wantIcon)
				}
				return
			}
			if tt.wantIcon == nil {
				t.Fatalf("icon payload mismatch: got %v, want nil", gotBody["icon"])
			}

			gotJSON, err := json.Marshal(gotBody["icon"])
			if err != nil {
				t.Fatalf("marshal got icon: %v", err)
			}
			wantJSON, err := json.Marshal(tt.wantIcon)
			if err != nil {
				t.Fatalf("marshal want icon: %v", err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("icon payload mismatch: got %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestVerifyTokenSendsGetRequest(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotAuth string
	var gotVersion string
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotVersion = r.Header.Get("Notion-Version")
		gotContentType = r.Header.Get("Content-Type")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"user","id":"user-id"}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{
		BaseURL:       srv.URL,
		NotionVersion: "2022-06-28",
	}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if err := client.VerifyToken(context.Background()); err != nil {
		t.Fatalf("verify token: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Fatalf("method mismatch: got %s", gotMethod)
	}
	if gotPath != "/users/me" {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth mismatch: got %s", gotAuth)
	}
	if gotVersion != "2022-06-28" {
		t.Fatalf("notion-version mismatch: got %s", gotVersion)
	}
	if gotContentType != "" {
		t.Fatalf("content-type should be empty for GET, got %q", gotContentType)
	}
}

func TestVerifyTokenRequiresUserIDInResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"user"}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = client.VerifyToken(context.Background())
	if err == nil {
		t.Fatal("expected empty user id error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "empty user id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

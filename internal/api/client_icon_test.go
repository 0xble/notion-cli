package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lox/notion-cli/internal/config"
)

func TestParsePageIconValues(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  PageIcon
	}{
		{"✅", PageIcon{Emoji: "✅"}},
		{"https://example.com/icon.png", PageIcon{ExternalURL: "https://example.com/icon.png"}},
		{"none", PageIcon{Clear: true}},
	} {
		got, err := ParsePageIcon(tc.input)
		if err != nil {
			t.Fatalf("ParsePageIcon(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("ParsePageIcon(%q) = %+v, want %+v", tc.input, got, tc.want)
		}
	}
	if _, err := ParsePageIcon("plain text"); err == nil {
		t.Fatal("expected plain text to be rejected")
	}
}

func TestSetPageIconPayloads(t *testing.T) {
	for _, tc := range []struct {
		name string
		icon PageIcon
		want any
	}{
		{"emoji", PageIcon{Emoji: "🔥"}, map[string]any{"type": "emoji", "emoji": "🔥"}},
		{"external", PageIcon{ExternalURL: "https://example.com/icon.png"}, map[string]any{"type": "external", "external": map[string]any{"url": "https://example.com/icon.png"}}},
		{"clear", PageIcon{Clear: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch || r.URL.Path != "/v1/pages/page-id" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()
			client, err := NewClient(config.APIConfig{BaseURL: srv.URL + "/v1"}, "token")
			if err != nil {
				t.Fatal(err)
			}
			if err := client.SetPageIcon(context.Background(), "page-id", tc.icon); err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(body["icon"])
			want, _ := json.Marshal(tc.want)
			if string(got) != string(want) {
				t.Fatalf("icon payload = %s, want %s", got, want)
			}
		})
	}
}

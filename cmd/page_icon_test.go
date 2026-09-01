package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunPageEditIconOnly(t *testing.T) {
	pageID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/pages/"+pageID {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := &Context{APIBaseURL: srv.URL + "/v1", APIToken: "test-token"}
	if err := runPageEdit(ctx, pageID, "", "", "", "", nil, "✅", false); err != nil {
		t.Fatal(err)
	}
	icon := body["icon"].(map[string]any)
	if icon["type"] != "emoji" || icon["emoji"] != "✅" {
		t.Fatalf("unexpected icon payload: %#v", icon)
	}
}

func TestRunPageEditIconRejectsInvalidValue(t *testing.T) {
	err := runPageEdit(&Context{}, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "", "", "", "", nil, "plain text", false)
	if err == nil {
		t.Fatal("expected invalid icon error")
	}
}

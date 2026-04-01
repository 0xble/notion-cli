package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareLocalImageUploadsUploadsAndDeduplicates(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	createCalls := 0
	sendCalls := 0
	getCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads":
			createCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads/upload_123/send":
			sendCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/file_uploads/upload_123":
			getCalls++
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "secret-token")

	rewritten, uploads, err := prepareLocalImageUploads(context.Background(), doc, "![One](./diagram.png)\n![Two](./diagram.png)\n")
	if err != nil {
		t.Fatalf("prepareLocalImageUploads: %v", err)
	}
	if len(uploads) != 2 {
		t.Fatalf("len(uploads) = %d, want 2", len(uploads))
	}
	if createCalls != 1 || sendCalls != 1 || getCalls != 1 {
		t.Fatalf("unexpected call counts create=%d send=%d get=%d", createCalls, sendCalls, getCalls)
	}
	if uploads[0].FileUploadID != "upload_123" || uploads[1].FileUploadID != "upload_123" {
		t.Fatalf("unexpected upload ids: %#v", uploads)
	}
	if !strings.Contains(rewritten, uploads[0].Placeholder) || !strings.Contains(rewritten, uploads[1].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholders: %q", rewritten)
	}
}

func TestSubstituteUploadedLocalImagesAppendsAfterPlaceholderAndDeletes(t *testing.T) {
	var sawAppend bool
	var sawDelete bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/blocks/page_123/children":
			_, _ = w.Write([]byte(`{"results":[{"id":"block_123","type":"paragraph","paragraph":{"rich_text":[{"plain_text":"PLACEHOLDER"}]}}],"has_more":false}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/blocks/page_123/children":
			sawAppend = true
			defer func() { _ = r.Body.Close() }()
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if payload["after"] != "block_123" {
				t.Fatalf("after = %#v", payload["after"])
			}
			_, _ = w.Write([]byte(`{"results":[]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/blocks/block_123":
			sawDelete = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "secret-token")

	err := substituteUploadedLocalImages(context.Background(), "page_123", []uploadedLocalImage{{
		Alt:          "Diagram",
		FileUploadID: "upload_123",
		Placeholder:  "PLACEHOLDER",
		ResolvedPath: "/tmp/diagram.png",
	}})
	if err != nil {
		t.Fatalf("substituteUploadedLocalImages: %v", err)
	}
	if !sawAppend || !sawDelete {
		t.Fatalf("expected append and delete, saw append=%v delete=%v", sawAppend, sawDelete)
	}
}

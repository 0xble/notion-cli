package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMaybeUploadLocalImagesSkipsWhenAssetBaseURLSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	uploads, err := maybeUploadLocalImages(context.Background(), "/tmp/doc.md", "![A](./a.png)", "https://cdn.example.com/base", "")
	if err != nil {
		t.Fatalf("maybeUploadLocalImages: %v", err)
	}
	if len(uploads) != 0 {
		t.Fatalf("expected no uploads, got %d", len(uploads))
	}
}

func TestMaybeUploadLocalImagesUploadsAndDeduplicates(t *testing.T) {
	tmp := t.TempDir()
	docDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(filepath.Join(docDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	img := filepath.Join(docDir, "assets", "diagram.png")
	if err := os.WriteFile(img, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	doc := filepath.Join(docDir, "guide.md")
	markdown := "![One](./assets/diagram.png)\n![Two](./assets/diagram.png)\n"

	createCalls := 0
	sendCalls := 0
	getCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads":
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/file_uploads/upload_123/send":
			sendCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/v1/file_uploads/upload_123":
			getCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	uploads, err := maybeUploadLocalImages(context.Background(), doc, markdown, "", "")
	if err != nil {
		t.Fatalf("maybeUploadLocalImages: %v", err)
	}
	if len(uploads) != 2 {
		t.Fatalf("len(uploads)=%d, want 2", len(uploads))
	}
	if uploads[0].FileUploadID != "upload_123" || uploads[1].FileUploadID != "upload_123" {
		t.Fatalf("unexpected upload ids: %#v", uploads)
	}
	if createCalls != 1 || sendCalls != 1 || getCalls != 1 {
		t.Fatalf("unexpected call counts create=%d send=%d get=%d", createCalls, sendCalls, getCalls)
	}
}

func TestAppendUploadedLocalImages(t *testing.T) {
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/blocks/page_123/children" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[]}`))
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("NOTION_API_BASE_URL", srv.URL+"/v1")
	t.Setenv("NOTION_API_TOKEN", "test-token")

	err := appendUploadedLocalImages(context.Background(), "page_123", []uploadedLocalImage{
		{Alt: "Diagram", FileUploadID: "upload_1"},
	})
	if err != nil {
		t.Fatalf("appendUploadedLocalImages: %v", err)
	}

	children, ok := gotBody["children"].([]any)
	if !ok || len(children) != 1 {
		t.Fatalf("children payload mismatch: %#v", gotBody["children"])
	}
}

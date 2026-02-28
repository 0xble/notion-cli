package api

import (
	"context"
	"encoding/json"
	"io"
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

func TestUploadFileSinglePart(t *testing.T) {
	t.Parallel()

	var createVersion string
	var sendVersion string
	var getVersion string
	var sentFileName string
	var sentFileData string
	getCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/file_uploads":
			createVersion = r.Header.Get("Notion-Version")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/file_uploads/upload_123/send":
			sendVersion = r.Header.Get("Notion-Version")
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("parse multipart form: %v", err)
			}
			file, hdr, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("form file: %v", err)
			}
			defer func() { _ = file.Close() }()
			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("read form file: %v", err)
			}
			sentFileName = hdr.Filename
			sentFileData = string(data)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/file_uploads/upload_123":
			getVersion = r.Header.Get("Notion-Version")
			getCalls++
			w.Header().Set("Content-Type", "application/json")
			if getCalls == 1 {
				_, _ = w.Write([]byte(`{"id":"upload_123","status":"pending"}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"upload_123","status":"uploaded"}`))
			}
			return
		}

		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	id, err := client.UploadFile(context.Background(), "diagram.png", []byte("PNGDATA"))
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	if id != "upload_123" {
		t.Fatalf("upload id mismatch: got %q", id)
	}
	if sentFileName != "diagram.png" {
		t.Fatalf("file name mismatch: got %q", sentFileName)
	}
	if sentFileData != "PNGDATA" {
		t.Fatalf("file data mismatch: got %q", sentFileData)
	}
	if createVersion != "2025-09-03" || sendVersion != "2025-09-03" || getVersion != "2025-09-03" {
		t.Fatalf("unexpected notion version headers: create=%q send=%q get=%q", createVersion, sendVersion, getVersion)
	}
}

func TestAppendUploadedImageBlocks(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotVersion string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotVersion = r.Header.Get("Notion-Version")

		defer func() { _ = r.Body.Close() }()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","results":[]}`))
	}))
	defer srv.Close()

	client, err := NewClient(config.APIConfig{BaseURL: srv.URL}, "secret-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = client.AppendUploadedImageBlocks(context.Background(), "page_123", []UploadedImageBlock{
		{FileUploadID: "upload_1", Caption: "Diagram"},
	})
	if err != nil {
		t.Fatalf("append uploaded image blocks: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Fatalf("method mismatch: got %s", gotMethod)
	}
	if gotPath != "/blocks/page_123/children" {
		t.Fatalf("path mismatch: got %s", gotPath)
	}
	if gotVersion != "2025-09-03" {
		t.Fatalf("notion-version mismatch: got %s", gotVersion)
	}

	children, ok := gotBody["children"].([]any)
	if !ok || len(children) != 1 {
		t.Fatalf("children payload mismatch: %#v", gotBody["children"])
	}
	child, ok := children[0].(map[string]any)
	if !ok {
		t.Fatalf("child payload mismatch: %#v", children[0])
	}
	image, ok := child["image"].(map[string]any)
	if !ok {
		t.Fatalf("image payload mismatch: %#v", child["image"])
	}
	fileUpload, ok := image["file_upload"].(map[string]any)
	if !ok {
		t.Fatalf("file_upload payload mismatch: %#v", image["file_upload"])
	}
	if fileUpload["id"] != "upload_1" {
		t.Fatalf("file_upload id mismatch: got %#v", fileUpload["id"])
	}
}

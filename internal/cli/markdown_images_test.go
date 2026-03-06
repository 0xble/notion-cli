package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteLocalMarkdownImages(t *testing.T) {
	tmp := t.TempDir()
	docDir := filepath.Join(tmp, "docs")
	assetsDir := filepath.Join(docDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}

	imagePath := filepath.Join(assetsDir, "diagram.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	docFile := filepath.Join(docDir, "guide.md")

	md := "Intro\n\n![Diagram](./assets/diagram.png \"caption\")\n![Remote](https://example.com/x.png)\n"
	got, rewrites, err := RewriteLocalMarkdownImages(md, MarkdownImageRewriteOptions{
		SourceFile:   docFile,
		AssetBaseURL: "https://assets.example.com/notion",
	})
	if err != nil {
		t.Fatalf("RewriteLocalMarkdownImages() error: %v", err)
	}

	if len(rewrites) != 1 {
		t.Fatalf("rewrites len = %d, want 1", len(rewrites))
	}
	if rewrites[0].Resolved != imagePath {
		t.Fatalf("resolved = %q, want %q", rewrites[0].Resolved, imagePath)
	}
	if !strings.Contains(got, "![Diagram](https://assets.example.com/notion/assets/diagram.png)") {
		t.Fatalf("expected rewritten local image, got: %q", got)
	}
	if !strings.Contains(got, "![Remote](https://example.com/x.png)") {
		t.Fatalf("expected remote image untouched, got: %q", got)
	}
}

func TestRewriteLocalMarkdownImages_AssetRoot(t *testing.T) {
	tmp := t.TempDir()
	assetRoot := filepath.Join(tmp, "render")
	nested := filepath.Join(assetRoot, "images")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	imagePath := filepath.Join(nested, "chart 1.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	docFile := filepath.Join(tmp, "doc.md")
	md := "![Chart](<./render/images/chart 1.png>)\n"
	got, rewrites, err := RewriteLocalMarkdownImages(md, MarkdownImageRewriteOptions{
		SourceFile:   docFile,
		AssetBaseURL: "https://cdn.example.com/base/",
		AssetRoot:    assetRoot,
	})
	if err != nil {
		t.Fatalf("RewriteLocalMarkdownImages() error: %v", err)
	}

	if len(rewrites) != 1 {
		t.Fatalf("rewrites len = %d, want 1", len(rewrites))
	}

	want := "![Chart](https://cdn.example.com/base/images/chart%201.png)\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRewriteLocalMarkdownImages_FileURL(t *testing.T) {
	tmp := t.TempDir()
	imagePath := filepath.Join(tmp, "socket.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	docFile := filepath.Join(tmp, "doc.md")
	fileURL := "file://" + filepath.ToSlash(imagePath)
	md := "![Socket](" + fileURL + ")\n"
	got, rewrites, err := RewriteLocalMarkdownImages(md, MarkdownImageRewriteOptions{
		SourceFile:   docFile,
		AssetBaseURL: "https://assets.example.com",
	})
	if err != nil {
		t.Fatalf("RewriteLocalMarkdownImages() error: %v", err)
	}

	if len(rewrites) != 1 {
		t.Fatalf("rewrites len = %d, want 1", len(rewrites))
	}
	if got != "![Socket](https://assets.example.com/socket.png)\n" {
		t.Fatalf("unexpected rewrite: %q", got)
	}
}

func TestRewriteLocalMarkdownImages_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	docFile := filepath.Join(tmp, "doc.md")
	md := "![Missing](./missing.png)\n"
	_, _, err := RewriteLocalMarkdownImages(md, MarkdownImageRewriteOptions{
		SourceFile:   docFile,
		AssetBaseURL: "https://assets.example.com",
	})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRewriteLocalMarkdownImages_NoBaseURL(t *testing.T) {
	md := "![Local](./img.png)\n"
	got, rewrites, err := RewriteLocalMarkdownImages(md, MarkdownImageRewriteOptions{
		SourceFile: "/tmp/doc.md",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rewrites) != 0 {
		t.Fatalf("rewrites len = %d, want 0", len(rewrites))
	}
	if got != md {
		t.Fatalf("got %q, want %q", got, md)
	}
}

func TestFindLocalMarkdownImages(t *testing.T) {
	tmp := t.TempDir()
	docDir := filepath.Join(tmp, "docs")
	if err := os.MkdirAll(filepath.Join(docDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}

	img1 := filepath.Join(docDir, "assets", "diagram.png")
	if err := os.WriteFile(img1, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image1: %v", err)
	}
	img2 := filepath.Join(docDir, "assets", "chart.jpg")
	if err := os.WriteFile(img2, []byte("jpg"), 0o644); err != nil {
		t.Fatalf("write image2: %v", err)
	}

	docFile := filepath.Join(docDir, "guide.md")
	md := "![Diagram](./assets/diagram.png)\n![Remote](https://example.com/r.png)\n![Chart](./assets/chart.jpg)\n"

	got, err := FindLocalMarkdownImages(md, docFile)
	if err != nil {
		t.Fatalf("FindLocalMarkdownImages() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got)=%d, want 2", len(got))
	}
	if got[0].Resolved != img1 {
		t.Fatalf("first resolved = %q, want %q", got[0].Resolved, img1)
	}
	if got[0].Alt != "Diagram" {
		t.Fatalf("first alt = %q, want %q", got[0].Alt, "Diagram")
	}
	if got[1].Resolved != img2 {
		t.Fatalf("second resolved = %q, want %q", got[1].Resolved, img2)
	}
}

func TestFindLocalMarkdownImages_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	docFile := filepath.Join(tmp, "doc.md")
	md := "![Missing](./missing.png)\n"

	_, err := FindLocalMarkdownImages(md, docFile)
	if err == nil {
		t.Fatal("expected error for missing local file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

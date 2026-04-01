package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripLocalImagesRemovesStandaloneLocalImageLines(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	markdown := "# Title\n\n![Local](./diagram.png)\n\nParagraph\n"

	got, err := stripLocalImages(doc, markdown)
	if err != nil {
		t.Fatalf("stripLocalImages: %v", err)
	}

	want := "# Title\n\n\n\nParagraph\n"
	if got != want {
		t.Fatalf("stripLocalImages() = %q, want %q", got, want)
	}
}

func TestStripLocalImagesPreservesRemoteImages(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	markdown := "![Remote](https://example.com/remote.png)\n![Local](./diagram.png)\n"

	got, err := stripLocalImages(doc, markdown)
	if err != nil {
		t.Fatalf("stripLocalImages: %v", err)
	}

	want := "![Remote](https://example.com/remote.png)\n\n"
	if got != want {
		t.Fatalf("stripLocalImages() = %q, want %q", got, want)
	}
}

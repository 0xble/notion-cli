package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteStandaloneLocalImagesRewritesStandaloneLocalLines(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages("# Title\n\n![Diagram](./diagram.png)\n\nDone\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
	if placements[0].Resolved != img {
		t.Fatalf("Resolved = %q, want %q", placements[0].Resolved, img)
	}
}

func TestRewriteStandaloneLocalImagesHandlesCRLF(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rewritten, placements, err := RewriteStandaloneLocalImages("# Title\r\n\r\n![Diagram](./diagram.png)\r\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
}

func TestRewriteStandaloneLocalImagesRejectsInlineLocalImage(t *testing.T) {
	tmp := t.TempDir()
	doc := filepath.Join(tmp, "doc.md")
	img := filepath.Join(tmp, "diagram.png")
	if err := os.WriteFile(img, []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := RewriteStandaloneLocalImages("before ![Diagram](./diagram.png) after\n", doc)
	if err == nil || !strings.Contains(err.Error(), "must appear on their own line") {
		t.Fatalf("expected unsupported syntax error, got %v", err)
	}
}

func TestRewriteStandaloneLocalImagesIgnoresRemoteImages(t *testing.T) {
	doc := filepath.Join(t.TempDir(), "doc.md")

	rewritten, placements, err := RewriteStandaloneLocalImages("![Diagram](https://example.test/diagram.png)\n", doc)
	if err != nil {
		t.Fatalf("RewriteStandaloneLocalImages: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0", len(placements))
	}
	if rewritten != "![Diagram](https://example.test/diagram.png)\n" {
		t.Fatalf("rewritten = %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesAcceptsMissingFiles(t *testing.T) {
	rewritten, placements, err := FindStandaloneLocalImageLines("# Title\n\n![Diagram](./does-not-exist.png)\n\nDone\n")
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("len(placements) = %d, want 1", len(placements))
	}
	if placements[0].Resolved != "" {
		t.Fatalf("Resolved = %q, want empty (strip mode does not resolve paths)", placements[0].Resolved)
	}
	if placements[0].Placeholder == "" {
		t.Fatalf("Placeholder was not set")
	}
	if !strings.Contains(rewritten, placements[0].Placeholder) {
		t.Fatalf("rewritten markdown missing placeholder: %q", rewritten)
	}
	if strings.Contains(rewritten, "does-not-exist.png") {
		t.Fatalf("rewritten markdown still contains original image line: %q", rewritten)
	}
}

func TestFindStandaloneLocalImageLinesRejectsInlineLocalImage(t *testing.T) {
	_, _, err := FindStandaloneLocalImageLines("before ![Diagram](./diagram.png) after\n")
	if err == nil || !strings.Contains(err.Error(), "must appear on their own line") {
		t.Fatalf("expected unsupported syntax error, got %v", err)
	}
}

func TestFindStandaloneLocalImageLinesIgnoresRemoteImages(t *testing.T) {
	rewritten, placements, err := FindStandaloneLocalImageLines("![Diagram](https://example.test/diagram.png)\n")
	if err != nil {
		t.Fatalf("FindStandaloneLocalImageLines: %v", err)
	}
	if len(placements) != 0 {
		t.Fatalf("len(placements) = %d, want 0", len(placements))
	}
	if rewritten != "![Diagram](https://example.test/diagram.png)\n" {
		t.Fatalf("rewritten = %q", rewritten)
	}
}

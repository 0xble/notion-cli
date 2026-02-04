package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
}

func NewMarkdownRenderer() (*MarkdownRenderer, error) {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
		if width > 120 {
			width = 120
		}
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, fmt.Errorf("creating markdown renderer: %w", err)
	}

	return &MarkdownRenderer{renderer: r}, nil
}

func (m *MarkdownRenderer) Render(content string) (string, error) {
	content = preprocessNotionMarkdown(content)

	out, err := m.renderer.Render(content)
	if err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func (m *MarkdownRenderer) RenderAndPrint(content string) error {
	out, err := m.Render(content)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func preprocessNotionMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inCallout := false
	calloutContent := []string{}

	for _, line := range lines {
		if strings.HasPrefix(line, "> ℹ️") || strings.HasPrefix(line, "> ⚠️") ||
			strings.HasPrefix(line, "> 💡") || strings.HasPrefix(line, "> 📌") ||
			strings.HasPrefix(line, "> ❗") || strings.HasPrefix(line, "> 🔥") {
			inCallout = true
			calloutContent = append(calloutContent, line)
			continue
		}

		if inCallout {
			if strings.HasPrefix(line, "> ") {
				calloutContent = append(calloutContent, line)
				continue
			} else {
				result = append(result, calloutContent...)
				result = append(result, "")
				calloutContent = nil
				inCallout = false
			}
		}

		result = append(result, line)
	}

	if len(calloutContent) > 0 {
		result = append(result, calloutContent...)
	}

	return strings.Join(result, "\n")
}

func RenderMarkdown(content string) error {
	r, err := NewMarkdownRenderer()
	if err != nil {
		return err
	}
	return r.RenderAndPrint(content)
}

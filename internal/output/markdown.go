package output

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
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

// RenderPage renders a Notion page with pretty metadata header
func RenderPage(content string) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	meta, body := parseNotionResponse(content)

	if meta != nil {
		renderPageHeader(meta, isTTY)
	}

	if body != "" {
		r, err := NewMarkdownRenderer()
		if err != nil {
			return err
		}
		return r.RenderAndPrint(body)
	}

	return nil
}

type pageMetadata struct {
	Title     string
	URL       string
	Created   string
	Author    string
	Type      string
	ExtraInfo string
}

func parseNotionResponse(content string) (*pageMetadata, string) {
	meta := &pageMetadata{}

	// Extract URL from <page url="{{...}}"> tag
	pageTagRe := regexp.MustCompile(`<page url="\{\{([^}]+)\}\}"`)
	if match := pageTagRe.FindStringSubmatch(content); len(match) > 1 {
		meta.URL = match[1]
	}

	// Extract properties JSON from <properties> tag
	if start := strings.Index(content, "<properties>"); start != -1 {
		if end := strings.Index(content[start:], "</properties>"); end != -1 {
			propsContent := content[start+len("<properties>") : start+end]
			propsContent = strings.TrimSpace(propsContent)
			var data map[string]any
			if err := json.Unmarshal([]byte(propsContent), &data); err == nil {
				if name, ok := data["Name"].(string); ok {
					meta.Title = name
				}
				if title, ok := data["title"].(string); ok && meta.Title == "" {
					meta.Title = title
				}
				if url, ok := data["url"].(string); ok {
					meta.URL = cleanNotionURL(url)
				}
				if created, ok := data["Created"].(string); ok {
					meta.Created = created
				}
			}
		}
	}

	// Extract content from <content> tag
	contentRe := regexp.MustCompile(`(?s)<content>\s*(.*?)\s*</content>`)
	if match := contentRe.FindStringSubmatch(content); len(match) > 1 {
		body := match[1]
		// Clean up Notion-specific markup
		body = cleanNotionMarkup(body)
		return meta, body
	}

	// Check for database
	if strings.Contains(content, "<database") {
		meta.Type = "database"
		// Extract title
		if strings.Contains(content, "The title of this Database is:") {
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "The title of this Database is:") {
					meta.Title = strings.TrimSpace(strings.TrimPrefix(line, "The title of this Database is:"))
					break
				}
			}
		}
		// Format database content nicely
		body := formatDatabaseContent(content)
		return meta, body
	}

	// Fallback: return raw content
	return meta, content
}

func formatDatabaseContent(content string) string {
	var out strings.Builder

	// Extract and format schema
	if start := strings.Index(content, "<data-source-state>"); start != -1 {
		if end := strings.Index(content[start:], "</data-source-state>"); end != -1 {
			stateJSON := strings.TrimSpace(content[start+len("<data-source-state>") : start+end])
			var state struct {
				Name   string `json:"name"`
				Schema map[string]struct {
					Name    string `json:"name"`
					Type    string `json:"type"`
					Options []struct {
						Name string `json:"name"`
					} `json:"options,omitempty"`
				} `json:"schema"`
			}
			if err := json.Unmarshal([]byte(stateJSON), &state); err == nil {
				out.WriteString("## Schema\n\n")
				out.WriteString("| Column | Type |\n")
				out.WriteString("|--------|------|\n")
				for _, prop := range state.Schema {
					typeStr := prop.Type
					if len(prop.Options) > 0 {
						opts := make([]string, 0, len(prop.Options))
						for _, opt := range prop.Options {
							opts = append(opts, opt.Name)
						}
						typeStr = fmt.Sprintf("%s (%s)", prop.Type, strings.Join(opts, ", "))
					}
					out.WriteString(fmt.Sprintf("| %s | %s |\n", prop.Name, typeStr))
				}
				out.WriteString("\n")
			}
		}
	}

	// Extract and format views
	if strings.Contains(content, "<views>") {
		out.WriteString("## Views\n\n")
		viewRe := regexp.MustCompile(`<view url="[^"]*">`)
		viewStarts := viewRe.FindAllStringIndex(content, -1)
		for _, loc := range viewStarts {
			start := loc[1]
			end := strings.Index(content[start:], "</view>")
			if end != -1 {
				viewJSON := strings.TrimSpace(content[start : start+end])
				var view struct {
					Name string `json:"name"`
					Type string `json:"type"`
				}
				if err := json.Unmarshal([]byte(viewJSON), &view); err == nil {
					out.WriteString(fmt.Sprintf("- **%s** (%s)\n", view.Name, view.Type))
				}
			}
		}
		out.WriteString("\n")
	}

	return out.String()
}

func cleanNotionMarkup(content string) string {
	// Transform callouts to blockquotes with icon
	content = transformCallouts(content)

	// Transform columns - flatten to sequential content
	content = transformColumns(content)

	// Transform child page links: <page url="...">Title</page> -> [📄 Title](url)
	// Add newline before if not at line start
	pageRe := regexp.MustCompile(`<page url="\{\{([^}]+)\}\}"[^>]*>([^<]+)</page>`)
	content = pageRe.ReplaceAllString(content, "\n- [📄 $2]($1)")

	// Transform inline databases: <database url="..." inline="true">Title</database> -> [📊 Title](url)
	dbRe := regexp.MustCompile(`<database url="\{\{([^}]+)\}\}"[^>]*>([^<]+)</database>`)
	content = dbRe.ReplaceAllString(content, "\n**[📊 $2]($1)**\n")

	// Transform mention-page with title: <mention-page url="...">Title</mention-page> -> [Title](url)
	mentionWithTitleRe := regexp.MustCompile(`<mention-page url="\{\{([^}]+)\}\}">([^<]+)</mention-page>`)
	content = mentionWithTitleRe.ReplaceAllString(content, "\n- [$2]($1)")

	// Transform mention-page without content: <mention-page url="..."/> -> [→ page](url)
	mentionEmptyRe := regexp.MustCompile(`<mention-page url="\{\{([^}]+)\}\}"/>`)
	content = mentionEmptyRe.ReplaceAllString(content, "\n- [→ page]($1)")

	// Remove <span discussion-urls="...">...</span> wrappers but keep content
	spanRe := regexp.MustCompile(`<span[^>]*>([^<]*)</span>`)
	content = spanRe.ReplaceAllString(content, "$1")

	// Remove empty blocks and unknown elements
	content = regexp.MustCompile(`<empty-block\s*/>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<unknown[^>]*/>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`<omitted\s*/>`).ReplaceAllString(content, "")

	// Clean up color annotations like {color="gray"}
	content = regexp.MustCompile(`\s*\{color="[^"]+"\}`).ReplaceAllString(content, "")

	// Clean up Notion URL wrappers in remaining markdown links
	content = strings.ReplaceAll(content, "{{", "")
	content = strings.ReplaceAll(content, "}}", "")

	// Clean up Slack channel links: [#channel](slackChannel://...) -> #channel
	slackRe := regexp.MustCompile(`\[([^\]]+)\]\(slackChannel://[^)]+\)`)
	content = slackRe.ReplaceAllString(content, "$1")

	// Clean up excess blank lines
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")

	return strings.TrimSpace(content)
}

func transformCallouts(content string) string {
	// Match <callout icon="..." color="...">content</callout>
	calloutRe := regexp.MustCompile(`(?s)<callout icon="([^"]*)"[^>]*>\s*(.*?)\s*</callout>`)

	return calloutRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := calloutRe.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}

		icon := parts[1]
		body := parts[2]

		// Handle custom emoji (notion://custom_emoji/...) - use a generic icon
		if strings.HasPrefix(icon, "notion://") {
			icon = "💡"
		}

		// Clean internal page links within callout - keep inline
		pageRe := regexp.MustCompile(`<page url="\{\{([^}]+)\}\}"[^>]*>([^<]+)</page>`)
		body = pageRe.ReplaceAllString(body, "**[$2]($1)**")

		// Clean internal mention-page links within callout
		mentionRe := regexp.MustCompile(`<mention-page url="\{\{([^}]+)\}\}">([^<]+)</mention-page>`)
		body = mentionRe.ReplaceAllString(body, "[$2]($1)")

		// Format as blockquote
		lines := strings.Split(strings.TrimSpace(body), "\n")
		var quoted []string
		for _, line := range lines {
			quoted = append(quoted, "> "+strings.TrimSpace(line))
		}

		return fmt.Sprintf("> %s\n%s\n", icon, strings.Join(quoted, "\n"))
	})
}

func transformColumns(content string) string {
	// Match <columns>...</columns> and flatten
	columnsRe := regexp.MustCompile(`(?s)<columns>\s*(.*?)\s*</columns>`)

	return columnsRe.ReplaceAllStringFunc(content, func(match string) string {
		parts := columnsRe.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		inner := parts[1]

		// Extract content from each <column>...</column>
		columnRe := regexp.MustCompile(`(?s)<column>\n?(.*?)\s*</column>`)
		columns := columnRe.FindAllStringSubmatch(inner, -1)

		var result []string
		for _, col := range columns {
			if len(col) >= 2 {
				// Dedent column content
				colContent := dedentContent(col[1])
				result = append(result, colContent)
			}
		}

		return strings.Join(result, "\n\n")
	})
}

func dedentContent(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	// Find minimum indentation (excluding empty lines), counting tabs as 1 char
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Count leading whitespace (tabs or spaces)
		indent := 0
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				indent++
			} else {
				break
			}
		}
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return strings.TrimSpace(content)
	}

	// Remove common indentation
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result = append(result, "")
			continue
		}
		// Skip minIndent characters
		runes := []rune(line)
		if len(runes) >= minIndent {
			result = append(result, string(runes[minIndent:]))
		} else {
			result = append(result, strings.TrimSpace(line))
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func cleanNotionURL(url string) string {
	// Remove {{ }} wrappers
	url = strings.TrimPrefix(url, "{{")
	url = strings.TrimSuffix(url, "}}")
	return url
}

func renderPageHeader(meta *pageMetadata, isTTY bool) {
	if meta.Title == "" && meta.URL == "" {
		return
	}

	if isTTY {
		titleStyle := color.New(color.Bold, color.FgWhite)
		urlStyle := color.New(color.Faint)
		labelStyle := color.New(color.Faint)

		fmt.Println()
		if meta.Title != "" {
			titleStyle.Println(meta.Title)
		}
		if meta.URL != "" {
			urlStyle.Println(meta.URL)
		}
		if meta.Type != "" {
			labelStyle.Printf("Type: ")
			fmt.Println(meta.Type)
		}
		fmt.Println()
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println()
	} else {
		if meta.Title != "" {
			fmt.Printf("Title: %s\n", meta.Title)
		}
		if meta.URL != "" {
			fmt.Printf("URL: %s\n", meta.URL)
		}
		fmt.Println()
	}
}

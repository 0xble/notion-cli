package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type LocalImagePlacement struct {
	Alt         string
	Original    string
	Resolved    string
	Placeholder string
}

var markdownImageRE = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\n]+)\)`)
var standaloneMarkdownImageRE = regexp.MustCompile(`^\s*!\[([^\]]*)\]\(([^)\n]+)\)\s*$`)
var uriSchemeRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

func RewriteStandaloneLocalImages(markdown, sourceFile string) (string, []LocalImagePlacement, error) {
	sourceFileAbs, err := filepath.Abs(sourceFile)
	if err != nil {
		return "", nil, fmt.Errorf("resolve source file path: %w", err)
	}
	sourceDir := filepath.Dir(sourceFileAbs)

	return scanStandaloneLocalImages(markdown, func(dest string) (string, error) {
		resolvedPath, err := resolveLocalPath(dest, sourceDir)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("local image %q not found (from %s): %w", dest, sourceFile, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("local image %q resolves to a directory: %s", dest, resolvedPath)
		}
		return resolvedPath, nil
	})
}

// FindStandaloneLocalImageLines rewrites standalone local image lines into
// placeholders without validating that the referenced files exist on disk. It
// still rejects inline or mixed-content local image syntax with the same error
// as RewriteStandaloneLocalImages, preserving the "local images must appear on
// their own line" invariant. Use this when the caller only needs to identify
// and strip local image lines (e.g. page upload/sync --skip-local-images) and
// does not need the resolved file path.
func FindStandaloneLocalImageLines(markdown string) (string, []LocalImagePlacement, error) {
	return scanStandaloneLocalImages(markdown, nil)
}

// scanStandaloneLocalImages walks markdown line-by-line, rejecting inline local
// images and replacing each standalone local image line with a placeholder.
// When resolvePath is non-nil, it is invoked for every local destination and
// its returned path is stored on the placement's Resolved field; a non-nil
// error from resolvePath aborts the scan. When resolvePath is nil, placements
// are recorded without a Resolved path.
func scanStandaloneLocalImages(markdown string, resolvePath func(dest string) (string, error)) (string, []LocalImagePlacement, error) {
	normalizedMarkdown := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(markdown)
	lines := strings.Split(normalizedMarkdown, "\n")
	placements := make([]LocalImagePlacement, 0)
	for i, line := range lines {
		matches := markdownImageRE.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		standalone := standaloneMarkdownImageRE.FindStringSubmatch(line)
		if standalone == nil {
			for _, match := range matches {
				dest, ok := parseMarkdownDestination(match[2])
				if ok && isLocalDestination(dest) {
					return "", nil, fmt.Errorf("unsupported local image syntax on line %d: local images must appear on their own line", i+1)
				}
			}
			continue
		}

		dest, ok := parseMarkdownDestination(standalone[2])
		if !ok || !isLocalDestination(dest) {
			continue
		}

		var resolvedPath string
		if resolvePath != nil {
			r, err := resolvePath(dest)
			if err != nil {
				return "", nil, err
			}
			resolvedPath = r
		}

		placeholder := "NOTION_CLI_LOCAL_IMAGE_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
		lines[i] = placeholder
		placements = append(placements, LocalImagePlacement{
			Alt:         standalone[1],
			Original:    dest,
			Resolved:    resolvedPath,
			Placeholder: placeholder,
		})
	}

	return strings.Join(lines, "\n"), placements, nil
}

func parseMarkdownDestination(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}

	if strings.HasPrefix(s, "<") {
		end := strings.Index(s, ">")
		if end > 1 {
			return s[1:end], true
		}
	}

	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			s = s[:i]
			break
		}
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func isLocalDestination(dest string) bool {
	d := strings.TrimSpace(dest)
	if d == "" {
		return false
	}

	if len(d) >= 2 && d[1] == ':' {
		return true
	}

	lower := strings.ToLower(d)
	switch {
	case strings.HasPrefix(lower, "#"):
		return false
	case strings.HasPrefix(lower, "http://"),
		strings.HasPrefix(lower, "https://"),
		strings.HasPrefix(lower, "mailto:"),
		strings.HasPrefix(lower, "tel:"),
		strings.HasPrefix(lower, "data:"):
		return false
	case strings.HasPrefix(lower, "file://"):
		return true
	}

	return !uriSchemeRE.MatchString(d)
}

func resolveLocalPath(dest, sourceDir string) (string, error) {
	d := strings.TrimSpace(dest)
	if strings.HasPrefix(strings.ToLower(d), "file://") {
		parsed, err := url.Parse(d)
		if err != nil {
			return "", fmt.Errorf("invalid file URL %q: %w", d, err)
		}
		unescaped, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return "", fmt.Errorf("invalid file URL path %q: %w", d, err)
		}
		d = unescaped
	}

	if strings.HasPrefix(d, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home path %q: %w", d, err)
		}
		d = filepath.Join(home, strings.TrimPrefix(d, "~"+string(filepath.Separator)))
	}

	if !filepath.IsAbs(d) {
		d = filepath.Join(sourceDir, d)
	}

	abs, err := filepath.Abs(d)
	if err != nil {
		return "", fmt.Errorf("resolve local path %q: %w", dest, err)
	}
	return filepath.Clean(abs), nil
}

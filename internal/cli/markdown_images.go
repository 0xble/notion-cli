package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type MarkdownImageRewriteOptions struct {
	SourceFile   string
	AssetBaseURL string
	AssetRoot    string
}

type MarkdownImageRewrite struct {
	Original string
	Resolved string
	URL      string
}

type LocalMarkdownImage struct {
	Alt      string
	Original string
	Resolved string
}

var markdownImageRE = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\n]+)\)`)
var uriSchemeRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

// RewriteLocalMarkdownImages rewrites local markdown image links to absolute URLs.
// If AssetBaseURL is empty, markdown is returned unchanged.
func RewriteLocalMarkdownImages(markdown string, opts MarkdownImageRewriteOptions) (string, []MarkdownImageRewrite, error) {
	if strings.TrimSpace(opts.AssetBaseURL) == "" {
		return markdown, nil, nil
	}

	baseURL, err := url.Parse(strings.TrimSpace(opts.AssetBaseURL))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return "", nil, fmt.Errorf("invalid asset base URL %q", opts.AssetBaseURL)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return "", nil, fmt.Errorf("asset base URL must use http or https: %q", opts.AssetBaseURL)
	}

	sourceFileAbs, err := filepath.Abs(opts.SourceFile)
	if err != nil {
		return "", nil, fmt.Errorf("resolve source file path: %w", err)
	}
	sourceDir := filepath.Dir(sourceFileAbs)

	assetRootAbs := ""
	if strings.TrimSpace(opts.AssetRoot) != "" {
		assetRootAbs, err = filepath.Abs(opts.AssetRoot)
		if err != nil {
			return "", nil, fmt.Errorf("resolve asset root path: %w", err)
		}
		assetRootAbs = filepath.Clean(assetRootAbs)
	}

	matches := markdownImageRE.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return markdown, nil, nil
	}

	var out strings.Builder
	out.Grow(len(markdown) + len(matches)*16)

	last := 0
	rewrites := make([]MarkdownImageRewrite, 0, len(matches))
	for _, m := range matches {
		matchStart, matchEnd := m[0], m[1]
		altStart, altEnd := m[2], m[3]
		destStart, destEnd := m[4], m[5]

		out.WriteString(markdown[last:matchStart])

		alt := markdown[altStart:altEnd]
		rawDest := markdown[destStart:destEnd]

		dest, ok := parseMarkdownDestination(rawDest)
		if !ok || !isLocalDestination(dest) {
			out.WriteString(markdown[matchStart:matchEnd])
			last = matchEnd
			continue
		}

		originalDest := dest
		resolvedPath, err := resolveLocalPath(dest, sourceDir)
		if err != nil {
			return "", nil, err
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			return "", nil, fmt.Errorf("local image %q not found (from %s): %w", originalDest, opts.SourceFile, err)
		}
		if info.IsDir() {
			return "", nil, fmt.Errorf("local image %q resolves to a directory: %s", originalDest, resolvedPath)
		}

		urlPath := buildURLPath(originalDest, resolvedPath, sourceDir, assetRootAbs)
		assetURL := joinBaseURL(baseURL, urlPath)

		out.WriteString("![")
		out.WriteString(alt)
		out.WriteString("](")
		out.WriteString(assetURL)
		out.WriteString(")")

		rewrites = append(rewrites, MarkdownImageRewrite{
			Original: originalDest,
			Resolved: resolvedPath,
			URL:      assetURL,
		})
		last = matchEnd
	}

	out.WriteString(markdown[last:])
	return out.String(), rewrites, nil
}

// FindLocalMarkdownImages returns all local markdown image links in order.
func FindLocalMarkdownImages(markdown, sourceFile string) ([]LocalMarkdownImage, error) {
	sourceFileAbs, err := filepath.Abs(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("resolve source file path: %w", err)
	}
	sourceDir := filepath.Dir(sourceFileAbs)

	matches := markdownImageRE.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	local := make([]LocalMarkdownImage, 0, len(matches))
	for _, m := range matches {
		altStart, altEnd := m[2], m[3]
		destStart, destEnd := m[4], m[5]

		alt := markdown[altStart:altEnd]
		rawDest := markdown[destStart:destEnd]

		dest, ok := parseMarkdownDestination(rawDest)
		if !ok || !isLocalDestination(dest) {
			continue
		}

		resolvedPath, err := resolveLocalPath(dest, sourceDir)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("local image %q not found (from %s): %w", dest, sourceFile, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("local image %q resolves to a directory: %s", dest, resolvedPath)
		}

		local = append(local, LocalMarkdownImage{
			Alt:      alt,
			Original: dest,
			Resolved: resolvedPath,
		})
	}

	return local, nil
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

	// Windows absolute paths like C:\foo are local paths.
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

func buildURLPath(originalDest, resolvedPath, sourceDir, assetRootAbs string) string {
	if assetRootAbs != "" {
		if rel, ok := relativeInside(assetRootAbs, resolvedPath); ok {
			return rel
		}
	}

	if !filepath.IsAbs(originalDest) && !strings.HasPrefix(strings.ToLower(originalDest), "file://") {
		return filepath.ToSlash(filepath.Clean(originalDest))
	}

	if rel, ok := relativeInside(sourceDir, resolvedPath); ok {
		return rel
	}

	return filepath.Base(resolvedPath)
}

func relativeInside(root, target string) (string, bool) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", false
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return "", true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func joinBaseURL(base *url.URL, relPath string) string {
	u := *base
	basePath := strings.TrimSuffix(u.Path, "/")
	if relPath == "" {
		if basePath == "" {
			u.Path = "/"
		} else {
			u.Path = basePath
		}
		return u.String()
	}
	if basePath == "" {
		u.Path = "/" + strings.TrimPrefix(relPath, "/")
	} else {
		u.Path = basePath + "/" + strings.TrimPrefix(relPath, "/")
	}
	return u.String()
}

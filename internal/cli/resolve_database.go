package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type DatabaseRefKind int

const (
	DatabaseRefID DatabaseRefKind = iota
	DatabaseRefURL
	DatabaseRefName
)

type DatabaseRef struct {
	Kind DatabaseRefKind
	Raw  string
	ID   string
}

func ParseDatabaseRef(s string) DatabaseRef {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if id, ok := ExtractNotionUUID(s); ok {
			return DatabaseRef{Kind: DatabaseRefID, Raw: s, ID: id}
		}
		return DatabaseRef{Kind: DatabaseRefURL, Raw: s}
	}
	if LooksLikeID(s) {
		id, _ := ExtractNotionUUID(s)
		return DatabaseRef{Kind: DatabaseRefID, Raw: s, ID: id}
	}
	return DatabaseRef{Kind: DatabaseRefName, Raw: s}
}

func ResolveDatabaseID(ctx context.Context, client *mcp.Client, input string) (string, error) {
	ref := ParseDatabaseRef(input)
	switch ref.Kind {
	case DatabaseRefID:
		return ref.ID, nil
	case DatabaseRefURL:
		if id, ok := ExtractNotionUUID(input); ok {
			return id, nil
		}
		return "", &output.UserError{Message: fmt.Sprintf("could not extract database ID from URL: %s\nUse the database ID directly instead.", input)}
	case DatabaseRefName:
		return resolveDatabaseByName(ctx, client, input)
	}
	return "", &output.UserError{Message: "invalid database reference: " + input}
}

func resolveDatabaseByName(ctx context.Context, client *mcp.Client, name string) (string, error) {
	resp, err := client.Search(ctx, name, &mcp.SearchOptions{ContentSearchMode: "workspace_search"})
	if err != nil {
		return "", err
	}

	var exactKnown []mcp.SearchResult
	var exactUnknown []mcp.SearchResult
	for _, r := range resp.Results {
		if !strings.EqualFold(r.Title, name) {
			continue
		}
		if isDatabaseSearchResult(r) {
			exactKnown = append(exactKnown, r)
			continue
		}
		if isUnknownSearchType(r) {
			exactUnknown = append(exactUnknown, r)
		}
	}
	if len(exactKnown) == 1 {
		return exactKnown[0].ID, nil
	}
	if len(exactKnown) > 1 {
		return "", ambiguousDatabaseError(name, exactKnown)
	}
	if len(exactUnknown) > 0 {
		exactVerified := filterDatabaseCandidatesByFetch(ctx, client, exactUnknown)
		if len(exactVerified) == 1 {
			return exactVerified[0].ID, nil
		}
		if len(exactVerified) > 1 {
			return "", ambiguousDatabaseError(name, exactVerified)
		}
	}

	var partialKnown []mcp.SearchResult
	var partialUnknown []mcp.SearchResult
	for _, r := range resp.Results {
		if !strings.Contains(strings.ToLower(r.Title), strings.ToLower(name)) {
			continue
		}
		if isDatabaseSearchResult(r) {
			partialKnown = append(partialKnown, r)
			continue
		}
		if isUnknownSearchType(r) {
			partialUnknown = append(partialUnknown, r)
		}
	}
	if len(partialKnown) == 0 && len(partialUnknown) > 0 {
		partialKnown = filterDatabaseCandidatesByFetch(ctx, client, partialUnknown)
	}
	if len(partialKnown) == 0 {
		return "", &output.UserError{Message: "database not found: " + name}
	}
	return "", ambiguousDatabaseError(name, partialKnown)
}

func ambiguousDatabaseError(name string, matches []mcp.SearchResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "ambiguous database name %q, matching databases:\n", name)
	limit := len(matches)
	if limit > 5 {
		limit = 5
	}
	for _, m := range matches[:limit] {
		if m.URL != "" {
			fmt.Fprintf(&b, "  %s (%s)\n", m.Title, m.URL)
		} else {
			fmt.Fprintf(&b, "  %s (%s)\n", m.Title, m.ID)
		}
	}
	if len(matches) > 5 {
		fmt.Fprintf(&b, "  ... and %d more\n", len(matches)-5)
	}
	b.WriteString("Use a database URL or ID to be specific.")
	return &output.UserError{Message: b.String()}
}

func isDatabaseSearchResult(r mcp.SearchResult) bool {
	return strings.EqualFold(strings.TrimSpace(r.ObjectType), "database") ||
		strings.EqualFold(strings.TrimSpace(r.Object), "database")
}

func isUnknownSearchType(r mcp.SearchResult) bool {
	return strings.TrimSpace(r.ObjectType) == "" && strings.TrimSpace(r.Object) == ""
}

func filterDatabaseCandidatesByFetch(ctx context.Context, client *mcp.Client, candidates []mcp.SearchResult) []mcp.SearchResult {
	out := make([]mcp.SearchResult, 0, len(candidates))
	for _, c := range candidates {
		fetched, err := client.Fetch(ctx, c.ID)
		if err != nil || fetched == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(fetched.ObjectType), "database") ||
			strings.Contains(fetched.Content, "<database ") {
			out = append(out, c)
		}
	}
	return out
}

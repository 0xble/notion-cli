package cli

import (
	"testing"

	"github.com/lox/notion-cli/internal/mcp"
)

func TestIsDatabaseSearchResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   mcp.SearchResult
		want bool
	}{
		{
			name: "object_type_database",
			in:   mcp.SearchResult{ObjectType: "database"},
			want: true,
		},
		{
			name: "object_database_case_insensitive",
			in:   mcp.SearchResult{Object: "DataBase"},
			want: true,
		},
		{
			name: "non_database",
			in:   mcp.SearchResult{Object: "page"},
			want: false,
		},
		{
			name: "unknown_empty",
			in:   mcp.SearchResult{},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isDatabaseSearchResult(tt.in)
			if got != tt.want {
				t.Fatalf("isDatabaseSearchResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUnknownSearchType(t *testing.T) {
	t.Parallel()

	if !isUnknownSearchType(mcp.SearchResult{}) {
		t.Fatal("expected empty result to be unknown type")
	}
	if isUnknownSearchType(mcp.SearchResult{ObjectType: "database"}) {
		t.Fatal("expected non-empty object_type not to be unknown")
	}
	if isUnknownSearchType(mcp.SearchResult{Object: "page"}) {
		t.Fatal("expected non-empty object not to be unknown")
	}
}

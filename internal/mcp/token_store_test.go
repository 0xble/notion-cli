package mcp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFileTokenStoreUsesProfilePath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := NewFileTokenStore("work")
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}

	if got := filepath.Base(store.Path()); got != "token.json" {
		t.Fatalf("token filename = %q, want token.json", got)
	}
	if !strings.Contains(store.Path(), filepath.Join("profiles", "work")) {
		t.Fatalf("token path = %q, want profiles/work segment", store.Path())
	}
}

package cmd

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lox/notion-cli/internal/api"
)

type fakeOfficialSearcher struct {
	req api.SearchRequest
	res *api.SearchResponse
	err error
}

func (f *fakeOfficialSearcher) Search(_ context.Context, req api.SearchRequest) (*api.SearchResponse, error) {
	f.req = req
	return f.res, f.err
}

func TestSearchPagesUsesOfficialPageFilter(t *testing.T) {
	searcher := &fakeOfficialSearcher{
		res: &api.SearchResponse{
			Results: []api.SearchObject{
				{
					Object: "page",
					ID:     "page_123",
					URL:    "https://notion.so/page_123",
					Properties: map[string]json.RawMessage{
						"Name": mustRaw(`{"type":"title","title":[{"plain_text":"Tasks"}]}`),
					},
				},
				{Object: "database", ID: "db_123"},
			},
		},
	}

	pages, err := searchPages(context.Background(), searcher, "Tasks", 10)
	if err != nil {
		t.Fatalf("searchPages: %v", err)
	}
	if searcher.req.Filter == nil || searcher.req.Filter.Property != "object" || searcher.req.Filter.Value != "page" {
		t.Fatalf("filter = %#v", searcher.req.Filter)
	}
	if searcher.req.Query != "Tasks" || searcher.req.PageSize != 10 {
		t.Fatalf("request = %#v", searcher.req)
	}
	if len(pages) != 1 || pages[0].ID != "page_123" || pages[0].Title != "Tasks" {
		t.Fatalf("pages = %#v", pages)
	}
}

func mustRaw(s string) json.RawMessage {
	return json.RawMessage(s)
}

func TestSearchDatabasesUsesOfficialDataSourceFilter(t *testing.T) {
	searcher := &fakeOfficialSearcher{
		res: &api.SearchResponse{
			Results: []api.SearchObject{
				{
					Object:      "data_source",
					ID:          "db_123",
					URL:         "https://notion.so/db_123",
					Title:       []api.RichText{{PlainText: "Tasks"}},
					Description: []api.RichText{{PlainText: "Open work"}},
				},
				{Object: "page", ID: "page_123"},
			},
		},
	}

	dbs, err := searchDatabases(context.Background(), searcher, "", 150)
	if err != nil {
		t.Fatalf("searchDatabases: %v", err)
	}
	if searcher.req.Filter == nil || searcher.req.Filter.Property != "object" || searcher.req.Filter.Value != "data_source" {
		t.Fatalf("filter = %#v", searcher.req.Filter)
	}
	if searcher.req.PageSize != 100 {
		t.Fatalf("page size = %d", searcher.req.PageSize)
	}
	if len(dbs) != 1 || dbs[0].ID != "db_123" || dbs[0].Title != "Tasks" || dbs[0].Description != "Open work" {
		t.Fatalf("dbs = %#v", dbs)
	}
}

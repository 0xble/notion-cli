package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lox/notion-cli/internal/api"
)

func TestResolveDataSourceByName(t *testing.T) {
	searcher := &fakeOfficialSearcher{
		res: &api.SearchResponse{
			Results: []api.SearchObject{
				{Object: "data_source", ID: "source_123", Title: []api.RichText{{PlainText: "Tasks"}}},
			},
		},
	}

	got, err := resolveDataSourceID(context.Background(), searcher, "Tasks")
	if err != nil {
		t.Fatalf("resolveDataSourceID: %v", err)
	}
	if got != "source_123" {
		t.Fatalf("id = %q", got)
	}
	if searcher.req.Filter == nil || searcher.req.Filter.Value != "data_source" {
		t.Fatalf("filter = %#v", searcher.req.Filter)
	}
}

func TestResolveDataSourceByNameReportsAmbiguousMatches(t *testing.T) {
	searcher := &fakeOfficialSearcher{
		res: &api.SearchResponse{
			Results: []api.SearchObject{
				{Object: "data_source", ID: "source_1", Title: []api.RichText{{PlainText: "Tasks"}}},
				{Object: "data_source", ID: "source_2", Title: []api.RichText{{PlainText: "Tasks"}}},
			},
		},
	}

	_, err := resolveDataSourceID(context.Background(), searcher, "Tasks")
	if err == nil || !strings.Contains(err.Error(), "multiple data sources match") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseJSONObjectAndJSONArray(t *testing.T) {
	obj, err := parseJSONObject(`{"property":"Status"}`, "filter")
	if err != nil {
		t.Fatalf("parseJSONObject: %v", err)
	}
	if obj["property"] != "Status" {
		t.Fatalf("object = %#v", obj)
	}

	arr, err := parseJSONArray(`[{"timestamp":"created_time","direction":"descending"}]`, "sorts")
	if err != nil {
		t.Fatalf("parseJSONArray: %v", err)
	}
	if len(arr) != 1 || arr[0]["direction"] != "descending" {
		t.Fatalf("array = %#v", arr)
	}
}

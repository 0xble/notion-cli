package cmd

import (
	"context"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/output"
)

const defaultSearchPageSize = 20

type officialSearcher interface {
	Search(ctx context.Context, req api.SearchRequest) (*api.SearchResponse, error)
}

func searchPageSize(limit int) int {
	if limit <= 0 {
		return defaultSearchPageSize
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func searchPages(ctx context.Context, client officialSearcher, query string, limit int) ([]output.Page, error) {
	resp, err := client.Search(ctx, api.SearchRequest{
		Query:    query,
		Filter:   &api.SearchFilter{Property: "object", Value: "page"},
		PageSize: searchPageSize(limit),
	})
	if err != nil {
		return nil, err
	}

	pages := make([]output.Page, 0, len(resp.Results))
	for _, result := range resp.Results {
		if result.Object != "page" {
			continue
		}
		if limit > 0 && len(pages) >= limit {
			break
		}
		pages = append(pages, output.Page{
			ID:             result.ID,
			Title:          result.DisplayTitle(),
			URL:            result.URL,
			CreatedTime:    result.CreatedTime,
			LastEditedTime: result.LastEditedTime,
			Archived:       result.InTrash || result.Archived,
		})
	}
	return pages, nil
}

func searchDatabases(ctx context.Context, client officialSearcher, query string, limit int) ([]output.Database, error) {
	resp, err := client.Search(ctx, api.SearchRequest{
		Query:    query,
		Filter:   &api.SearchFilter{Property: "object", Value: "data_source"},
		PageSize: searchPageSize(limit),
	})
	if err != nil {
		return nil, err
	}

	dbs := make([]output.Database, 0, len(resp.Results))
	for _, result := range resp.Results {
		if result.Object != "data_source" {
			continue
		}
		if limit > 0 && len(dbs) >= limit {
			break
		}
		dbs = append(dbs, output.Database{
			ID:             result.ID,
			Title:          result.DisplayTitle(),
			URL:            result.URL,
			CreatedTime:    result.CreatedTime,
			LastEditedTime: result.LastEditedTime,
			Description:    result.DisplayDescription(),
		})
	}
	return dbs, nil
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/output"
)

type SourceCmd struct {
	List      SourceListCmd      `cmd:"" help:"List data sources"`
	View      SourceViewCmd      `cmd:"" help:"View a data source"`
	Query     SourceQueryCmd     `cmd:"" help:"Query a data source"`
	Templates SourceTemplatesCmd `cmd:"" help:"List data source templates"`
}

type SourceListCmd struct {
	Query string `help:"Filter data sources by name" short:"q"`
	Limit int    `help:"Maximum number of results" short:"l" default:"20"`
	JSON  bool   `help:"Output as JSON" short:"j"`
}

func (c *SourceListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runSourceList(ctx, c.Query, c.Limit)
}

type SourceViewCmd struct {
	Source string `arg:"" help:"Data source URL, ID, or name"`
	JSON   bool   `help:"Output as JSON" short:"j"`
}

func (c *SourceViewCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runSourceView(ctx, c.Source)
}

type SourceQueryCmd struct {
	Source     string `arg:"" help:"Data source URL, ID, or name"`
	Limit      int    `help:"Maximum number of results" short:"l" default:"20"`
	FilterJSON string `help:"Raw Notion filter JSON" name:"filter-json"`
	SortsJSON  string `help:"Raw Notion sorts JSON array" name:"sorts-json"`
	ResultType string `help:"Filter wiki results to page or data_source"`
	InTrash    *bool  `help:"Filter by trash status" name:"in-trash"`
	JSON       bool   `help:"Output raw JSON" short:"j"`
}

func (c *SourceQueryCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runSourceQuery(ctx, c.Source, c.Limit, c.FilterJSON, c.SortsJSON, c.ResultType, c.InTrash)
}

type SourceTemplatesCmd struct {
	Source string `arg:"" help:"Data source URL, ID, or name"`
	Name   string `help:"Filter templates by name" short:"q"`
	Limit  int    `help:"Maximum number of templates" short:"l" default:"20"`
	JSON   bool   `help:"Output as JSON" short:"j"`
}

func (c *SourceTemplatesCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runSourceTemplates(ctx, c.Source, c.Name, c.Limit)
}

func runSourceList(ctx *Context, query string, limit int) error {
	client, err := cli.RequireOfficialAPIClient(officialAPIOverrides(ctx))
	if err != nil {
		return err
	}

	bgCtx := context.Background()
	dbs, err := searchDatabases(bgCtx, client, query, limit)
	if err != nil {
		output.PrintError(err)
		return err
	}
	return output.PrintDatabases(dbs, ctx.JSON)
}

func runSourceView(ctx *Context, source string) error {
	client, err := cli.RequireOfficialAPIClient(officialAPIOverrides(ctx))
	if err != nil {
		return err
	}

	bgCtx := context.Background()
	sourceID, err := resolveDataSourceID(bgCtx, client, source)
	if err != nil {
		output.PrintError(err)
		return err
	}

	dataSource, err := client.GetDataSource(bgCtx, sourceID)
	if err != nil {
		output.PrintError(err)
		return err
	}
	if ctx.JSON {
		return output.PrintJSON(dataSource)
	}
	return output.PrintDatabases([]output.Database{dataSourceToDatabase(*dataSource)}, false)
}

func runSourceQuery(ctx *Context, source string, limit int, filterJSON, sortsJSON, resultType string, inTrash *bool) error {
	client, err := cli.RequireOfficialAPIClient(officialAPIOverrides(ctx))
	if err != nil {
		return err
	}

	filter, err := parseJSONObject(filterJSON, "filter")
	if err != nil {
		output.PrintError(err)
		return err
	}
	sorts, err := parseJSONArray(sortsJSON, "sorts")
	if err != nil {
		output.PrintError(err)
		return err
	}
	resultType = strings.TrimSpace(resultType)
	if resultType != "" && resultType != "page" && resultType != "data_source" {
		err := &output.UserError{Message: "invalid result type: " + resultType + " (expected page or data_source)"}
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	sourceID, err := resolveDataSourceID(bgCtx, client, source)
	if err != nil {
		output.PrintError(err)
		return err
	}

	resp, err := client.QueryDataSource(bgCtx, sourceID, api.DataSourceQueryRequest{
		Filter:     filter,
		Sorts:      sorts,
		PageSize:   searchPageSize(limit),
		InTrash:    inTrash,
		ResultType: resultType,
	})
	if err != nil {
		output.PrintError(err)
		return err
	}
	if ctx.JSON {
		return output.PrintJSON(resp)
	}

	results := make([]output.SearchResult, 0, len(resp.Results))
	for _, result := range resp.Results {
		results = append(results, output.SearchResult{
			ID:    result.ID,
			Type:  result.Object,
			Title: result.DisplayTitle(),
			URL:   result.URL,
		})
	}
	return output.PrintSearchResults(results, false)
}

func runSourceTemplates(ctx *Context, source, name string, limit int) error {
	client, err := cli.RequireOfficialAPIClient(officialAPIOverrides(ctx))
	if err != nil {
		return err
	}

	bgCtx := context.Background()
	sourceID, err := resolveDataSourceID(bgCtx, client, source)
	if err != nil {
		output.PrintError(err)
		return err
	}

	resp, err := client.ListDataSourceTemplates(bgCtx, sourceID, name, "", searchPageSize(limit))
	if err != nil {
		output.PrintError(err)
		return err
	}
	if ctx.JSON {
		return output.PrintJSON(resp)
	}

	results := make([]output.SearchResult, 0, len(resp.Templates))
	for _, template := range resp.Templates {
		title := template.Name
		if title == "" {
			title = template.ID
		}
		results = append(results, output.SearchResult{
			ID:    template.ID,
			Type:  "template",
			Title: title,
			URL:   template.URL,
		})
	}
	return output.PrintSearchResults(results, false)
}

func resolveDataSourceID(ctx context.Context, client officialSearcher, input string) (string, error) {
	ref := cli.ParsePageRef(input)
	switch ref.Kind {
	case cli.RefID:
		return ref.ID, nil
	case cli.RefURL:
		if id, ok := cli.ExtractNotionUUID(input); ok {
			return id, nil
		}
		return "", &output.UserError{Message: fmt.Sprintf("could not extract data source ID from URL: %s\nUse the data source ID directly instead.", input)}
	case cli.RefName:
		return resolveDataSourceByName(ctx, client, input)
	}
	return "", &output.UserError{Message: "invalid data source reference: " + input}
}

func resolveDataSourceByName(ctx context.Context, client officialSearcher, name string) (string, error) {
	resp, err := client.Search(ctx, api.SearchRequest{
		Query:    name,
		Filter:   &api.SearchFilter{Property: "object", Value: "data_source"},
		PageSize: 10,
	})
	if err != nil {
		return "", err
	}

	var exactMatches []api.SearchObject
	var partialMatches []api.SearchObject
	for _, result := range resp.Results {
		if result.Object != "data_source" {
			continue
		}
		title := result.DisplayTitle()
		if strings.EqualFold(title, name) {
			exactMatches = append(exactMatches, result)
			continue
		}
		if strings.Contains(strings.ToLower(title), strings.ToLower(name)) {
			partialMatches = append(partialMatches, result)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0].ID, nil
	}
	if len(exactMatches) > 1 {
		return "", ambiguousDataSourceError(name, exactMatches)
	}
	if len(partialMatches) == 1 {
		return partialMatches[0].ID, nil
	}
	if len(partialMatches) > 1 {
		return "", ambiguousDataSourceError(name, partialMatches)
	}
	return "", &output.UserError{Message: "data source not found: " + name}
}

func ambiguousDataSourceError(name string, matches []api.SearchObject) error {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "multiple data sources match %q:\n", name)
	for _, match := range matches {
		title := match.DisplayTitle()
		if title == "" {
			title = match.ID
		}
		_, _ = fmt.Fprintf(&b, "  %s (%s)\n", title, match.URL)
	}
	b.WriteString("Use a data source ID or URL instead.")
	return &output.UserError{Message: b.String()}
}

func dataSourceToDatabase(source api.SearchObject) output.Database {
	return output.Database{
		ID:             source.ID,
		Title:          source.DisplayTitle(),
		URL:            source.URL,
		CreatedTime:    source.CreatedTime,
		LastEditedTime: source.LastEditedTime,
		Description:    source.DisplayDescription(),
	}
}

func parseJSONObject(raw, label string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, &output.UserError{Message: fmt.Sprintf("invalid %s JSON: %v", label, err)}
	}
	return out, nil
}

func parseJSONArray(raw, label string) ([]map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, &output.UserError{Message: fmt.Sprintf("invalid %s JSON: %v", label, err)}
	}
	return out, nil
}

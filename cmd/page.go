package cmd

import (
	"context"

	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type PageCmd struct {
	List   PageListCmd   `cmd:"" help:"List pages"`
	View   PageViewCmd   `cmd:"" help:"View a page"`
	Create PageCreateCmd `cmd:"" help:"Create a page"`
}

type PageListCmd struct {
	Query string `help:"Filter pages by name" short:"q"`
	Limit int    `help:"Maximum number of results" short:"l" default:"20"`
	JSON  bool   `help:"Output as JSON" short:"j"`
}

func (c *PageListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageList(ctx, c.Query, c.Limit)
}

func runPageList(ctx *Context, query string, limit int) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bgCtx := context.Background()

	searchQuery := query
	if searchQuery == "" {
		searchQuery = "*"
	}

	resp, err := client.Search(bgCtx, searchQuery)
	if err != nil {
		output.PrintError(err)
		return err
	}

	pages := filterPages(resp.Results, limit)
	return output.PrintPages(pages, ctx.JSON)
}

func filterPages(results []mcp.SearchResult, limit int) []output.Page {
	pages := make([]output.Page, 0)
	for _, r := range results {
		if r.ObjectType != "page" && r.Object != "page" {
			continue
		}
		if limit > 0 && len(pages) >= limit {
			break
		}
		pages = append(pages, output.Page{
			ID:    r.ID,
			Title: r.Title,
			URL:   r.URL,
		})
	}
	return pages
}

type PageViewCmd struct {
	URL  string `arg:"" help:"Page URL or ID"`
	JSON bool   `help:"Output as JSON" short:"j"`
}

func (c *PageViewCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageView(ctx, c.URL)
}

func runPageView(ctx *Context, url string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bgCtx := context.Background()
	result, err := client.Fetch(bgCtx, url)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if result.Content == "" {
		output.PrintWarning("No content found")
		return nil
	}

	return output.RenderMarkdown(result.Content)
}

type PageCreateCmd struct {
	Title   string `help:"Page title" short:"t" required:""`
	Parent  string `help:"Parent page ID" short:"p"`
	Content string `help:"Page content (markdown)" short:"c"`
	JSON    bool   `help:"Output as JSON" short:"j"`
}

func (c *PageCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageCreate(ctx, c.Title, c.Parent, c.Content)
}

func runPageCreate(ctx *Context, title, parent, content string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer client.Close()

	bgCtx := context.Background()
	req := mcp.CreatePageRequest{
		Title:        title,
		ParentPageID: parent,
		Content:      content,
	}

	page, err := client.CreatePage(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	outPage := output.Page{
		ID:             page.ID,
		URL:            page.URL,
		CreatedTime:    page.CreatedTime,
		LastEditedTime: page.LastEditedTime,
		Archived:       page.Archived,
		Title:          title,
	}

	if ctx.JSON {
		return output.PrintPage(outPage, true)
	}

	output.PrintSuccess("Page created: " + page.URL)
	return nil
}

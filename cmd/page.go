package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type PageCmd struct {
	List    PageListCmd    `cmd:"" help:"List pages"`
	View    PageViewCmd    `cmd:"" help:"View a page"`
	Create  PageCreateCmd  `cmd:"" help:"Create a page"`
	Upload  PageUploadCmd  `cmd:"" help:"Upload a markdown file as a page"`
	Sync    PageSyncCmd    `cmd:"" help:"Sync a markdown file to a page (create or update)"`
	Edit    PageEditCmd    `cmd:"" help:"Edit a page"`
	Archive PageArchiveCmd `cmd:"" help:"Archive a page"`
	Delete  PageDeleteCmd  `cmd:"" help:"Delete a page (move to trash)"`
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
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	searchQuery := query
	if searchQuery == "" {
		searchQuery = "*"
	}

	resp, err := client.Search(bgCtx, searchQuery, &mcp.SearchOptions{ContentSearchMode: "workspace_search"})
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
	Page string `arg:"" help:"Page URL, name, or ID"`
	JSON bool   `help:"Output as JSON" short:"j"`
	Raw  bool   `help:"Output raw Notion response without formatting" short:"r"`
}

func (c *PageViewCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageView(ctx, c.Page, c.Raw)
}

func runPageView(ctx *Context, page string, raw bool) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	fetchID := page
	if ref.Kind == cli.RefName {
		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		fetchID = resolved
	}

	result, err := client.Fetch(bgCtx, fetchID)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if result.Content == "" {
		output.PrintWarning("No content found")
		return nil
	}

	if raw {
		fmt.Println(result.Content)
		return nil
	}

	return output.RenderPage(result.Content)
}

type PageCreateCmd struct {
	Title   string `help:"Page title" short:"t" required:""`
	Parent  string `help:"Parent page URL, name, or ID" short:"p"`
	Content string `help:"Page content (markdown)" short:"c"`
	Icon    string `help:"Page icon (emoji, https URL, or 'none' to clear)" short:"i"`
	JSON    bool   `help:"Output as JSON" short:"j"`
}

func (c *PageCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageCreate(ctx, c.Title, c.Parent, c.Content, c.Icon)
}

func runPageCreate(ctx *Context, title, parent, content, icon string) error {
	explicitIcon, parsedIcon, err := parseExplicitIcon(icon)
	if err != nil {
		output.PrintError(err)
		return err
	}

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	parentID := parent
	if parent != "" {
		resolved, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		parentID = resolved
	}

	req := mcp.CreatePageRequest{
		Title:        title,
		ParentPageID: parentID,
		Content:      content,
	}

	resp, err := client.CreatePage(bgCtx, req)
	if err != nil {
		output.PrintError(err)
		return err
	}

	pageID := pageIDFromCreateResponse(resp)
	if explicitIcon {
		if pageID == "" {
			output.PrintWarning("Page created but could not retrieve ID to apply icon")
		} else if err := setPageIcon(bgCtx, pageID, parsedIcon); err != nil {
			output.PrintError(err)
			return err
		}
	}

	if ctx.JSON {
		outPage := output.Page{
			ID:    pageID,
			URL:   resp.URL,
			Title: title,
			Icon:  outputIconValue(icon, explicitIcon, parsedIcon),
		}
		return output.PrintPage(outPage, true)
	}

	if resp.URL != "" {
		output.PrintSuccess("Page created: " + resp.URL)
	} else {
		output.PrintSuccess("Page created")
	}
	return nil
}

type PageUploadCmd struct {
	File         string   `arg:"" help:"Markdown file to upload" type:"existingfile"`
	Title        string   `help:"Page title (default: filename or first heading)" short:"t"`
	Parent       string   `help:"Parent page URL, name, or ID" short:"p"`
	ParentDB     string   `help:"Parent database URL, name, or ID" name:"parent-db" short:"d"`
	Icon         string   `help:"Page icon (emoji, https URL, or 'none' to clear)" short:"i"`
	JSON         bool     `help:"Output as JSON" short:"j"`
	AssetBaseURL string   `help:"Base URL used to rewrite local image embeds (or NOTION_CLI_ASSET_BASE_URL)"`
	AssetRoot    string   `help:"Local asset root mapped to --asset-base-url (or NOTION_CLI_ASSET_ROOT)"`
	PropertyMode string   `help:"Property sync mode: off, warn, or strict" enum:"off,warn,strict" default:"warn" name:"property-mode"`
	Props        []string `help:"Semicolon-delimited properties (key=value;key2=value2). Repeatable." name:"props"`
	Prop         []string `help:"Single property assignment key=value. Repeatable." name:"prop"`
}

func (c *PageUploadCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageUpload(ctx, c.File, c.Title, c.Parent, c.ParentDB, c.Icon, c.AssetBaseURL, c.AssetRoot, c.PropertyMode, c.Props, c.Prop)
}

func runPageUpload(ctx *Context, file, title, parent, parentDB, icon, assetBaseURL, assetRoot, propertyModeRaw string, propsFlags, propFlags []string) error {
	explicitIcon, parsedIcon, err := parseExplicitIcon(icon)
	if err != nil {
		output.PrintError(err)
		return err
	}

	content, err := os.ReadFile(file)
	if err != nil {
		output.PrintError(err)
		return err
	}

	markdown := string(content)
	markdown, rewrittenCount, err := rewriteLocalImages(file, markdown, assetBaseURL, assetRoot)
	if err != nil {
		output.PrintError(err)
		return err
	}
	if rewrittenCount > 0 {
		output.PrintInfo(fmt.Sprintf("Rewrote %d local image(s) to hosted URLs", rewrittenCount))
	}

	if title == "" {
		title = extractTitleFromMarkdown(markdown)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}

	if icon == "" {
		icon, title = extractEmojiFromTitle(title)
	}

	propertyMode, err := cli.ParsePropertyMode(propertyModeRaw)
	if err != nil {
		output.PrintError(err)
		return err
	}

	flagProperties := map[string]any{}
	if propertyMode != cli.PropertyModeOff {
		var parseErrs []error
		flagProperties, parseErrs = cli.ParsePropertiesFlags(propsFlags, propFlags)
		if err := handlePropertyParseErrors(propertyMode, parseErrs); err != nil {
			output.PrintError(err)
			return err
		}
	}

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	req := mcp.CreatePageRequest{
		Title:      title,
		Content:    markdown,
		Properties: flagProperties,
	}

	if parentDB != "" {
		dbID, err := cli.ResolveDatabaseID(bgCtx, client, parentDB)
		if err != nil {
			output.PrintError(err)
			return err
		}
		dbID, err = client.ResolveDataSourceID(bgCtx, dbID)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentDatabaseID = dbID
	} else if parent != "" {
		parentID, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentPageID = parentID
	}

	resp, err := client.CreatePage(bgCtx, req)
	if err != nil {
		if len(flagProperties) > 0 && propertyMode == cli.PropertyModeWarn {
			output.PrintWarning("Page creation with properties failed; retrying without properties: " + err.Error())
			req.Properties = nil
			resp, err = client.CreatePage(bgCtx, req)
		}
		if err != nil {
			output.PrintError(err)
			return err
		}
	}

	pageID := pageIDFromCreateResponse(resp)
	if explicitIcon {
		if pageID == "" {
			output.PrintWarning("Page created but could not retrieve ID to apply icon")
		} else if err := setPageIcon(bgCtx, pageID, parsedIcon); err != nil {
			output.PrintError(err)
			return err
		}
	}

	displayTitle := titleWithIcon(title, icon)

	if ctx.JSON {
		outPage := output.Page{
			ID:    pageID,
			URL:   resp.URL,
			Title: displayTitle,
			Icon:  outputIconValue(icon, explicitIcon, parsedIcon),
		}
		return output.PrintPage(outPage, true)
	}

	output.PrintSuccess("Uploaded: " + displayTitle)
	if resp.URL != "" {
		output.PrintInfo(resp.URL)
	}
	return nil
}

func extractTitleFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func extractEmojiFromTitle(title string) (icon, cleanTitle string) {
	runes := []rune(title)
	if len(runes) == 0 {
		return "", title
	}

	first := runes[0]
	if cli.IsEmoji(first) {
		rest := strings.TrimSpace(string(runes[1:]))
		return string(first), rest
	}

	return "", title
}

type PageEditCmd struct {
	Page        string `arg:"" help:"Page URL, name, or ID"`
	Replace     string `help:"Replace entire content with this text" xor:"action"`
	Find        string `help:"Text to find (use ... for ellipsis)"`
	ReplaceWith string `help:"Text to replace with (requires --find)" name:"replace-with"`
	Append      string `help:"Append text after selection (requires --find)"`
	Icon        string `help:"Page icon (emoji, https URL, or 'none' to clear)"`
}

func (c *PageEditCmd) Run(ctx *Context) error {
	return runPageEdit(ctx, c.Page, c.Replace, c.Find, c.ReplaceWith, c.Append, c.Icon)
}

func runPageEdit(ctx *Context, page, replace, find, replaceWith, appendText, icon string) error {
	explicitIcon, parsedIcon, err := parseExplicitIcon(icon)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if replace != "" && (find != "" || replaceWith != "" || appendText != "") {
		return &output.UserError{Message: "use --replace alone, or --find with --replace-with/--append"}
	}
	if find == "" && (replaceWith != "" || appendText != "") {
		return &output.UserError{Message: "--replace-with/--append require --find"}
	}
	if replaceWith != "" && appendText != "" {
		return &output.UserError{Message: "use either --replace-with or --append with --find"}
	}
	needsContentUpdate := replace != "" || (find != "" && (replaceWith != "" || appendText != ""))
	if !needsContentUpdate && !explicitIcon {
		return &output.UserError{Message: "specify --replace, --find with --replace-with/--append, or --icon"}
	}

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	pageID := ref.ID
	var client *mcp.Client
	switch ref.Kind {
	case cli.RefName:
		client, err = cli.RequireClient()
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()

		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		pageID = resolved
	case cli.RefID:
		pageID = ref.ID
	case cli.RefURL:
		if extractedID, ok := cli.ExtractNotionUUID(page); ok {
			pageID = extractedID
			break
		}
		return &output.UserError{Message: fmt.Sprintf("could not extract page ID from URL: %s\nUse the page ID directly instead.", page)}
	}

	if needsContentUpdate {
		if client == nil {
			client, err = cli.RequireClient()
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()
		}

		req := mcp.UpdatePageRequest{PageID: pageID}
		switch {
		case replace != "":
			req.Command = "replace_content"
			req.NewContent = replace
		case find != "" && replaceWith != "":
			req.Command = "replace_content_range"
			req.Selection = find
			req.NewStr = replaceWith
		case find != "" && appendText != "":
			req.Command = "insert_content_after"
			req.Selection = find
			req.NewStr = appendText
		}

		if err := client.UpdatePage(bgCtx, req); err != nil {
			output.PrintError(err)
			return err
		}
	}

	if explicitIcon {
		if err := setPageIcon(bgCtx, pageID, parsedIcon); err != nil {
			output.PrintError(err)
			return err
		}
	}

	switch {
	case needsContentUpdate && explicitIcon:
		output.PrintSuccess("Page content and icon updated")
	case needsContentUpdate:
		output.PrintSuccess("Page updated")
	default:
		output.PrintSuccess("Page icon updated")
	}
	return nil
}

type PageArchiveCmd struct {
	Page string `arg:"" help:"Page URL, name, or ID"`
}

func (c *PageArchiveCmd) Run(ctx *Context) error {
	return runPageArchive(ctx, c.Page)
}

func runPageArchive(ctx *Context, page string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	pageID := page
	switch ref.Kind {
	case cli.RefName:
		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		pageID = resolved
	case cli.RefID:
		pageID = ref.ID
	}

	if err := client.ArchivePage(bgCtx, pageID); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Page archived")
	return nil
}

type PageDeleteCmd struct {
	Page string `arg:"" help:"Page URL, name, or ID"`
}

func (c *PageDeleteCmd) Run(ctx *Context) error {
	return runPageDelete(ctx, c.Page)
}

func runPageDelete(ctx *Context, page string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	ref := cli.ParsePageRef(page)
	pageID := page
	switch ref.Kind {
	case cli.RefName:
		resolved, err := cli.ResolvePageID(bgCtx, client, page)
		if err != nil {
			output.PrintError(err)
			return err
		}
		pageID = resolved
	case cli.RefID:
		pageID = ref.ID
	}

	if err := client.DeletePage(bgCtx, pageID); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Page moved to trash")
	return nil
}

type PageSyncCmd struct {
	File         string   `arg:"" help:"Markdown file to sync" type:"existingfile"`
	Title        string   `help:"Page title (default: filename or first heading)" short:"t"`
	Parent       string   `help:"Parent page URL, name, or ID" short:"p"`
	ParentDB     string   `help:"Parent database URL, name, or ID" name:"parent-db" short:"d"`
	Icon         string   `help:"Page icon (emoji, https URL, or 'none' to clear)" short:"i"`
	JSON         bool     `help:"Output as JSON" short:"j"`
	AssetBaseURL string   `help:"Base URL used to rewrite local image embeds (or NOTION_CLI_ASSET_BASE_URL)"`
	AssetRoot    string   `help:"Local asset root mapped to --asset-base-url (or NOTION_CLI_ASSET_ROOT)"`
	PropertyMode string   `help:"Property sync mode: off, warn, or strict" enum:"off,warn,strict" default:"warn" name:"property-mode"`
	Props        []string `help:"Semicolon-delimited properties (key=value;key2=value2). Repeatable." name:"props"`
	Prop         []string `help:"Single property assignment key=value. Repeatable." name:"prop"`
}

func (c *PageSyncCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runPageSync(ctx, c.File, c.Title, c.Parent, c.ParentDB, c.Icon, c.AssetBaseURL, c.AssetRoot, c.PropertyMode, c.Props, c.Prop)
}

func runPageSync(ctx *Context, file, title, parent, parentDB, icon, assetBaseURL, assetRoot, propertyModeRaw string, propsFlags, propFlags []string) error {
	explicitIcon, parsedIcon, err := parseExplicitIcon(icon)
	if err != nil {
		output.PrintError(err)
		return err
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		output.PrintError(err)
		return err
	}

	content := string(raw)
	fm, body := cli.ParseFrontmatter(content)
	body, rewrittenCount, err := rewriteLocalImages(file, body, assetBaseURL, assetRoot)
	if err != nil {
		output.PrintError(err)
		return err
	}
	if rewrittenCount > 0 {
		output.PrintInfo(fmt.Sprintf("Rewrote %d local image(s) to hosted URLs", rewrittenCount))
	}

	if title == "" {
		title = extractTitleFromMarkdown(body)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}
	if icon == "" {
		icon, title = extractEmojiFromTitle(title)
	}

	propertyMode, err := cli.ParsePropertyMode(propertyModeRaw)
	if err != nil {
		output.PrintError(err)
		return err
	}

	frontmatterProperties := map[string]any{}
	if propertyMode != cli.PropertyModeOff {
		frontmatterProperties, err = cli.ParseFrontmatterProperties(content)
		if err != nil {
			if propertyMode == cli.PropertyModeStrict {
				output.PrintError(err)
				return err
			}
			output.PrintWarning(err.Error())
			frontmatterProperties = map[string]any{}
		}
	}

	flagProperties := map[string]any{}
	if propertyMode != cli.PropertyModeOff {
		var parseErrs []error
		flagProperties, parseErrs = cli.ParsePropertiesFlags(propsFlags, propFlags)
		if err := handlePropertyParseErrors(propertyMode, parseErrs); err != nil {
			output.PrintError(err)
			return err
		}
	}

	properties := cli.MergeProperties(frontmatterProperties, flagProperties)
	if propertyMode == cli.PropertyModeOff {
		properties = nil
	}

	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	if fm.NotionID != "" {
		if len(properties) > 0 {
			propReq := mcp.UpdatePageRequest{
				PageID:     fm.NotionID,
				Command:    "update_properties",
				Properties: properties,
			}
			if err := client.UpdatePage(bgCtx, propReq); err != nil {
				if propertyMode == cli.PropertyModeStrict {
					output.PrintError(err)
					return err
				}
				output.PrintWarning("Property update failed; continuing content sync due --property-mode=warn: " + err.Error())
			}
		}

		req := mcp.UpdatePageRequest{
			PageID:     fm.NotionID,
			Command:    "replace_content",
			NewContent: body,
		}
		if err := client.UpdatePage(bgCtx, req); err != nil {
			output.PrintError(err)
			return err
		}

		if explicitIcon {
			if err := setPageIcon(bgCtx, fm.NotionID, parsedIcon); err != nil {
				output.PrintError(err)
				return err
			}
		}
		displayTitle := titleWithIcon(title, icon)

		if ctx.JSON {
			outPage := output.Page{
				ID:    fm.NotionID,
				Title: displayTitle,
				Icon:  outputIconValue(icon, explicitIcon, parsedIcon),
			}
			return output.PrintPage(outPage, true)
		}

		output.PrintSuccess("Synced: " + displayTitle)
		return nil
	}

	req := mcp.CreatePageRequest{
		Title:      title,
		Content:    body,
		Properties: properties,
	}

	if parentDB != "" {
		dbID, err := cli.ResolveDatabaseID(bgCtx, client, parentDB)
		if err != nil {
			output.PrintError(err)
			return err
		}
		dbID, err = client.ResolveDataSourceID(bgCtx, dbID)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentDatabaseID = dbID
	} else if parent != "" {
		parentID, err := cli.ResolvePageID(bgCtx, client, parent)
		if err != nil {
			output.PrintError(err)
			return err
		}
		req.ParentPageID = parentID
	}

	resp, err := client.CreatePage(bgCtx, req)
	if err != nil {
		if len(properties) > 0 && propertyMode == cli.PropertyModeWarn {
			output.PrintWarning("Page creation with properties failed; retrying without properties: " + err.Error())
			req.Properties = nil
			resp, err = client.CreatePage(bgCtx, req)
		}
		if err != nil {
			output.PrintError(err)
			return err
		}
	}

	pageID := resp.ID
	if pageID == "" && resp.URL != "" {
		pageID, _ = cli.ExtractNotionUUID(resp.URL)
	}

	if explicitIcon {
		if pageID == "" {
			output.PrintWarning("Page created but could not retrieve ID to apply icon")
		} else if err := setPageIcon(bgCtx, pageID, parsedIcon); err != nil {
			output.PrintError(err)
			return err
		}
	}
	if pageID == "" {
		output.PrintWarning("Page created but could not retrieve ID for frontmatter")
	} else {
		updated := cli.SetFrontmatterID(content, pageID)
		fileMode := os.FileMode(0o644)
		if info, err := os.Stat(file); err == nil {
			fileMode = info.Mode()
		}
		if err := os.WriteFile(file, []byte(updated), fileMode); err != nil {
			output.PrintError(fmt.Errorf("page created but failed to update frontmatter: %w", err))
			return err
		}
	}

	displayTitle := titleWithIcon(title, icon)

	if ctx.JSON {
		outPage := output.Page{
			ID:    pageID,
			URL:   resp.URL,
			Title: displayTitle,
			Icon:  outputIconValue(icon, explicitIcon, parsedIcon),
		}
		return output.PrintPage(outPage, true)
	}

	output.PrintSuccess("Created: " + displayTitle)
	if resp.URL != "" {
		output.PrintInfo(resp.URL)
	}
	return nil
}

func rewriteLocalImages(sourceFile, markdown, flagBaseURL, flagAssetRoot string) (string, int, error) {
	assetBaseURL := strings.TrimSpace(flagBaseURL)
	if assetBaseURL == "" {
		assetBaseURL = strings.TrimSpace(os.Getenv("NOTION_CLI_ASSET_BASE_URL"))
	}
	assetRoot := strings.TrimSpace(flagAssetRoot)
	if assetRoot == "" {
		assetRoot = strings.TrimSpace(os.Getenv("NOTION_CLI_ASSET_ROOT"))
	}

	rewritten, rewrites, err := cli.RewriteLocalMarkdownImages(markdown, cli.MarkdownImageRewriteOptions{
		SourceFile:   sourceFile,
		AssetBaseURL: assetBaseURL,
		AssetRoot:    assetRoot,
	})
	if err != nil {
		return "", 0, err
	}
	return rewritten, len(rewrites), nil
}

func handlePropertyParseErrors(mode cli.PropertyMode, errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if mode == cli.PropertyModeStrict {
		return fmt.Errorf("property parsing failed: %w", errs[0])
	}
	for _, err := range errs {
		output.PrintWarning(err.Error())
	}
	return nil
}

func parseExplicitIcon(raw string) (bool, api.PageIcon, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, api.PageIcon{}, nil
	}

	icon, err := api.ParsePageIcon(raw)
	if err != nil {
		return false, api.PageIcon{}, &output.UserError{Message: err.Error()}
	}

	return true, icon, nil
}

func setPageIcon(ctx context.Context, pageID string, icon api.PageIcon) error {
	client, err := cli.RequireOfficialAPIClient()
	if err != nil {
		return err
	}
	return client.SetPageIcon(ctx, pageID, icon)
}

func pageIDFromCreateResponse(resp *mcp.CreatePageResponse) string {
	if resp == nil {
		return ""
	}
	if resp.ID != "" {
		return resp.ID
	}
	if resp.URL != "" {
		if id, ok := cli.ExtractNotionUUID(resp.URL); ok {
			return id
		}
	}
	return ""
}

func outputIconValue(rawIcon string, explicit bool, parsed api.PageIcon) string {
	if !explicit {
		return rawIcon
	}
	switch {
	case parsed.Clear:
		return ""
	case parsed.Emoji != "":
		return parsed.Emoji
	case parsed.ExternalURL != "":
		return parsed.ExternalURL
	default:
		return rawIcon
	}
}

func titleWithIcon(title, icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return title
	}

	runes := []rune(icon)
	if len(runes) == 0 {
		return title
	}
	if !cli.IsEmoji(runes[0]) {
		return title
	}
	return icon + " " + title
}

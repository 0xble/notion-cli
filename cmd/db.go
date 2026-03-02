package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/lox/notion-cli/internal/cli"
	appconfig "github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
	"github.com/lox/notion-cli/internal/privateapi"
)

type DBCmd struct {
	List  DBListCmd  `cmd:"" help:"List databases"`
	Query DBQueryCmd `cmd:"" help:"Query a database"`
	View  DBViewCmd  `cmd:"" help:"Manage database views"`
}

type DBListCmd struct {
	Query string `help:"Filter databases by name" short:"q"`
	Limit int    `help:"Maximum number of results" short:"l" default:"20"`
	JSON  bool   `help:"Output as JSON" short:"j"`
}

func (c *DBListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBList(ctx, c.Query, c.Limit)
}

func runDBList(ctx *Context, query string, limit int) error {
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

	dbs := filterDatabases(resp.Results, limit)
	return output.PrintDatabases(dbs, ctx.JSON)
}

func filterDatabases(results []mcp.SearchResult, limit int) []output.Database {
	dbs := make([]output.Database, 0)
	for _, r := range results {
		if r.ObjectType != "database" && r.Object != "database" {
			continue
		}
		if limit > 0 && len(dbs) >= limit {
			break
		}
		dbs = append(dbs, output.Database{
			ID:    r.ID,
			Title: r.Title,
			URL:   r.URL,
		})
	}
	return dbs
}

type DBQueryCmd struct {
	ID   string `arg:"" help:"Database URL or ID"`
	JSON bool   `help:"Output as JSON" short:"j"`
}

func (c *DBQueryCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBQuery(ctx, c.ID)
}

func runDBQuery(ctx *Context, id string) error {
	client, err := cli.RequireClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	bgCtx := context.Background()

	// If the ID contains ?v=, it's a view URL — use the dedicated query tool
	if strings.Contains(id, "?v=") {
		result, err := client.QueryDatabaseView(bgCtx, id)
		if err != nil {
			output.PrintError(err)
			return err
		}
		if result == "" {
			output.PrintWarning("No content found")
			return nil
		}
		return output.RenderMarkdown(result)
	}

	result, err := client.Fetch(bgCtx, id)
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

type DBViewCmd struct {
	List   DBViewListCmd   `cmd:"" help:"List database views"`
	Create DBViewCreateCmd `cmd:"" help:"Create a database view"`
	Update DBViewUpdateCmd `cmd:"" help:"Update a database view"`
}

type DBViewListCmd struct {
	Database string `arg:"" help:"Database URL, name, or ID"`
	JSON     bool   `help:"Output as JSON" short:"j"`
}

func (c *DBViewListCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBViewList(ctx, c.Database)
}

func runDBViewList(ctx *Context, dbRef string) error {
	mcpClient, webClient, dbID, err := resolveDBAndPrivateClient(ctx, dbRef)
	if err != nil {
		return err
	}
	defer func() { _ = mcpClient.Close() }()

	bgCtx := context.Background()
	views, err := webClient.ListDatabaseViews(bgCtx, dbID)
	if err != nil {
		output.PrintError(err)
		return err
	}
	outViews := toOutputViews(views)

	if ctx.JSON {
		return output.PrintViews(outViews, true)
	}
	return output.PrintViews(outViews, false)
}

type DBViewCreateCmd struct {
	Database string `arg:"" help:"Database URL, name, or ID"`
	Name     string `help:"View name" short:"n" required:""`
	Layout   string `help:"View layout" default:"table" enum:"table,board,timeline,calendar,list,gallery,chart,feed,map"`

	ShowPageIcon   bool   `help:"Show page icon on cards" name:"show-page-icon"`
	NoShowPageIcon bool   `help:"Hide page icon on cards" name:"no-show-page-icon"`
	WrapContent    bool   `help:"Wrap all content" name:"wrap-content"`
	NoWrapContent  bool   `help:"Disable content wrapping" name:"no-wrap-content"`
	OpenPagesIn    string `help:"How to open pages" name:"open-pages-in"`
	CardPreview    string `help:"Card preview mode" name:"card-preview"`
	CardSize       string `help:"Card size" name:"card-size"`
	CardLayout     string `help:"Card layout" name:"card-layout"`
	JSON           bool   `help:"Output as JSON" short:"j"`
}

func (c *DBViewCreateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBViewCreate(ctx, c)
}

func runDBViewCreate(ctx *Context, cmd *DBViewCreateCmd) error {
	if !isValidLayout(cmd.Layout) {
		err := &output.UserError{Message: "invalid --layout. Supported values: table, board, timeline, calendar, list, gallery, chart, feed, map"}
		output.PrintError(err)
		return err
	}

	mcpClient, webClient, dbID, err := resolveDBAndPrivateClient(ctx, cmd.Database)
	if err != nil {
		return err
	}
	defer func() { _ = mcpClient.Close() }()

	opts, err := buildViewOptions(viewOptionInputs{
		ShowPageIcon:   cmd.ShowPageIcon,
		NoShowPageIcon: cmd.NoShowPageIcon,
		WrapContent:    cmd.WrapContent,
		NoWrapContent:  cmd.NoWrapContent,
		OpenPagesIn:    cmd.OpenPagesIn,
		CardPreview:    cmd.CardPreview,
		CardSize:       cmd.CardSize,
		CardLayout:     cmd.CardLayout,
	})
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	view, err := webClient.CreateDatabaseView(bgCtx, privateapi.ViewCreateRequest{
		DatabaseID: dbID,
		Name:       cmd.Name,
		Layout:     cmd.Layout,
		Options:    opts,
	})
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		return output.PrintView(toOutputView(view), true)
	}
	output.PrintSuccess("View created")
	output.PrintInfo(view.URL)
	return nil
}

type DBViewUpdateCmd struct {
	Database string `arg:"" help:"Database URL, name, or ID"`
	View     string `arg:"" help:"View URL, view:// ID, name, or ID"`
	Name     string `help:"New view name" short:"n"`
	Layout   string `help:"New view layout"`

	ShowPageIcon   bool   `help:"Show page icon on cards" name:"show-page-icon"`
	NoShowPageIcon bool   `help:"Hide page icon on cards" name:"no-show-page-icon"`
	WrapContent    bool   `help:"Wrap all content" name:"wrap-content"`
	NoWrapContent  bool   `help:"Disable content wrapping" name:"no-wrap-content"`
	OpenPagesIn    string `help:"How to open pages" name:"open-pages-in"`
	CardPreview    string `help:"Card preview mode" name:"card-preview"`
	CardSize       string `help:"Card size" name:"card-size"`
	CardLayout     string `help:"Card layout" name:"card-layout"`
	JSON           bool   `help:"Output as JSON" short:"j"`
}

func (c *DBViewUpdateCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runDBViewUpdate(ctx, c)
}

func runDBViewUpdate(ctx *Context, cmd *DBViewUpdateCmd) error {
	mcpClient, webClient, dbID, err := resolveDBAndPrivateClient(ctx, cmd.Database)
	if err != nil {
		return err
	}
	defer func() { _ = mcpClient.Close() }()

	bgCtx := context.Background()

	viewID, err := resolveViewID(bgCtx, webClient, dbID, cmd.View)
	if err != nil {
		output.PrintError(err)
		return err
	}

	opts, err := buildViewOptions(viewOptionInputs{
		ShowPageIcon:   cmd.ShowPageIcon,
		NoShowPageIcon: cmd.NoShowPageIcon,
		WrapContent:    cmd.WrapContent,
		NoWrapContent:  cmd.NoWrapContent,
		OpenPagesIn:    cmd.OpenPagesIn,
		CardPreview:    cmd.CardPreview,
		CardSize:       cmd.CardSize,
		CardLayout:     cmd.CardLayout,
	})
	if err != nil {
		output.PrintError(err)
		return err
	}

	updateReq := privateapi.ViewUpdateRequest{
		DatabaseID: dbID,
		ViewID:     viewID,
		Options:    opts,
	}
	if strings.TrimSpace(cmd.Name) != "" {
		name := strings.TrimSpace(cmd.Name)
		updateReq.Name = &name
	}
	if strings.TrimSpace(cmd.Layout) != "" {
		layout := strings.TrimSpace(cmd.Layout)
		if !isValidLayout(layout) {
			err := &output.UserError{Message: "invalid --layout. Supported values: table, board, timeline, calendar, list, gallery, chart, feed, map"}
			output.PrintError(err)
			return err
		}
		updateReq.Layout = &layout
	}

	view, err := webClient.UpdateDatabaseView(bgCtx, updateReq)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		return output.PrintView(toOutputView(view), true)
	}
	output.PrintSuccess("View updated")
	output.PrintInfo(view.URL)
	return nil
}

func resolveDBAndPrivateClient(ctx *Context, dbRef string) (*mcp.Client, *privateapi.Client, string, error) {
	if !ctx.Config.PrivateAPI.Enabled {
		path, _ := appconfig.Path()
		return nil, nil, "", &output.UserError{
			Message: fmt.Sprintf("private API is disabled. Enable it in %s with private_api.enabled = true", path),
		}
	}

	mcpClient, err := cli.RequireClient()
	if err != nil {
		return nil, nil, "", err
	}

	bgCtx := context.Background()
	dbID, err := cli.ResolveDatabaseID(bgCtx, mcpClient, dbRef)
	if err != nil {
		_ = mcpClient.Close()
		output.PrintError(err)
		return nil, nil, "", err
	}

	webClient, err := privateapi.NewClient(ctx.Config.PrivateAPI)
	if err != nil {
		_ = mcpClient.Close()
		return nil, nil, "", &output.UserError{
			Message: err.Error() + " (configure in ~/.config/notion-cli/config.json under private_api)",
		}
	}

	return mcpClient, webClient, dbID, nil
}

func resolveViewID(ctx context.Context, webClient *privateapi.Client, databaseID, ref string) (string, error) {
	if id, ok := extractViewID(ref); ok {
		return id, nil
	}
	return webClient.ResolveViewIDByName(ctx, databaseID, ref)
}

func extractViewID(ref string) (string, bool) {
	if strings.HasPrefix(ref, "view://") {
		id := strings.TrimPrefix(ref, "view://")
		if out, ok := cli.ExtractNotionUUID(id); ok {
			return out, true
		}
	}

	if strings.Contains(ref, "?v=") {
		u, err := url.Parse(ref)
		if err == nil {
			v := u.Query().Get("v")
			if out, ok := cli.ExtractNotionUUID(v); ok {
				return out, true
			}
		}
		idx := strings.Index(ref, "?v=")
		if idx >= 0 {
			v := ref[idx+3:]
			if cut := strings.IndexByte(v, '&'); cut >= 0 {
				v = v[:cut]
			}
			if out, ok := cli.ExtractNotionUUID(v); ok {
				return out, true
			}
		}
	}

	if out, ok := cli.ExtractNotionUUID(ref); ok {
		return out, true
	}
	return "", false
}

type viewOptionInputs struct {
	ShowPageIcon   bool
	NoShowPageIcon bool
	WrapContent    bool
	NoWrapContent  bool
	OpenPagesIn    string
	CardPreview    string
	CardSize       string
	CardLayout     string
}

func buildViewOptions(in viewOptionInputs) (privateapi.ViewOptions, error) {
	var out privateapi.ViewOptions

	if in.ShowPageIcon && in.NoShowPageIcon {
		return out, &output.UserError{Message: "use only one of --show-page-icon or --no-show-page-icon"}
	}
	if in.ShowPageIcon {
		v := true
		out.ShowPageIcon = &v
	} else if in.NoShowPageIcon {
		v := false
		out.ShowPageIcon = &v
	}

	if in.WrapContent && in.NoWrapContent {
		return out, &output.UserError{Message: "use only one of --wrap-content or --no-wrap-content"}
	}
	if in.WrapContent {
		v := true
		out.WrapContent = &v
	} else if in.NoWrapContent {
		v := false
		out.WrapContent = &v
	}

	if v := strings.TrimSpace(in.OpenPagesIn); v != "" {
		if !inSet(v, "center-peek", "side-peek", "full-page") {
			return out, &output.UserError{Message: "invalid --open-pages-in. Supported values: center-peek, side-peek, full-page"}
		}
		out.OpenPagesIn = &v
	}
	if v := strings.TrimSpace(in.CardPreview); v != "" {
		if !inSet(v, "none", "page-content", "page-cover", "files-media") {
			return out, &output.UserError{Message: "invalid --card-preview. Supported values: none, page-content, page-cover, files-media"}
		}
		out.CardPreview = &v
	}
	if v := strings.TrimSpace(in.CardSize); v != "" {
		if !inSet(v, "small", "medium", "large") {
			return out, &output.UserError{Message: "invalid --card-size. Supported values: small, medium, large"}
		}
		out.CardSize = &v
	}
	if v := strings.TrimSpace(in.CardLayout); v != "" {
		if !inSet(v, "compact", "list") {
			return out, &output.UserError{Message: "invalid --card-layout. Supported values: compact, list"}
		}
		out.CardLayout = &v
	}

	return out, nil
}

func toOutputView(v privateapi.View) output.View {
	return output.View{
		ID:     v.ID,
		Name:   v.Name,
		Layout: v.Layout,
		URL:    v.URL,
		Format: v.Format,
	}
}

func toOutputViews(views []privateapi.View) []output.View {
	out := make([]output.View, 0, len(views))
	for _, v := range views {
		out = append(out, toOutputView(v))
	}
	return out
}

func inSet(value string, allowed ...string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}

func isValidLayout(layout string) bool {
	return inSet(layout, "table", "board", "timeline", "calendar", "list", "gallery", "chart", "feed", "map")
}

package privateapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lox/notion-cli/internal/config"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	cfg        config.PrivateAPIConfig
}

type View struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Layout string         `json:"layout"`
	URL    string         `json:"url"`
	Format map[string]any `json:"format,omitempty"`
}

type DatabaseInfo struct {
	BlockID      string   `json:"block_id"`
	CollectionID string   `json:"collection_id"`
	SpaceID      string   `json:"space_id"`
	ViewIDs      []string `json:"view_ids"`
}

type ViewOptions struct {
	ShowPageIcon *bool
	WrapContent  *bool
	OpenPagesIn  *string
	CardPreview  *string
	CardSize     *string
	CardLayout   *string
}

type ViewCreateRequest struct {
	DatabaseID string
	Name       string
	Layout     string
	Options    ViewOptions
}

type ViewUpdateRequest struct {
	DatabaseID string
	ViewID     string
	Name       *string
	Layout     *string
	Options    ViewOptions
}

type ViewDeleteRequest struct {
	DatabaseID string
	ViewID     string
}

func NewClient(cfg config.PrivateAPIConfig) (*Client, error) {
	missing := missingAuthFields(cfg)
	if len(missing) > 0 {
		return nil, fmt.Errorf("private_api auth missing required fields: %s", strings.Join(missing, ", "))
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://www.notion.so"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    baseURL,
		cfg:        cfg,
	}, nil
}

func missingAuthFields(cfg config.PrivateAPIConfig) []string {
	var missing []string
	if strings.TrimSpace(cfg.TokenV2) == "" {
		missing = append(missing, "token_v2")
	}
	if strings.TrimSpace(cfg.NotionUserID) == "" {
		missing = append(missing, "notion_user_id")
	}
	return missing
}

func (c *Client) GetDatabaseInfo(ctx context.Context, databaseID string) (DatabaseInfo, error) {
	val, err := c.getRecordValue(ctx, "block", databaseID, "")
	if err != nil {
		return DatabaseInfo{}, err
	}

	info := DatabaseInfo{
		BlockID:      databaseID,
		CollectionID: stringValue(val["collection_id"]),
		SpaceID:      stringValue(val["space_id"]),
		ViewIDs:      toStringSlice(val["view_ids"]),
	}
	if info.CollectionID == "" {
		return DatabaseInfo{}, fmt.Errorf("database %s has no collection_id", databaseID)
	}
	return info, nil
}

func (c *Client) ListDatabaseViews(ctx context.Context, databaseID string) ([]View, error) {
	info, err := c.GetDatabaseInfo(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	if len(info.ViewIDs) == 0 {
		return []View{}, nil
	}

	reqs := make([]recordPointer, 0, len(info.ViewIDs))
	for _, id := range info.ViewIDs {
		reqs = append(reqs, recordPointer{Table: "collection_view", ID: id})
	}

	results, err := c.getRecordValues(ctx, reqs, info.SpaceID)
	if err != nil {
		return nil, err
	}

	views := make([]View, 0, len(results))
	for _, rv := range results {
		if rv.Value == nil {
			continue
		}
		viewID := stringValue(rv.Value["id"])
		if viewID == "" {
			viewID = rv.ID
		}
		format := toMap(rv.Value["format"])
		views = append(views, View{
			ID:     viewID,
			Name:   stringValue(rv.Value["name"]),
			Layout: stringValue(rv.Value["type"]),
			URL:    c.viewURL(databaseID, viewID),
			Format: format,
		})
	}
	return views, nil
}

func (c *Client) ResolveViewIDByName(ctx context.Context, databaseID, name string) (string, error) {
	views, err := c.ListDatabaseViews(ctx, databaseID)
	if err != nil {
		return "", err
	}

	var matches []View
	for _, v := range views {
		if strings.EqualFold(v.Name, name) {
			matches = append(matches, v)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous view name %q: %d matches", name, len(matches))
	}
	return "", fmt.Errorf("view not found by name: %s", name)
}

func (c *Client) CreateDatabaseView(ctx context.Context, req ViewCreateRequest) (View, error) {
	info, err := c.GetDatabaseInfo(ctx, req.DatabaseID)
	if err != nil {
		return View{}, err
	}

	now := time.Now().UnixMilli()
	viewID := uuid.NewString()
	var collectionSchema map[string]any
	if req.Layout == "chart" {
		collectionSchema, err = c.getCollectionSchema(ctx, info.CollectionID, info.SpaceID)
		if err != nil {
			return View{}, err
		}
	}
	format := freshFormatForLayout(req.Layout, info.CollectionID, info.SpaceID, collectionSchema)
	applyViewOptions(format, req.Options, req.Layout)

	viewRecord := map[string]any{
		"id":               viewID,
		"version":          1,
		"name":             req.Name,
		"type":             req.Layout,
		"format":           format,
		"query2":           defaultQuery2(),
		"parent_id":        req.DatabaseID,
		"parent_table":     "block",
		"space_id":         info.SpaceID,
		"alive":            true,
		"created_time":     now,
		"last_edited_time": now,
	}

	newViewIDs := append([]string{}, info.ViewIDs...)
	newViewIDs = append(newViewIDs, viewID)

	ops := []operation{
		{
			ID:      viewID,
			Table:   "collection_view",
			Path:    []string{},
			Command: "set",
			Args:    viewRecord,
		},
		{
			ID:      req.DatabaseID,
			Table:   "block",
			Path:    []string{"view_ids"},
			Command: "set",
			Args:    newViewIDs,
		},
		{
			ID:      req.DatabaseID,
			Table:   "block",
			Path:    []string{"last_edited_time"},
			Command: "set",
			Args:    now,
		},
	}
	if err := c.submitTransaction(ctx, submitTransactionRequest{Operations: ops}, info.SpaceID); err != nil {
		return View{}, err
	}

	return View{
		ID:     viewID,
		Name:   req.Name,
		Layout: req.Layout,
		URL:    c.viewURL(req.DatabaseID, viewID),
		Format: format,
	}, nil
}

func (c *Client) UpdateDatabaseView(ctx context.Context, req ViewUpdateRequest) (View, error) {
	info, err := c.GetDatabaseInfo(ctx, req.DatabaseID)
	if err != nil {
		return View{}, err
	}

	viewVal, err := c.getRecordValue(ctx, "collection_view", req.ViewID, info.SpaceID)
	if err != nil {
		return View{}, err
	}

	originalLayout := stringValue(viewVal["type"])
	targetLayout := originalLayout
	layoutChanged := false
	if req.Layout != nil && *req.Layout != "" {
		targetLayout = *req.Layout
		layoutChanged = targetLayout != originalLayout
	}

	format := toMap(viewVal["format"])
	if layoutChanged {
		// Reinitialize format on layout changes so Notion can apply layout defaults.
		// Keeping stale layout-specific keys can fail validation (for example chart).
		var collectionSchema map[string]any
		if targetLayout == "chart" {
			collectionSchema, err = c.getCollectionSchema(ctx, info.CollectionID, info.SpaceID)
			if err != nil {
				return View{}, err
			}
		}
		format = freshFormatForLayout(targetLayout, info.CollectionID, info.SpaceID, collectionSchema)
	} else {
		ensureCollectionPointer(format, info.CollectionID, info.SpaceID)
	}
	applyViewOptions(format, req.Options, targetLayout)

	now := time.Now().UnixMilli()
	ops := []operation{
		{
			ID:      req.ViewID,
			Table:   "collection_view",
			Path:    []string{"format"},
			Command: "set",
			Args:    format,
		},
		{
			ID:      req.ViewID,
			Table:   "collection_view",
			Path:    []string{"last_edited_time"},
			Command: "set",
			Args:    now,
		},
	}
	name := stringValue(viewVal["name"])
	if req.Name != nil {
		name = *req.Name
		ops = append(ops, operation{
			ID:      req.ViewID,
			Table:   "collection_view",
			Path:    []string{"name"},
			Command: "set",
			Args:    *req.Name,
		})
	}
	if req.Layout != nil && *req.Layout != "" {
		ops = append(ops, operation{
			ID:      req.ViewID,
			Table:   "collection_view",
			Path:    []string{"type"},
			Command: "set",
			Args:    *req.Layout,
		})
	}

	if err := c.submitTransaction(ctx, submitTransactionRequest{Operations: ops}, info.SpaceID); err != nil {
		return View{}, err
	}

	return View{
		ID:     req.ViewID,
		Name:   name,
		Layout: targetLayout,
		URL:    c.viewURL(req.DatabaseID, req.ViewID),
		Format: format,
	}, nil
}

func (c *Client) DeleteDatabaseView(ctx context.Context, req ViewDeleteRequest) (View, error) {
	req.DatabaseID = strings.TrimSpace(req.DatabaseID)
	req.ViewID = strings.TrimSpace(req.ViewID)
	if req.DatabaseID == "" {
		return View{}, fmt.Errorf("database ID is required")
	}
	if req.ViewID == "" {
		return View{}, fmt.Errorf("view ID is required")
	}

	info, err := c.GetDatabaseInfo(ctx, req.DatabaseID)
	if err != nil {
		return View{}, err
	}

	if len(info.ViewIDs) == 0 {
		return View{}, fmt.Errorf("database %s has no views", req.DatabaseID)
	}

	reqs := make([]recordPointer, 0, len(info.ViewIDs))
	for _, id := range info.ViewIDs {
		reqs = append(reqs, recordPointer{Table: "collection_view", ID: id})
	}
	results, err := c.getRecordValues(ctx, reqs, info.SpaceID)
	if err != nil {
		return View{}, err
	}

	nextViewIDs := make([]string, 0, len(info.ViewIDs)-1)
	var deleted View
	found := false
	for _, rv := range results {
		viewID := rv.ID
		if rv.Value != nil {
			if id := stringValue(rv.Value["id"]); id != "" {
				viewID = id
			}
		}

		if viewID == req.ViewID {
			found = true
			if rv.Value != nil {
				deleted = View{
					ID:     req.ViewID,
					Name:   stringValue(rv.Value["name"]),
					Layout: stringValue(rv.Value["type"]),
					URL:    c.viewURL(req.DatabaseID, req.ViewID),
					Format: toMap(rv.Value["format"]),
				}
			} else {
				deleted = View{
					ID:  req.ViewID,
					URL: c.viewURL(req.DatabaseID, req.ViewID),
				}
			}
			continue
		}
		nextViewIDs = append(nextViewIDs, viewID)
	}

	if !found {
		return View{}, fmt.Errorf("view not found: %s", req.ViewID)
	}

	now := time.Now().UnixMilli()
	ops := []operation{
		{
			ID:      req.DatabaseID,
			Table:   "block",
			Path:    []string{"view_ids"},
			Command: "set",
			Args:    nextViewIDs,
		},
		{
			ID:      req.DatabaseID,
			Table:   "block",
			Path:    []string{"last_edited_time"},
			Command: "set",
			Args:    now,
		},
		{
			ID:      req.ViewID,
			Table:   "collection_view",
			Path:    []string{"alive"},
			Command: "set",
			Args:    false,
		},
		{
			ID:      req.ViewID,
			Table:   "collection_view",
			Path:    []string{"last_edited_time"},
			Command: "set",
			Args:    now,
		},
	}
	if err := c.submitTransaction(ctx, submitTransactionRequest{Operations: ops}, info.SpaceID); err != nil {
		return View{}, err
	}

	return deleted, nil
}

func defaultQuery2() map[string]any {
	return map[string]any{
		"filter": map[string]any{
			"operator": "and",
			"filters":  []any{},
		},
		"sort": []any{},
	}
}

func applyViewOptions(format map[string]any, options ViewOptions, layout string) {
	if format == nil {
		return
	}

	if options.ShowPageIcon != nil {
		format["show_page_icon"] = *options.ShowPageIcon
	}
	if options.WrapContent != nil {
		format["wrap_all_content"] = *options.WrapContent
		switch layout {
		case "table":
			format["table_wrap"] = *options.WrapContent
		case "list":
			format["list_wrap"] = *options.WrapContent
		}
	}
	if options.OpenPagesIn != nil {
		format["open_page_in"] = encodeOpenPagesIn(*options.OpenPagesIn)
	}
	if options.CardPreview != nil {
		format["card_preview"] = encodeCardPreview(*options.CardPreview)
	}
	if options.CardSize != nil {
		format["card_size"] = *options.CardSize
	}
	if options.CardLayout != nil {
		format["card_layout_mode"] = encodeCardLayout(*options.CardLayout)
	}
}

func freshFormatForLayout(layout, collectionID, spaceID string, collectionSchema map[string]any) map[string]any {
	format := map[string]any{}
	ensureCollectionPointer(format, collectionID, spaceID)
	if layout == "chart" {
		if cfg := defaultChartConfig(collectionSchema); len(cfg) > 0 {
			format["chart_config"] = cfg
		}
	}
	return format
}

func ensureCollectionPointer(format map[string]any, collectionID, spaceID string) {
	if format == nil {
		return
	}
	if ptr := toMap(format["collection_pointer"]); len(ptr) > 0 {
		return
	}
	format["collection_pointer"] = map[string]any{
		"id":      collectionID,
		"table":   "collection",
		"spaceId": spaceID,
	}
}

func defaultChartConfig(collectionSchema map[string]any) map[string]any {
	propertyID, propertyType := chooseDefaultChartProperty(collectionSchema)
	if propertyID == "" || propertyType == "" {
		return nil
	}

	groupBy := defaultChartGroupBy(propertyID, propertyType)
	if len(groupBy) == 0 {
		return nil
	}

	return map[string]any{
		"chartFormat": map[string]any{
			"axisHideEmptyGroups": false,
			"mainSort":            "x-ascending",
		},
		"type": "column",
		"dataConfig": map[string]any{
			"type":    "groups_reducer",
			"groupBy": groupBy,
			"aggregationConfig": map[string]any{
				"aggregation": map[string]any{
					"aggregator": "count",
				},
				"seriesFormat": map[string]any{
					"displayType": "column",
				},
			},
		},
	}
}

func chooseDefaultChartProperty(schema map[string]any) (string, string) {
	priority := map[string]int{
		"select":            0,
		"multi_select":      0,
		"status":            0,
		"date":              1,
		"number":            1,
		"person":            1,
		"relation":          2,
		"checkbox":          3,
		"created_time":      3,
		"created_by":        3,
		"last_edited_by":    3,
		"last_edited_time":  3,
		"last_visited_time": 3,
		"text":              3,
		"title":             3,
		"url":               4,
		"email":             4,
		"phone_number":      4,
		"formula":           5,
	}

	bestID := ""
	bestType := ""
	bestPriority := 1 << 30

	for propertyID, raw := range schema {
		property := toMap(raw)
		propertyType := stringValue(property["type"])
		p, ok := priority[propertyType]
		if !ok {
			continue
		}
		if p < bestPriority || (p == bestPriority && (bestID == "" || propertyID < bestID)) {
			bestID = propertyID
			bestType = propertyType
			bestPriority = p
		}
	}

	return bestID, bestType
}

func defaultChartGroupBy(propertyID, propertyType string) map[string]any {
	switch propertyType {
	case "date", "created_time", "last_edited_time", "last_visited_time":
		return map[string]any{
			"type":     propertyType,
			"property": propertyID,
			"groupBy":  "day",
			"sort": map[string]any{
				"type": "ascending",
			},
		}
	case "number":
		return map[string]any{
			"type":     "number",
			"property": propertyID,
			"groupBy": map[string]any{
				"type": "unique",
			},
			"start": -1,
			"end":   -1,
			"size":  -1,
			"sort": map[string]any{
				"type": "ascending",
			},
		}
	default:
		return map[string]any{
			"type":     propertyType,
			"property": propertyID,
			"sort": map[string]any{
				"type": "ascending",
			},
		}
	}
}

func encodeOpenPagesIn(v string) string {
	switch v {
	case "center-peek":
		return "center_peek"
	case "side-peek":
		return "side_peek"
	case "full-page":
		return "full_page"
	default:
		return v
	}
}

func encodeCardPreview(v string) string {
	switch v {
	case "page-content":
		return "page_content"
	case "page-cover":
		return "page_cover"
	case "files-media":
		return "files_and_media"
	default:
		return v
	}
}

func encodeCardLayout(v string) string {
	switch v {
	case "compact":
		return "default"
	default:
		return v
	}
}

func (c *Client) getRecordValue(ctx context.Context, table, id, spaceID string) (map[string]any, error) {
	results, err := c.getRecordValues(ctx, []recordPointer{{Table: table, ID: id}}, spaceID)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 || results[0].Value == nil {
		return nil, fmt.Errorf("%s %s not found", table, id)
	}
	return results[0].Value, nil
}

func (c *Client) getCollectionSchema(ctx context.Context, collectionID, spaceID string) (map[string]any, error) {
	val, err := c.getRecordValue(ctx, "collection", collectionID, spaceID)
	if err != nil {
		return nil, err
	}
	return toMap(val["schema"]), nil
}

func (c *Client) getRecordValues(ctx context.Context, reqs []recordPointer, spaceID string) ([]recordValue, error) {
	body := getRecordValuesRequest{Requests: reqs}
	var out getRecordValuesResponse
	if err := c.doJSON(ctx, "/api/v3/getRecordValues", body, &out, spaceID); err != nil {
		return nil, err
	}
	return out.Results, nil
}

func (c *Client) submitTransaction(ctx context.Context, req submitTransactionRequest, spaceID string) error {
	var out map[string]any
	if err := c.doJSON(ctx, "/api/v3/submitTransaction", req, &out, spaceID); err != nil {
		return err
	}
	if v, ok := out["error"]; ok && v != nil {
		return fmt.Errorf("private api transaction failed: %v", v)
	}
	return nil
}

func (c *Client) doJSON(ctx context.Context, path string, payload any, out any, spaceID string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", c.baseURL)
	req.Header.Set("referer", c.baseURL+"/")
	req.Header.Set("x-requested-with", "XMLHttpRequest")

	activeUser := strings.TrimSpace(c.cfg.ActiveUserID)
	if activeUser == "" {
		activeUser = strings.TrimSpace(c.cfg.NotionUserID)
	}
	if activeUser != "" {
		req.Header.Set("x-notion-active-user-header", activeUser)
	}

	space := strings.TrimSpace(spaceID)
	if space == "" {
		space = strings.TrimSpace(c.cfg.SpaceID)
	}
	if space != "" {
		req.Header.Set("x-notion-space-id", space)
	}
	if csrf := strings.TrimSpace(c.cfg.CSRF); csrf != "" {
		req.Header.Set("x-notion-csrf-token", csrf)
	}
	if ua := strings.TrimSpace(c.cfg.UserAgent); ua != "" {
		req.Header.Set("user-agent", ua)
	}
	req.Header.Set("cookie", c.cookieHeader())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("private api %s failed (%d): %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse private api response for %s: %w", path, err)
	}
	return nil
}

func (c *Client) cookieHeader() string {
	pairs := []string{
		cookiePair("token_v2", c.cfg.TokenV2),
		cookiePair("notion_user_id", c.cfg.NotionUserID),
		cookiePair("notion_users", c.cfg.NotionUsers),
		cookiePair("device_id", c.cfg.DeviceID),
		cookiePair("csrf", c.cfg.CSRF),
	}
	var out []string
	for _, p := range pairs {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}

func cookiePair(name, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return name + "=" + value
}

func (c *Client) viewURL(databaseID, viewID string) string {
	return fmt.Sprintf("%s/%s?v=%s", c.baseURL, strings.ReplaceAll(databaseID, "-", ""), strings.ReplaceAll(viewID, "-", ""))
}

func toMap(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		out[k] = val
	}
	return out
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func toStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

type recordPointer struct {
	Table string `json:"table"`
	ID    string `json:"id"`
}

type getRecordValuesRequest struct {
	Requests []recordPointer `json:"requests"`
}

type recordValue struct {
	ID    string         `json:"id"`
	Table string         `json:"table"`
	Value map[string]any `json:"value"`
}

type getRecordValuesResponse struct {
	Results []recordValue `json:"results"`
}

type operation struct {
	ID      string   `json:"id"`
	Table   string   `json:"table"`
	Path    []string `json:"path"`
	Command string   `json:"command"`
	Args    any      `json:"args"`
}

type submitTransactionRequest struct {
	Operations []operation `json:"operations"`
}

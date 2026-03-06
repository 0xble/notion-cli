package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/lox/notion-cli/internal/config"
)

const (
	defaultBaseURL      = "https://api.notion.com/v1"
	defaultNotionAPIRev = "2022-06-28"
	fileUploadAPIRev    = "2025-09-03"
)

type Client struct {
	httpClient    *http.Client
	baseURL       string
	notionVersion string
	token         string
}

type PageIcon struct {
	Emoji       string
	ExternalURL string
	Clear       bool
}

type FileUpload struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UploadedImageBlock struct {
	FileUploadID string
	Caption      string
}

type PagePropertyMeta struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func NewClient(cfg config.APIConfig, token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("official API token is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	notionVersion := strings.TrimSpace(cfg.NotionVersion)
	if notionVersion == "" {
		notionVersion = defaultNotionAPIRev
	}

	return &Client{
		httpClient:    &http.Client{Timeout: 20 * time.Second},
		baseURL:       baseURL,
		notionVersion: notionVersion,
		token:         token,
	}, nil
}

func (c *Client) PatchPage(ctx context.Context, pageID string, patch map[string]any) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return fmt.Errorf("page ID is required")
	}
	if len(patch) == 0 {
		return fmt.Errorf("patch payload is required")
	}

	return c.doJSON(ctx, http.MethodPatch, "/pages/"+pageID, patch, nil)
}

func (c *Client) RetrievePageProperties(ctx context.Context, pageID string) (map[string]PagePropertyMeta, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil, fmt.Errorf("page ID is required")
	}

	var resp struct {
		Properties map[string]PagePropertyMeta `json:"properties"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/pages/"+pageID, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Properties == nil {
		return map[string]PagePropertyMeta{}, nil
	}
	return resp.Properties, nil
}

func (c *Client) RetrievePagePropertyItems(ctx context.Context, pageID, propertyID string) ([]any, error) {
	pageID = strings.TrimSpace(pageID)
	propertyID = strings.TrimSpace(propertyID)
	if pageID == "" {
		return nil, fmt.Errorf("page ID is required")
	}
	if propertyID == "" {
		return nil, fmt.Errorf("property ID is required")
	}

	escapedPropertyID := url.PathEscape(propertyID)
	basePath := "/pages/" + pageID + "/properties/" + escapedPropertyID

	items := make([]any, 0)
	var cursor string
	for {
		path := basePath
		if cursor != "" {
			path += "?start_cursor=" + url.QueryEscape(cursor)
		}

		var resp map[string]any
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}

		object, _ := resp["object"].(string)
		if object == "list" {
			if results, ok := resp["results"].([]any); ok {
				items = append(items, results...)
			}
			hasMore, _ := resp["has_more"].(bool)
			nextCursor, _ := resp["next_cursor"].(string)
			if hasMore && strings.TrimSpace(nextCursor) != "" {
				cursor = nextCursor
				continue
			}
			break
		}

		items = append(items, resp)
		break
	}

	return items, nil
}

func (c *Client) UploadFile(ctx context.Context, filename string, data []byte) (string, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("file data is required")
	}

	filename = filepath.Base(filename)

	var created FileUpload
	createPayload := map[string]any{
		"mode":     "single_part",
		"filename": filename,
	}
	if err := c.doJSONWithVersion(ctx, http.MethodPost, "/file_uploads", createPayload, &created, fileUploadAPIRev); err != nil {
		return "", err
	}
	if strings.TrimSpace(created.ID) == "" {
		return "", fmt.Errorf("create file upload failed: empty upload ID")
	}

	sent, err := c.sendFileUploadPart(ctx, created.ID, filename, data)
	if err != nil {
		return "", err
	}

	uploaded, err := c.waitForFileUploadUploaded(ctx, sent.ID)
	if err != nil {
		return "", err
	}
	return uploaded.ID, nil
}

func (c *Client) GetFileUpload(ctx context.Context, fileUploadID string) (*FileUpload, error) {
	fileUploadID = strings.TrimSpace(fileUploadID)
	if fileUploadID == "" {
		return nil, fmt.Errorf("file upload ID is required")
	}

	var out FileUpload
	if err := c.doJSONWithVersion(ctx, http.MethodGet, "/file_uploads/"+fileUploadID, nil, &out, fileUploadAPIRev); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.ID) == "" {
		out.ID = fileUploadID
	}
	return &out, nil
}

func (c *Client) AppendUploadedImageBlocks(ctx context.Context, parentID string, blocks []UploadedImageBlock) error {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return fmt.Errorf("parent ID is required")
	}
	if len(blocks) == 0 {
		return nil
	}

	children := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		id := strings.TrimSpace(block.FileUploadID)
		if id == "" {
			return fmt.Errorf("file upload ID is required for image block")
		}

		image := map[string]any{
			"type": "file_upload",
			"file_upload": map[string]any{
				"id": id,
			},
		}
		if caption := strings.TrimSpace(block.Caption); caption != "" {
			image["caption"] = []map[string]any{
				{
					"type": "text",
					"text": map[string]any{
						"content": caption,
					},
				},
			}
		}

		children = append(children, map[string]any{
			"object": "block",
			"type":   "image",
			"image":  image,
		})
	}

	payload := map[string]any{"children": children}
	return c.doJSONWithVersion(ctx, http.MethodPatch, "/blocks/"+parentID+"/children", payload, nil, fileUploadAPIRev)
}

func ParsePageIcon(value string) (PageIcon, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return PageIcon{}, fmt.Errorf("icon value is required")
	}

	switch strings.ToLower(value) {
	case "none", "clear":
		return PageIcon{Clear: true}, nil
	}

	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		parsedURL, err := url.Parse(value)
		if err != nil {
			return PageIcon{}, fmt.Errorf("invalid icon URL: %w", err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return PageIcon{}, fmt.Errorf("icon URL must use http or https")
		}
		if parsedURL.Host == "" {
			return PageIcon{}, fmt.Errorf("icon URL must include a host")
		}
		return PageIcon{ExternalURL: value}, nil
	}

	firstRune, _ := utf8.DecodeRuneInString(value)
	if firstRune == utf8.RuneError {
		return PageIcon{}, fmt.Errorf("invalid icon value")
	}
	if !isLikelyEmoji(firstRune) {
		return PageIcon{}, fmt.Errorf("icon must be an emoji, an http(s) URL, or 'none'")
	}

	return PageIcon{Emoji: value}, nil
}

func (c *Client) SetPageIcon(ctx context.Context, pageID string, icon PageIcon) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return fmt.Errorf("page ID is required")
	}

	setCount := 0
	if strings.TrimSpace(icon.Emoji) != "" {
		setCount++
	}
	if strings.TrimSpace(icon.ExternalURL) != "" {
		setCount++
	}
	if icon.Clear {
		setCount++
	}
	if setCount != 1 {
		return fmt.Errorf("icon must set exactly one of emoji, external URL, or clear")
	}

	var patch map[string]any
	switch {
	case icon.Clear:
		patch = map[string]any{
			"icon": nil,
		}
	case strings.TrimSpace(icon.ExternalURL) != "":
		parsedURL, err := url.Parse(icon.ExternalURL)
		if err != nil {
			return fmt.Errorf("invalid icon URL: %w", err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("icon URL must use http or https")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("icon URL must include a host")
		}
		patch = map[string]any{
			"icon": map[string]any{
				"type": "external",
				"external": map[string]any{
					"url": icon.ExternalURL,
				},
			},
		}
	default:
		patch = map[string]any{
			"icon": map[string]any{
				"type":  "emoji",
				"emoji": icon.Emoji,
			},
		}
	}

	return c.PatchPage(ctx, pageID, patch)
}

func (c *Client) VerifyToken(ctx context.Context) error {
	var me struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/users/me", nil, &me); err != nil {
		return err
	}
	if strings.TrimSpace(me.ID) == "" {
		return fmt.Errorf("official API verify token failed: empty user ID in response")
	}
	return nil
}

func isLikelyEmoji(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) && !unicode.IsPunct(r) && r > 127
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	return c.doJSONWithVersion(ctx, method, path, payload, out, c.notionVersion)
}

func (c *Client) doJSONWithVersion(ctx context.Context, method, path string, payload any, out any, notionVersion string) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	contentType := ""
	if payload != nil {
		contentType = "application/json"
	}
	return c.doRequest(ctx, method, path, bodyReader, contentType, out, notionVersion)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string, out any, notionVersion string) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("authorization", "Bearer "+c.token)
	req.Header.Set("notion-version", notionVersion)
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(respBody))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		} else {
			var errResp struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(respBody, &errResp); err == nil && strings.TrimSpace(errResp.Message) != "" {
				message = strings.TrimSpace(errResp.Message)
			}
		}
		return fmt.Errorf("official API %s %s failed (%d): %s", method, path, resp.StatusCode, message)
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parse official API response for %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) sendFileUploadPart(ctx context.Context, fileUploadID, filename string, data []byte) (*FileUpload, error) {
	uploadID := strings.TrimSpace(fileUploadID)
	if uploadID == "" {
		return nil, fmt.Errorf("file upload ID is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write multipart file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	var out FileUpload
	path := "/file_uploads/" + uploadID + "/send"
	if err := c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(body.Bytes()), writer.FormDataContentType(), &out, fileUploadAPIRev); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.ID) == "" {
		out.ID = uploadID
	}
	return &out, nil
}

func (c *Client) waitForFileUploadUploaded(ctx context.Context, fileUploadID string) (*FileUpload, error) {
	id := strings.TrimSpace(fileUploadID)
	if id == "" {
		return nil, fmt.Errorf("file upload ID is required")
	}

	const maxChecks = 20
	for i := 0; i < maxChecks; i++ {
		upload, err := c.GetFileUpload(ctx, id)
		if err != nil {
			return nil, err
		}

		status := strings.ToLower(strings.TrimSpace(upload.Status))
		switch status {
		case "", "uploaded":
			return upload, nil
		case "pending":
			if i == maxChecks-1 {
				return nil, fmt.Errorf("file upload %s did not reach uploaded status in time", id)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		default:
			return nil, fmt.Errorf("file upload %s failed with status %q", id, upload.Status)
		}
	}
	return nil, fmt.Errorf("file upload %s did not reach uploaded status in time", id)
}

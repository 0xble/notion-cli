package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lox/notion-cli/internal/config"
)

const (
	defaultBaseURL      = "https://api.notion.com/v1"
	defaultNotionAPIRev = "2022-06-28"
)

type Client struct {
	httpClient    *http.Client
	baseURL       string
	notionVersion string
	token         string
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

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("authorization", "Bearer "+c.token)
	req.Header.Set("notion-version", c.notionVersion)
	if payload != nil {
		req.Header.Set("content-type", "application/json")
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

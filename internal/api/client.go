package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lox/notion-cli/internal/config"
)

// APIError is a structured error returned by the Notion API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	parts := make([]string, 0, 3)
	parts = append(parts, fmt.Sprintf("notion API error (status %d)", e.StatusCode))
	if e.Code != "" {
		parts = append(parts, fmt.Sprintf("code=%s", e.Code))
	}
	if e.RequestID != "" {
		parts = append(parts, fmt.Sprintf("request_id=%s", e.RequestID))
	}
	msg := strings.Join(parts, " ")
	if e.Message != "" {
		msg = msg + ": " + e.Message
	}
	return msg
}

// Page represents a Notion page response (minimal fields the client surfaces today).
type Page struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Archived bool   `json:"archived"`
}

// Icon represents a Notion page icon. Left opaque for callers that pass it through.
type Icon map[string]any

// Cover represents a Notion page cover. Left opaque for callers that pass it through.
type Cover map[string]any

// PropertyValue represents a Notion property value. Left opaque for callers that pass it through.
type PropertyValue map[string]any

// PageUpdate is the typed payload for PatchPage.
// All fields are optional; only non-nil fields are sent.
type PageUpdate struct {
	Archived   *bool
	Icon       *Icon
	Cover      *Cover
	Properties map[string]PropertyValue
}

func (u PageUpdate) payload() (map[string]any, error) {
	out := make(map[string]any)
	if u.Archived != nil {
		out["archived"] = *u.Archived
	}
	if u.Icon != nil {
		out["icon"] = *u.Icon
	}
	if u.Cover != nil {
		out["cover"] = *u.Cover
	}
	if len(u.Properties) > 0 {
		out["properties"] = u.Properties
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("page update is empty")
	}
	return out, nil
}

type Client struct {
	httpClient    *http.Client
	baseURL       string
	notionVersion string
	token         string
}

func NewClient(cfg config.APIConfig, token string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("notion API token is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = config.DefaultAPIBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	notionVersion := strings.TrimSpace(cfg.NotionVersion)
	if notionVersion == "" {
		notionVersion = config.DefaultNotionAPIVersion
	}

	return &Client{
		httpClient:    &http.Client{Timeout: 20 * time.Second},
		baseURL:       baseURL,
		notionVersion: notionVersion,
		token:         token,
	}, nil
}

func (c *Client) PatchPage(ctx context.Context, pageID string, update PageUpdate) (*Page, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil, fmt.Errorf("page ID is required")
	}

	payload, err := update.payload()
	if err != nil {
		return nil, err
	}

	pagePath, err := url.JoinPath(c.baseURL, "pages", url.PathEscape(pageID))
	if err != nil {
		return nil, fmt.Errorf("build page URL: %w", err)
	}

	var page Page
	if err := c.doJSON(ctx, http.MethodPatch, pagePath, payload, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (c *Client) doJSON(ctx context.Context, method, fullURL string, payload any, out any) error {
	var body []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = data
	}

	resp, err := c.sendOnce(ctx, method, fullURL, body, payload != nil)
	if err != nil {
		return err
	}

	// Minimal 429 handling: honor Retry-After once, retry once.
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		_ = resp.Body.Close()
		if retryAfter > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryAfter):
			}
		}
		resp, err = c.sendOnce(ctx, method, fullURL, body, payload != nil)
		if err != nil {
			return err
		}
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("parse notion API response for %s %s: %w", method, fullURL, err)
	}
	return nil
}

func (c *Client) sendOnce(ctx context.Context, method, fullURL string, body []byte, hasPayload bool) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", c.notionVersion)
	if hasPayload {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func parseAPIError(resp *http.Response) error {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Notion-Request-Id"),
	}

	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err == nil {
		apiErr.Code = strings.TrimSpace(parsed.Code)
		apiErr.Message = strings.TrimSpace(parsed.Message)
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	return apiErr
}

func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

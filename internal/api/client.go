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
	"unicode"
	"unicode/utf8"

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

type PageIcon struct {
	Emoji       string
	ExternalURL string
	Clear       bool
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

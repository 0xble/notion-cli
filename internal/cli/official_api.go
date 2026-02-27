package cli

import (
	"fmt"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/config"
)

func RequireOfficialAPIClient() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	client, err := api.NewClient(cfg.API, cfg.API.Token)
	if err != nil {
		return nil, fmt.Errorf("create official API client: %w (set api.token in ~/.config/notion-cli/config.json or NOTION_API_TOKEN)", err)
	}

	return client, nil
}

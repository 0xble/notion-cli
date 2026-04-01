package cli

import (
	"fmt"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/config"
)

type OfficialAPIConfig struct {
	Config         config.Config
	ConfigPath     string
	APITokenSource string
}

func LoadOfficialAPIConfig() (*OfficialAPIConfig, error) {
	loaded, err := config.LoadWithMeta()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &OfficialAPIConfig{
		Config:         loaded.Config,
		ConfigPath:     loaded.Path,
		APITokenSource: loaded.APITokenSource,
	}, nil
}

func RequireOfficialAPIClient() (*api.Client, error) {
	loaded, err := LoadOfficialAPIConfig()
	if err != nil {
		return nil, err
	}

	client, err := api.NewClient(loaded.Config.API, loaded.Config.API.Token)
	if err != nil {
		return nil, fmt.Errorf("create official API client: %w (run 'notion-cli auth api setup' or set NOTION_API_TOKEN)", err)
	}
	return client, nil
}

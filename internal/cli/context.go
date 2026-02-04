package cli

import (
	"context"
	"fmt"

	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

func GetClient() (*mcp.Client, error) {
	client, err := mcp.NewClient()
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		if mcp.IsAuthRequired(err) {
			output.PrintWarning("Not authenticated. Run 'notion config auth' to authenticate.")
			return nil, err
		}
		return nil, fmt.Errorf("start client: %w", err)
	}

	return client, nil
}

func RequireClient() (*mcp.Client, error) {
	return GetClient()
}

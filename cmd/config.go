package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
)

type ConfigCmd struct {
	Auth  ConfigAuthCmd  `cmd:"" default:"withargs" help:"Run OAuth flow to authenticate"`
	Show  ConfigShowCmd  `cmd:"" help:"Show current configuration"`
	Clear ConfigClearCmd `cmd:"" help:"Clear stored credentials"`
}

type ConfigAuthCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *ConfigAuthCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runConfigAuth(ctx)
}

func runConfigAuth(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	if err := mcp.RunOAuthFlow(bgCtx, tokenStore); err != nil {
		output.PrintError(err)
		return err
	}

	return nil
}

type ConfigShowCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *ConfigShowCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON
	return runConfigShow(ctx)
}

func runConfigShow(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	token, err := tokenStore.GetToken(context.Background())
	if err != nil {
		if err == mcp.ErrNoToken {
			fmt.Println("Not configured. Run 'notion config auth' to authenticate.")
			return nil
		}
		output.PrintError(err)
		return err
	}

	hasValidToken := token.AccessToken != "" && !token.IsExpired()

	if ctx.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"authenticated": hasValidToken,
			"token_type":    token.TokenType,
			"has_token":     token.AccessToken != "",
			"expires_at":    token.ExpiresAt,
			"config_path":   tokenStore.Path(),
		})
	}

	labelStyle := color.New(color.Faint)

	if hasValidToken {
		output.PrintSuccess("Authenticated")
	} else {
		output.PrintWarning("Token expired or not set")
	}
	fmt.Println()

	labelStyle.Print("Config path: ")
	fmt.Println(tokenStore.Path())

	labelStyle.Print("Token type:  ")
	fmt.Println(token.TokenType)

	if !token.ExpiresAt.IsZero() {
		labelStyle.Print("Expires:     ")
		fmt.Println(token.ExpiresAt.Format("2 Jan 2006 15:04"))
	}

	return nil
}

type ConfigClearCmd struct{}

func (c *ConfigClearCmd) Run(ctx *Context) error {
	return runConfigClear(ctx)
}

func runConfigClear(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	if err := tokenStore.Clear(); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Credentials cleared")
	return nil
}

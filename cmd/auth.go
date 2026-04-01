package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/mcp"
	"github.com/lox/notion-cli/internal/output"
	"golang.org/x/term"
)

type AuthCmd struct {
	Login   AuthLoginCmd   `cmd:"" help:"Authenticate with Notion via OAuth"`
	Refresh AuthRefreshCmd `cmd:"" help:"Refresh the access token"`
	Status  AuthStatusCmd  `cmd:"" default:"withargs" help:"Show authentication status"`
	Logout  AuthLogoutCmd  `cmd:"" help:"Clear stored credentials"`
	API     AuthAPICmd     `cmd:"" name:"api" help:"Official API token commands"`
}

var authAPIInput io.Reader = os.Stdin
var authAPIOutput io.Writer = os.Stdout
var authAPIError io.Writer = os.Stderr

type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run(ctx *Context) error {
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

type AuthRefreshCmd struct{}

func (c *AuthRefreshCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	token, err := tokenStore.GetToken(bgCtx)
	if err != nil {
		if err == mcp.ErrNoToken {
			output.PrintWarning("Not authenticated. Run 'notion-cli auth login' first.")
			return err
		}
		output.PrintError(err)
		return err
	}

	if token.RefreshToken == "" {
		output.PrintWarning("No refresh token available. Run 'notion-cli auth login' to re-authenticate.")
		return fmt.Errorf("no refresh token")
	}

	newToken, err := mcp.RefreshToken(bgCtx, tokenStore)
	if err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Token refreshed")
	fmt.Printf("Expires: %s\n", newToken.ExpiresAt.Format("2 Jan 2006 15:04"))
	return nil
}

type AuthStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthStatusCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	token, err := tokenStore.GetToken(context.Background())
	if err != nil {
		if err == mcp.ErrNoToken {
			fmt.Println("Not authenticated. Run 'notion-cli auth login' to authenticate.")
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

	_, _ = labelStyle.Print("Config path: ")
	fmt.Println(tokenStore.Path())

	_, _ = labelStyle.Print("Token type:  ")
	fmt.Println(token.TokenType)

	if !token.ExpiresAt.IsZero() {
		_, _ = labelStyle.Print("Expires:     ")
		fmt.Println(token.ExpiresAt.Format("2 Jan 2006 15:04"))
	}

	return nil
}

type AuthLogoutCmd struct{}

func (c *AuthLogoutCmd) Run(ctx *Context) error {
	tokenStore, err := mcp.NewFileTokenStore()
	if err != nil {
		output.PrintError(err)
		return err
	}

	if err := tokenStore.Clear(); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Logged out")
	return nil
}

type AuthAPICmd struct {
	Setup  AuthAPISetupCmd  `cmd:"" help:"Set up official Notion API token"`
	Status AuthAPIStatusCmd `cmd:"" help:"Show official API token status"`
	Verify AuthAPIVerifyCmd `cmd:"" help:"Verify official API token"`
	Unset  AuthAPIUnsetCmd  `cmd:"" help:"Remove saved official API token"`
}

type AuthAPISetupCmd struct{}

func (c *AuthAPISetupCmd) Run(ctx *Context) error {
	token, err := readOfficialAPIToken(authAPIInput, authAPIOutput, authAPIError)
	if err != nil {
		output.PrintError(err)
		return err
	}
	if token == "" {
		err := fmt.Errorf("official API token cannot be empty")
		output.PrintError(err)
		return err
	}
	if err := config.SetAPIToken(token); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Official API token saved")
	fmt.Fprintf(authAPIOutput, "Config path: %s\n", mustConfigPath())
	return nil
}

type AuthAPIStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIStatusCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	loaded, err := cli.LoadOfficialAPIConfig()
	if err != nil {
		output.PrintError(err)
		return err
	}
	return printAuthAPIStatus(ctx, loaded)
}

type AuthAPIVerifyCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIVerifyCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	loaded, err := cli.LoadOfficialAPIConfig()
	if err != nil {
		output.PrintError(err)
		return err
	}
	client, err := cli.RequireOfficialAPIClient()
	if err != nil {
		output.PrintError(err)
		return err
	}

	self, err := client.GetSelf(context.Background())
	if err != nil {
		output.PrintError(err)
		return err
	}

	if ctx.JSON {
		enc := json.NewEncoder(authAPIOutput)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"verified":       true,
			"token_source":   loaded.APITokenSource,
			"config_path":    loaded.ConfigPath,
			"base_url":       loaded.Config.API.BaseURL,
			"notion_version": loaded.Config.API.NotionVersion,
			"self":           self,
		})
	}

	output.PrintSuccess("Official API token verified")
	fmt.Fprintf(authAPIOutput, "Token source:   %s\n", loaded.APITokenSource)
	fmt.Fprintf(authAPIOutput, "Config path:    %s\n", loaded.ConfigPath)
	fmt.Fprintf(authAPIOutput, "Base URL:       %s\n", loaded.Config.API.BaseURL)
	fmt.Fprintf(authAPIOutput, "Notion version: %s\n", loaded.Config.API.NotionVersion)
	if self.Name != "" {
		fmt.Fprintf(authAPIOutput, "Actor:          %s\n", self.Name)
	}
	if self.Bot != nil && self.Bot.WorkspaceName != "" {
		fmt.Fprintf(authAPIOutput, "Workspace:      %s\n", self.Bot.WorkspaceName)
	}
	return nil
}

type AuthAPIUnsetCmd struct{}

func (c *AuthAPIUnsetCmd) Run(ctx *Context) error {
	if err := config.UnsetAPIToken(); err != nil {
		output.PrintError(err)
		return err
	}
	output.PrintSuccess("Official API token removed")
	fmt.Fprintf(authAPIOutput, "Config path: %s\n", mustConfigPath())
	return nil
}

func printAuthAPIStatus(ctx *Context, loaded *cli.OfficialAPIConfig) error {
	hasToken := strings.TrimSpace(loaded.Config.API.Token) != ""
	if ctx.JSON {
		enc := json.NewEncoder(authAPIOutput)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"configured":     hasToken,
			"token_source":   loaded.APITokenSource,
			"config_path":    loaded.ConfigPath,
			"base_url":       loaded.Config.API.BaseURL,
			"notion_version": loaded.Config.API.NotionVersion,
		})
	}

	if hasToken {
		output.PrintSuccess("Official API token configured")
	} else {
		output.PrintWarning("Official API token not configured")
	}
	fmt.Fprintln(authAPIOutput)
	fmt.Fprintf(authAPIOutput, "Token source:   %s\n", loaded.APITokenSource)
	fmt.Fprintf(authAPIOutput, "Config path:    %s\n", loaded.ConfigPath)
	fmt.Fprintf(authAPIOutput, "Base URL:       %s\n", loaded.Config.API.BaseURL)
	fmt.Fprintf(authAPIOutput, "Notion version: %s\n", loaded.Config.API.NotionVersion)
	if !hasToken {
		fmt.Fprintln(authAPIOutput, "Run 'notion-cli auth api setup' or set NOTION_API_TOKEN.")
	}
	return nil
}

func readOfficialAPIToken(in io.Reader, out, errOut io.Writer) (string, error) {
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(errOut, "Official API token: ")
		secret, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(errOut)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(secret)), nil
	}

	fmt.Fprint(out, "Official API token: ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func mustConfigPath() string {
	path, err := config.Path()
	if err != nil {
		return "<unknown>"
	}
	return path
}

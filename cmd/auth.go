package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
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
	Use     AuthUseCmd     `cmd:"" help:"Set the active account"`
	List    AuthListCmd    `cmd:"" help:"List saved accounts"`
	API     AuthAPICmd     `cmd:"" name:"api" help:"Official Notion API token setup and status"`
}

type AuthLoginCmd struct {
	SetupAPI           bool `help:"Run official API token setup after login" name:"setup-api"`
	SkipAPISetupPrompt bool `help:"Skip optional official API setup prompt after login" name:"skip-api-setup-prompt"`
}

func (c *AuthLoginCmd) Run(ctx *Context) error {
	account, err := resolveAuthAccount(ctx.Account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	tokenStore, err := mcp.NewFileTokenStoreForAccount(account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	if err := mcp.RunOAuthFlow(bgCtx, tokenStore); err != nil {
		output.PrintError(err)
		return err
	}

	if err := mcp.SetActiveAccount(account); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess(fmt.Sprintf("Active account set to '%s'", account))

	if err := maybeRunPostLoginAPISetup(c); err != nil {
		output.PrintError(err)
		return err
	}

	return nil
}

type AuthRefreshCmd struct {
}

func (c *AuthRefreshCmd) Run(ctx *Context) error {
	account, err := resolveAuthAccount(ctx.Account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	tokenStore, err := mcp.NewFileTokenStoreForAccount(account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	bgCtx := context.Background()
	token, err := tokenStore.GetToken(bgCtx)
	if err != nil {
		if errors.Is(err, mcp.ErrNoToken) {
			output.PrintWarning(fmt.Sprintf("Account '%s' is not authenticated. Run 'notion-cli auth login --account %s' first.", account, account))
			return err
		}
		output.PrintError(err)
		return err
	}

	if token.RefreshToken == "" {
		output.PrintWarning(fmt.Sprintf("No refresh token available for account '%s'. Run 'notion-cli auth login --account %s' to re-authenticate.", account, account))
		return fmt.Errorf("no refresh token")
	}

	newToken, err := mcp.RefreshToken(bgCtx, tokenStore)
	if err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess(fmt.Sprintf("Token refreshed for account '%s'", account))
	fmt.Printf("Expires: %s\n", newToken.ExpiresAt.Format("2 Jan 2006 15:04"))
	return nil
}

type AuthStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
	All  bool `help:"Show status for all saved accounts"`
}

func (c *AuthStatusCmd) Run(ctx *Context) error {
	ctx.JSON = c.JSON

	if c.All && ctx.Account != "" {
		return fmt.Errorf("--all cannot be used with --account")
	}

	if c.All {
		return c.runAll(ctx)
	}

	account, err := resolveAuthAccount(ctx.Account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	tokenStore, err := mcp.NewFileTokenStoreForAccount(account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	token, err := tokenStore.GetToken(context.Background())
	if err != nil {
		if errors.Is(err, mcp.ErrNoToken) {
			if ctx.JSON {
				return writeJSON(map[string]any{
					"account":       account,
					"authenticated": false,
					"has_token":     false,
					"config_path":   tokenStore.Path(),
				})
			}
			fmt.Printf("Account '%s' is not authenticated. Run 'notion-cli auth login --account %s' to authenticate.\n", account, account)
			return nil
		}
		output.PrintError(err)
		return err
	}

	hasValidToken := token.AccessToken != "" && !token.IsExpired()

	if ctx.JSON {
		return writeJSON(map[string]any{
			"account":       account,
			"authenticated": hasValidToken,
			"token_type":    token.TokenType,
			"has_token":     token.AccessToken != "",
			"expires_at":    token.ExpiresAt,
			"config_path":   tokenStore.Path(),
		})
	}

	labelStyle := color.New(color.Faint)

	if hasValidToken {
		output.PrintSuccess(fmt.Sprintf("Authenticated (%s)", account))
	} else {
		output.PrintWarning(fmt.Sprintf("Token expired or not set (%s)", account))
	}
	fmt.Println()

	_, _ = labelStyle.Print("Account:     ")
	fmt.Println(account)

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

func (c *AuthStatusCmd) runAll(ctx *Context) error {
	accounts, err := mcp.ListAccounts()
	if err != nil {
		output.PrintError(err)
		return err
	}

	activeAccount, err := mcp.GetActiveAccount()
	if err != nil {
		output.PrintError(err)
		return err
	}

	if len(accounts) == 0 {
		accounts = []string{activeAccount}
	} else if !slices.Contains(accounts, activeAccount) {
		accounts = append(accounts, activeAccount)
	}
	slices.Sort(accounts)

	type accountStatus struct {
		Account       string    `json:"account"`
		Authenticated bool      `json:"authenticated"`
		HasToken      bool      `json:"has_token"`
		TokenType     string    `json:"token_type,omitempty"`
		ExpiresAt     time.Time `json:"expires_at,omitempty"`
		ConfigPath    string    `json:"config_path"`
	}

	statuses := make([]accountStatus, 0, len(accounts))
	for _, account := range accounts {
		tokenStore, err := mcp.NewFileTokenStoreForAccount(account)
		if err != nil {
			output.PrintError(err)
			return err
		}

		status := accountStatus{Account: account, ConfigPath: tokenStore.Path()}
		token, err := tokenStore.GetToken(context.Background())
		if err != nil {
			if !errors.Is(err, mcp.ErrNoToken) {
				output.PrintError(err)
				return err
			}
			statuses = append(statuses, status)
			continue
		}

		status.HasToken = token.AccessToken != ""
		status.Authenticated = status.HasToken && !token.IsExpired()
		status.TokenType = token.TokenType
		status.ExpiresAt = token.ExpiresAt
		statuses = append(statuses, status)
	}

	if ctx.JSON {
		return writeJSON(map[string]any{
			"active_account": activeAccount,
			"accounts":       statuses,
		})
	}

	fmt.Printf("Active account: %s\n\n", activeAccount)
	for _, status := range statuses {
		marker := " "
		if status.Account == activeAccount {
			marker = "*"
		}

		state := "not authenticated"
		if status.Authenticated {
			state = "authenticated"
		} else if status.HasToken {
			state = "expired"
		}

		fmt.Printf("%s %-16s %s\n", marker, status.Account, state)
	}

	return nil
}

type AuthLogoutCmd struct {
	All bool `help:"Clear tokens for all accounts"`
}

func (c *AuthLogoutCmd) Run(ctx *Context) error {
	if c.All && ctx.Account != "" {
		return fmt.Errorf("--all cannot be used with --account")
	}

	if c.All {
		if err := mcp.ClearAllTokens(); err != nil {
			output.PrintError(err)
			return err
		}
		if err := mcp.SetActiveAccount("default"); err != nil {
			output.PrintError(err)
			return err
		}
		output.PrintSuccess("Logged out all accounts")
		return nil
	}

	account, err := resolveAuthAccount(ctx.Account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	tokenStore, err := mcp.NewFileTokenStoreForAccount(account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if err := tokenStore.Clear(); err != nil {
		output.PrintError(err)
		return err
	}

	activeAccount, err := mcp.GetActiveAccount()
	if err != nil {
		output.PrintError(err)
		return err
	}
	if activeAccount == account {
		if err := mcp.SetActiveAccount("default"); err != nil {
			output.PrintError(err)
			return err
		}
	}

	output.PrintSuccess(fmt.Sprintf("Logged out account '%s'", account))
	return nil
}

type AuthUseCmd struct {
	Account string `arg:"" name:"account" help:"Account profile name"`
}

func (c *AuthUseCmd) Run(ctx *Context) error {
	account, err := resolveAuthAccount(c.Account)
	if err != nil {
		output.PrintError(err)
		return err
	}

	if err := mcp.SetActiveAccount(account); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess(fmt.Sprintf("Active account set to '%s'", account))
	return nil
}

type AuthListCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthListCmd) Run(ctx *Context) error {
	accounts, err := mcp.ListAccounts()
	if err != nil {
		output.PrintError(err)
		return err
	}

	activeAccount, err := mcp.GetActiveAccount()
	if err != nil {
		output.PrintError(err)
		return err
	}

	if c.JSON {
		return writeJSON(map[string]any{
			"active_account": activeAccount,
			"accounts":       accounts,
		})
	}

	if len(accounts) == 0 {
		fmt.Println("No saved accounts. Run 'notion-cli auth login --account <name>' to add one.")
		fmt.Printf("Active account: %s\n", activeAccount)
		return nil
	}

	fmt.Printf("Active account: %s\n\n", activeAccount)
	for _, account := range accounts {
		marker := " "
		if account == activeAccount {
			marker = "*"
		}
		fmt.Printf("%s %s\n", marker, account)
	}

	return nil
}

func resolveAuthAccount(account string) (string, error) {
	account = strings.TrimSpace(account)
	return mcp.ResolveAccountName(account)
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func maybeRunPostLoginAPISetup(cmd *AuthLoginCmd) error {
	if cmd == nil {
		return nil
	}
	if cmd.SkipAPISetupPrompt {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.API.Token) != "" {
		return nil
	}

	if cmd.SetupAPI {
		return runAuthAPISetup(authAPISetupOptions{FromLogin: true})
	}
	if !isInteractiveTerminal() {
		return nil
	}

	yes, err := promptYesNo("Enable official API features now? [y/N]: ")
	if err != nil {
		return err
	}
	if !yes {
		output.PrintInfo("Skipping official API setup. You can run 'notion-cli auth api setup' later.")
		return nil
	}

	return runAuthAPISetup(authAPISetupOptions{FromLogin: true})
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func promptYesNo(prompt string) (bool, error) {
	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

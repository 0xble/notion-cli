package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lox/notion-cli/internal/api"
	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/output"
)

const (
	apiSetupInternalIntegrationsURL = "https://www.notion.so/profile/integrations/internal"
	apiSetupDocsURL                 = apiSetupInternalIntegrationsURL
)

type AuthAPICmd struct {
	Setup  AuthAPISetupCmd  `cmd:"" help:"Set up official Notion API token"`
	Status AuthAPIStatusCmd `cmd:"" help:"Show official Notion API token status"`
	Verify AuthAPIVerifyCmd `cmd:"" help:"Verify official Notion API token"`
	Unset  AuthAPIUnsetCmd  `cmd:"" help:"Remove saved official Notion API token"`
}

type AuthAPISetupCmd struct {
	Token    string `help:"Official Notion API token (optional; skips token input prompt)" name:"api-token"`
	NoVerify bool   `help:"Save token without verifying it against Notion API" name:"no-verify"`
	OpenDocs bool   `help:"Open integration setup docs in browser before setup" name:"open-docs"`
}

func (c *AuthAPISetupCmd) Run(ctx *Context) error {
	return runAuthAPISetup(authAPISetupOptions{
		Token:    c.Token,
		NoVerify: c.NoVerify,
		OpenDocs: c.OpenDocs,
	})
}

type AuthAPIStatusCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIStatusCmd) Run(ctx *Context) error {
	fileCfg, err := config.LoadFile()
	if err != nil {
		output.PrintError(err)
		return err
	}
	effectiveCfg, err := config.Load()
	if err != nil {
		output.PrintError(err)
		return err
	}
	path, err := config.Path()
	if err != nil {
		output.PrintError(err)
		return err
	}

	tokenSource := "none"
	if strings.TrimSpace(os.Getenv("NOTION_API_TOKEN")) != "" {
		tokenSource = "env"
	} else if strings.TrimSpace(fileCfg.API.Token) != "" {
		tokenSource = "config"
	}

	if c.JSON {
		return writeJSON(map[string]any{
			"configured":     strings.TrimSpace(effectiveCfg.API.Token) != "",
			"token_source":   tokenSource,
			"config_path":    path,
			"base_url":       effectiveCfg.API.BaseURL,
			"notion_version": effectiveCfg.API.NotionVersion,
		})
	}

	if strings.TrimSpace(effectiveCfg.API.Token) == "" {
		output.PrintWarning("Official API token is not configured")
	} else {
		output.PrintSuccess("Official API token is configured")
	}

	fmt.Printf("Source:         %s\n", tokenSource)
	fmt.Printf("Config path:    %s\n", path)
	fmt.Printf("API base URL:   %s\n", effectiveCfg.API.BaseURL)
	fmt.Printf("Notion version: %s\n", effectiveCfg.API.NotionVersion)
	if tokenSource == "env" {
		output.PrintInfo("Token comes from NOTION_API_TOKEN and is not persisted in config.")
	}

	return nil
}

type AuthAPIVerifyCmd struct {
	Token string `help:"Official Notion API token to verify (defaults to configured token)" name:"api-token"`
}

func (c *AuthAPIVerifyCmd) Run(ctx *Context) error {
	cfg, err := config.Load()
	if err != nil {
		output.PrintError(err)
		return err
	}

	token := strings.TrimSpace(c.Token)
	if token == "" {
		token = strings.TrimSpace(cfg.API.Token)
	}
	if token == "" {
		err := &output.UserError{Message: "Official API token is not configured. Run 'notion-cli auth api setup' first."}
		output.PrintError(err)
		return err
	}

	client, err := api.NewClient(cfg.API, token)
	if err != nil {
		output.PrintError(err)
		return err
	}
	if err := client.VerifyToken(context.Background()); err != nil {
		output.PrintError(err)
		return err
	}

	output.PrintSuccess("Official API token is valid")
	return nil
}

type AuthAPIUnsetCmd struct {
	JSON bool `help:"Output as JSON" short:"j"`
}

func (c *AuthAPIUnsetCmd) Run(ctx *Context) error {
	fileCfg, err := config.LoadFile()
	if err != nil {
		output.PrintError(err)
		return err
	}
	path, err := config.Path()
	if err != nil {
		output.PrintError(err)
		return err
	}

	hadToken := strings.TrimSpace(fileCfg.API.Token) != ""
	fileCfg.API.Token = ""
	if err := config.Save(fileCfg); err != nil {
		output.PrintError(err)
		return err
	}

	if c.JSON {
		return writeJSON(map[string]any{
			"had_token":   hadToken,
			"config_path": path,
		})
	}

	if hadToken {
		output.PrintSuccess("Removed saved official API token")
	} else {
		output.PrintInfo("No saved official API token was set")
	}
	if strings.TrimSpace(os.Getenv("NOTION_API_TOKEN")) != "" {
		output.PrintWarning("NOTION_API_TOKEN is still set in your environment and will override config.")
	}
	return nil
}

type authAPISetupOptions struct {
	Token     string
	NoVerify  bool
	OpenDocs  bool
	FromLogin bool
}

func runAuthAPISetup(opts authAPISetupOptions) error {
	if opts.OpenDocs {
		if err := openBrowserURL(apiSetupDocsURL); err != nil {
			output.PrintWarning(fmt.Sprintf("Could not open browser automatically: %v", err))
		}
	}

	cfgEffective, err := config.Load()
	if err != nil {
		return err
	}
	cfgFile, err := config.LoadFile()
	if err != nil {
		return err
	}

	token := strings.TrimSpace(opts.Token)
	if token == "" {
		if !isInteractiveTerminal() {
			return &output.UserError{Message: "Token input requires a terminal. Pass --api-token or set NOTION_API_TOKEN."}
		}
		token, err = runAuthAPISetupWizard()
		if err != nil {
			if errors.Is(err, errAuthAPISetupCancelled) {
				if opts.FromLogin {
					output.PrintInfo("Skipped official API setup. Run 'notion-cli auth api setup' later.")
					return nil
				}
				output.PrintInfo("Official API setup cancelled")
				return nil
			}
			return err
		}
	}

	if !opts.NoVerify {
		client, err := api.NewClient(cfgEffective.API, token)
		if err != nil {
			return err
		}
		if err := client.VerifyToken(context.Background()); err != nil {
			return err
		}
	}

	cfgFile.API.BaseURL = cfgEffective.API.BaseURL
	cfgFile.API.NotionVersion = cfgEffective.API.NotionVersion
	cfgFile.API.Token = token
	if err := config.Save(cfgFile); err != nil {
		return err
	}

	output.PrintSuccess("Official API token saved")
	if !opts.NoVerify {
		output.PrintSuccess("Official API token verified")
	}
	return nil
}

func openBrowserURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

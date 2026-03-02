package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/lox/notion-cli/cmd"
	"github.com/lox/notion-cli/internal/cli"
	"github.com/lox/notion-cli/internal/config"
	"github.com/lox/notion-cli/internal/output"
)

var version = "dev"

func main() {
	c := &cmd.CLI{}
	ctx := kong.Parse(c,
		kong.Name("notion"),
		kong.Description("A CLI for Notion"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	cli.SetAccessToken(c.Token)
	cli.SetAccount(c.Account)

	cfg, err := config.Load()
	if err != nil {
		output.PrintError(err)
		os.Exit(1)
	}

	err = ctx.Run(&cmd.Context{
		Token:   c.Token,
		Account: c.Account,
		Config:  cfg,
	})
	ctx.FatalIfErrorf(err)
	os.Exit(0)
}

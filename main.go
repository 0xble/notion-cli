package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/lox/notion-cli/cmd"
)

var version = "dev"

func main() {
	cli := &cmd.CLI{}
	ctx := kong.Parse(cli,
		kong.Name("notion"),
		kong.Description("A CLI for Notion"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	err := ctx.Run(&cmd.Context{})
	ctx.FatalIfErrorf(err)
	os.Exit(0)
}

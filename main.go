package main

import (
	"github.com/Crocmagnon/downloader-go/internal/freebox"
	"github.com/alecthomas/kong"
	"os"
)

type Context struct {
	OutputDir string
	Headless  bool
}

type FreeboxCmd struct {
	Username string `required:"" short:"u" help:"Freebox username"`
	Password string `required:"" short:"p" help:"Freebox password"`
}

func (r *FreeboxCmd) Run(ctx *Context) error {
	return freebox.Run(os.Stdout, os.Stderr, r.Username, r.Password, ctx.OutputDir, ctx.Headless)
}

type Cli struct {
	OutputDir string `help:"Output directory." required:"" short:"o" type:"path"`
	Headless  bool   `help:"Enable headless mode."`

	Freebox FreeboxCmd `cmd:"" help:"Download latest Freebox invoice."`
}

func main() {
	var cli Cli
	ctx := kong.Parse(&cli)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run(&Context{OutputDir: cli.OutputDir, Headless: cli.Headless})
	ctx.FatalIfErrorf(err)
}

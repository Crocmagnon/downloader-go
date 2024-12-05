package main

import (
	"github.com/Crocmagnon/downloader-go/internal/freebox"
	"github.com/Crocmagnon/downloader-go/internal/freemobile"
	"github.com/alecthomas/kong"
	"os"
)

type Context struct {
	OutputDir     string
	Headless      bool
	NoInteraction bool
}

type FreeboxCmd struct {
	Username string `required:"" short:"u" help:"Freebox username"`
	Password string `required:"" short:"p" help:"Freebox password"`
}

func (r *FreeboxCmd) Run(ctx *Context) error {
	return freebox.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type FreeMobileCmd struct {
	Username string `required:"" short:"u" help:"Free mobile username"`
	Password string `required:"" short:"p" help:"Free mobile password"`
}

func (r *FreeMobileCmd) Run(ctx *Context) error {
	return freemobile.Run(os.Stdout, os.Stderr, os.Stdin, ctx.Headless, r.Username, r.Password, ctx.OutputDir, ctx.NoInteraction)
}

type Cli struct {
	OutputDir     string `help:"Output directory." required:"" short:"o" type:"path"`
	Headless      bool   `help:"Enable headless mode."`
	NoInteraction bool   `help:"Enable interaction-less mode. In this mode, if a user interaction is required, it will generate an error instead."`

	Freebox    FreeboxCmd    `cmd:"" help:"Download latest Freebox invoice."`
	FreeMobile FreeMobileCmd `cmd:"" help:"Download latest Free mobile invoice."`
}

func main() {
	var cli Cli
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{OutputDir: cli.OutputDir, Headless: cli.Headless, NoInteraction: cli.NoInteraction})
	ctx.FatalIfErrorf(err)
}

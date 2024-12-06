package main

import (
	"github.com/Crocmagnon/downloader-go/internal/eaudugrandlyon"
	"github.com/Crocmagnon/downloader-go/internal/freebox"
	"github.com/Crocmagnon/downloader-go/internal/freemobile"
	"github.com/Crocmagnon/downloader-go/internal/lclchecking"
	"github.com/Crocmagnon/downloader-go/internal/octopusenergyaddress"
	"github.com/Crocmagnon/downloader-go/internal/shiva"
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

type EauDuGrandLyonCmd struct {
	Username string `required:"" short:"u" help:"Eau du Grand Lyon username"`
	Password string `required:"" short:"p" help:"Eau du Grand Lyon password"`
}

func (r *EauDuGrandLyonCmd) Run(ctx *Context) error {
	return eaudugrandlyon.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type OctopusEnergyAddressCmd struct {
	Username string `required:"" short:"u" help:"Octopus Energy username"`
	Password string `required:"" short:"p" help:"Octopus Energy password"`
}

func (r *OctopusEnergyAddressCmd) Run(ctx *Context) error {
	return octopusenergyaddress.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type ShivaCmd struct {
	Username string `required:"" short:"u" help:"Shiva username"`
	Password string `required:"" short:"p" help:"Shiva password"`
}

func (r *ShivaCmd) Run(ctx *Context) error {
	return shiva.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type LCLCheckingCmd struct {
	Username string `required:"" short:"u" help:"LCL username"`
	Password string `required:"" short:"p" help:"LCL password"`
}

func (r *LCLCheckingCmd) Run(ctx *Context) error {
	return lclchecking.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type Cli struct {
	OutputDir     string `help:"Output directory." required:"" short:"o" type:"path"`
	Headless      bool   `help:"Enable headless mode."`
	NoInteraction bool   `help:"Enable interaction-less mode. In this mode, if a user interaction is required, it will generate an error instead."`

	Freebox              FreeboxCmd              `cmd:"" help:"Download latest invoice from Freebox."`
	FreeMobile           FreeMobileCmd           `cmd:"" help:"Download latest invoice from Free mobile."`
	EauDuGrandLyon       EauDuGrandLyonCmd       `cmd:"" help:"Download latest invoice from Eau du Grand Lyon."`
	OctopusEnergyAddress OctopusEnergyAddressCmd `cmd:"" help:"Download latest proof of address from Octopus Energy."`
	Shiva                ShivaCmd                `cmd:"" help:"Download latest payslip from Shiva."`
	LCLChecking          LCLCheckingCmd          `cmd:"" help:"Download latest bank statement from LCL."`
}

func main() {
	var cli Cli
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{OutputDir: cli.OutputDir, Headless: cli.Headless, NoInteraction: cli.NoInteraction})
	ctx.FatalIfErrorf(err)
}

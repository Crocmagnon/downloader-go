package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/Crocmagnon/downloader-go/internal/eaudugrandlyon"
	"github.com/Crocmagnon/downloader-go/internal/freebox"
	"github.com/Crocmagnon/downloader-go/internal/freemobile"
	"github.com/Crocmagnon/downloader-go/internal/lclchecking"
	"github.com/Crocmagnon/downloader-go/internal/octopusenergyaddress"
	"github.com/Crocmagnon/downloader-go/internal/photos"
	"github.com/Crocmagnon/downloader-go/internal/recettescabas"
	"github.com/Crocmagnon/downloader-go/internal/shiva"
	"github.com/Crocmagnon/downloader-go/internal/zeplug"
	"github.com/alecthomas/kong"
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
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running Freebox...")
	return freebox.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type FreeMobileCmd struct {
	Username string `required:"" short:"u" help:"Free mobile username"`
	Password string `required:"" short:"p" help:"Free mobile password"`
}

func (r *FreeMobileCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running FreeMobile...")
	return freemobile.Run(os.Stdout, os.Stderr, os.Stdin, ctx.Headless, r.Username, r.Password, ctx.OutputDir, ctx.NoInteraction)
}

type EauDuGrandLyonCmd struct {
	Username string `required:"" short:"u" help:"Eau du Grand Lyon username"`
	Password string `required:"" short:"p" help:"Eau du Grand Lyon password"`
}

func (r *EauDuGrandLyonCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running EauDuGrandLyon...")
	return eaudugrandlyon.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type OctopusEnergyAddressCmd struct {
	Username string `required:"" short:"u" help:"Octopus Energy username"`
	Password string `required:"" short:"p" help:"Octopus Energy password"`
}

func (r *OctopusEnergyAddressCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running OctopusEnergyAddress...")
	return octopusenergyaddress.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type ShivaCmd struct {
	Username string `required:"" short:"u" help:"Shiva username"`
	Password string `required:"" short:"p" help:"Shiva password"`
}

func (r *ShivaCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running Shiva...")
	return shiva.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type LCLCheckingCmd struct {
	Username string `required:"" short:"u" help:"LCL username"`
	Password string `required:"" short:"p" help:"LCL password"`
}

func (r *LCLCheckingCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running LCLChecking...")
	return lclchecking.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type PhotosCmd struct {
	URL string `required:"" short:"u" help:"Photo library URL"`
}

func (r *PhotosCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running Photos...")
	return photos.Run(os.Stdout, os.Stderr, ctx.Headless, r.URL, ctx.OutputDir)
}

type ZeplugCmd struct {
	Username string `required:"" short:"u" help:"Zeplug username"`
	Password string `required:"" short:"p" help:"Zeplug password"`
}

func (r *ZeplugCmd) Run(ctx *Context) error {
	if err := requireOutputDir(ctx); err != nil {
		return err
	}
	fmt.Println("Running Zeplug...")
	return zeplug.Run(os.Stdout, os.Stderr, ctx.Headless, r.Username, r.Password, ctx.OutputDir)
}

type RecettesCabasFlags struct {
	Username       string `required:"" short:"u" env:"RC_USERNAME" help:"Recettes & Cabas username"`
	Password       string `required:"" short:"p" env:"RC_PASSWORD" help:"Recettes & Cabas password"`
	Config         string `required:"" type:"existingfile" help:"JSON config file (flags and excluded options)."`
	TelegramToken  string `required:"" env:"TELEGRAM_TOKEN" help:"Telegram bot token"`
	TelegramChatID int64  `required:"" env:"TELEGRAM_CHAT_ID" help:"The only Telegram chat the bot talks to"`
	HAWebhook      string `env:"RC_HA_WEBHOOK_URL" help:"Home Assistant webhook URL the week is pushed to after each read of the site."`
}

func (f RecettesCabasFlags) service(ctx *Context) (recettescabas.Service, error) {
	return recettescabas.NewService(os.Stdout, os.Stderr, ctx.Headless, f.Username, f.Password, f.Config, f.TelegramToken, f.TelegramChatID, f.HAWebhook)
}

type RecettesCabasRecapCmd struct {
	RecettesCabasFlags `embed:""`
}

func (r *RecettesCabasRecapCmd) Run(ctx *Context) error {
	svc, err := r.service(ctx)
	if err != nil {
		return err
	}
	return svc.SendRecap(context.Background())
}

type RecettesCabasBotCmd struct {
	RecettesCabasFlags `embed:""`
	APIListen          string `help:"Address of the API Home Assistant calls, e.g. 127.0.0.1:8789."`
	APIToken           string `env:"RC_API_TOKEN" help:"Bearer token the API requires."`
}

func (r *RecettesCabasBotCmd) Run(ctx *Context) error {
	bridge, err := recettescabas.CheckBridge(r.APIListen, r.APIToken, r.HAWebhook)
	if err != nil {
		return err
	}
	svc, err := r.service(ctx)
	if err != nil {
		return err
	}
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if bridge {
		if err := svc.StartAPI(sigCtx, r.APIListen, r.APIToken); err != nil {
			return err
		}
	}
	return svc.Run(sigCtx)
}

type RecettesCabasCmd struct {
	Recap RecettesCabasRecapCmd `cmd:"" help:"Send the weekly recap to Telegram."`
	Bot   RecettesCabasBotCmd   `cmd:"" help:"Run the Telegram bot that applies selection changes."`
}

type VersionCmd struct{}

func (v *VersionCmd) Run(_ *Context) error {
	info, ok := debug.ReadBuildInfo()
	var goVersion, gitCommit, buildDate, commitDirty string

	if ok && info != nil {
		goVersion = info.GoVersion
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				gitCommit = s.Value
			case "vcs.time":
				buildDate = s.Value
			case "vcs.modified":
				// true if the VCS had local modifications at build time
				commitDirty = s.Value
			}
		}
	}

	if goVersion == "" {
		goVersion = runtime.Version()
	}
	if gitCommit == "" {
		gitCommit = "unknown"
	}
	if buildDate == "" {
		buildDate = "unknown"
	}
	if commitDirty == "" {
		commitDirty = "unknown"
	}

	// Print version information
	fmt.Printf("go: %s\ncommit: %s\ndate: %s\ndirty: %s\n", goVersion, gitCommit, buildDate, commitDirty)
	return nil
}

type Cli struct {
	OutputDir     string `help:"Output directory." short:"o" type:"path"`
	Headless      bool   `help:"Enable headless mode."`
	NoInteraction bool   `help:"Enable interaction-less mode. In this mode, if a user interaction is required, it will generate an error instead."`

	Freebox              FreeboxCmd              `cmd:"" help:"Download latest invoice from Freebox."`
	FreeMobile           FreeMobileCmd           `cmd:"" help:"Download latest invoice from Free mobile."`
	EauDuGrandLyon       EauDuGrandLyonCmd       `cmd:"" help:"Download latest invoice from Eau du Grand Lyon."`
	OctopusEnergyAddress OctopusEnergyAddressCmd `cmd:"" help:"Download latest proof of address from Octopus Energy."`
	Shiva                ShivaCmd                `cmd:"" help:"Download latest payslip from Shiva."`
	LCLChecking          LCLCheckingCmd          `cmd:"" help:"Download latest bank statement from LCL."`
	Zeplug               ZeplugCmd               `cmd:"" help:"Download latest invoice from Zeplug."`
	RecettesCabas        RecettesCabasCmd        `cmd:"" help:"Recettes & Cabas weekly recap and Telegram bot."`
	Photos               PhotosCmd               `cmd:"" help:"Download photos."`
	Version              VersionCmd              `cmd:"" help:"Print version information."`
}

func requireOutputDir(ctx *Context) error {
	if ctx == nil || ctx.OutputDir == "" {
		return fmt.Errorf("missing required --output-dir (-o): this command needs an output directory")
	}
	return nil
}

func main() {
	var cli Cli
	ctx := kong.Parse(&cli)
	err := ctx.Run(&Context{OutputDir: cli.OutputDir, Headless: cli.Headless, NoInteraction: cli.NoInteraction})
	ctx.FatalIfErrorf(err)
}

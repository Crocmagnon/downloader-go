/*
Copyright © 2024 Gabriel Augendre <gabriel@augendre.info>
*/
package cmd

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"github.com/spf13/viper"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	freeboxUsername string
	freeboxPassword string
)

// freeboxCmd represents the freebox command
var freeboxCmd = &cobra.Command{
	Use:   "freebox",
	Short: "Download latest Freebox invoice",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(os.Stdout, os.Stderr, freeboxUsername, freeboxPassword, globalOutputDir, globalHeadless); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(freeboxCmd)

	freeboxCmd.Flags().StringVarP(&freeboxUsername, "username", "u", "", "Username")
	_ = freeboxCmd.MarkFlagRequired("username")
	freeboxCmd.Flags().StringVarP(&freeboxPassword, "password", "p", "", "Password")
	_ = freeboxCmd.MarkFlagRequired("password")
	_ = viper.BindPFlags(freeboxCmd.Flags())
}

func run(stdout io.Writer, stderr io.Writer, username, password, dir string, headless bool) error {
	err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"firefox"},
		Stdout:   stdout,
		Stderr:   stderr,
	})
	if err != nil {
		return fmt.Errorf("installing playwright: %w", err)
	}

	playw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("launching playwright: %w", err)
	}

	defer playw.Stop() //nolint:errcheck

	browser, err := playw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		return fmt.Errorf("launching Firefox: %w", err)
	}

	defer browser.Close()

	context, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("creating context: %w", err)
	}

	defer context.Close()

	page, err := context.NewPage()
	if err != nil {
		return fmt.Errorf("creating page: %w", err)
	}

	defer page.Close()

	if err := downloadFile(page, username, password, dir); err != nil {
		return err
	}

	return nil
}

func downloadFile(page playwright.Page, identifier, password, outputDir string) error {
	if err := login(page, identifier, password); err != nil {
		return fmt.Errorf("logging in: %w", err)
	}

	if err := downloadAndSave(page, outputDir); err != nil {
		return fmt.Errorf("downloading and saving: %w", err)
	}

	return nil
}

func login(page playwright.Page, identifier, password string) error {
	_, err := page.Goto("https://subscribe.free.fr/login/")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	if err := page.Locator("#login_b").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("#pass_b").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("#ok").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	download, err := page.ExpectDownload(func() error {
		return page.Locator("#widget_mesfactures .btn_download").First().Click()
	})
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	if err := download.SaveAs(outputDir + "/" + download.SuggestedFilename()); err != nil {
		return fmt.Errorf("saving download file: %w", err)
	}

	return nil
}

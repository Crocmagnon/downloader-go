package eaudugrandlyon

import (
	"errors"
	"fmt"
	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/playwright-community/playwright-go"
	"io"
)

var errNotImplemented = errors.New("not implemented")

func Run(stdout, stderr io.Writer, headless bool, username, password, dir string) error {
	return pw.Run(stdout, stderr, headless, pw.BrowserFirefox, func(page playwright.Page) error {
		return downloadFile(page, username, password, dir)
	})
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
	_, err := page.Goto("https://agence.eaudugrandlyon.com/#/login")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	err = page.WaitForURL("https://agence.eaudugrandlyon.com/#/tableau-de-bord", playwright.PageWaitForURLOptions{Timeout: playwright.Float(2000)})
	if nil == err {
		return nil // already logged in
	}

	if err := page.Locator("input[type=email]").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("input[type=password]").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("button[type=submit]").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	if _, err := page.Goto("https://agence.eaudugrandlyon.com/#/factures"); err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	return fmt.Errorf("%w: no invoice available when developing", errNotImplemented)

	//return pw.Download(page, outputDir, func() error {
	//	return page.Locator(".facture-access").First().Click()
	//})
}

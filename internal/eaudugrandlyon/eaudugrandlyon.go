package eaudugrandlyon

import (
	"fmt"
	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/playwright-community/playwright-go"
	"io"
)

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
	_, err := page.Goto("https://www.eaudugrandlyon.com/default.aspx")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	err = page.WaitForURL("https://www.eaudugrandlyon.com/default-connected.aspx", playwright.PageWaitForURLOptions{Timeout: playwright.Float(2000)})
	if nil == err {
		return nil // already logged in
	}

	if err := page.Locator("#login").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("input[type=password]").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("input[type=submit][name=connect]").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	if _, err := page.Goto("https://www.eaudugrandlyon.com/mon-espace-compte-consulter-facture.aspx"); err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	return pw.Download(page, outputDir, func() error {
		return page.Locator(".facture-access").First().Click()
	})
}

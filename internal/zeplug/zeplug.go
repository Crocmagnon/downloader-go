package zeplug

import (
	"fmt"
	"io"

	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/mxschmitt/playwright-go"
)

func Run(stdout, stderr io.Writer, headless bool, username, password, dir string) error {
	return pw.Run(stdout, stderr, headless, pw.BrowserDefault, func(page playwright.Page) error {
		return downloadFile(page, username, password, dir)
	})
}

func downloadFile(page playwright.Page, username, password, outputDir string) error {
	if err := login(page, username, password); err != nil {
		return fmt.Errorf("logging in: %w", err)
	}

	if err := downloadAndSave(page, outputDir); err != nil {
		return fmt.Errorf("downloading and saving: %w", err)
	}

	return nil
}

func login(page playwright.Page, username, password string) error {
	_, err := page.Goto("https://customer.zeplug.com/")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	err = page.WaitForURL("**/login", playwright.PageWaitForURLOptions{Timeout: playwright.Float(10000)})
	if err != nil {
		return nil // not redirected to login, already logged in
	}

	if err := page.Locator("input#email").Fill(username); err != nil {
		return fmt.Errorf("typing username: %w", err)
	}

	if err := page.Locator("input#password").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("button[type=submit]").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	if err := page.Locator("input#email").WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateDetached,
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("waiting for login to complete: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	if _, err := page.Goto("https://customer.zeplug.com/invoices"); err != nil {
		return fmt.Errorf("going to invoices: %w", err)
	}

	return pw.Download(page, outputDir, func() error {
		return page.Locator("button").Nth(1).Click()
	})
}

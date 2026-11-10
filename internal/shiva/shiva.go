package shiva

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
	_, err := page.Goto("https://portail.shiva.fr/clients")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	if err := page.WaitForURL("https://login.shiva.fr/login?*", playwright.PageWaitForURLOptions{Timeout: playwright.Float(5000)}); err != nil {
		if page.URL() == "https://portail.shiva.fr/clients" {
			return nil // already logged in
		}

		return fmt.Errorf("waiting for redirect to login page: %w", err)
	}

	// The default login mode sends a one-time code by email.
	if err := page.GetByText("Me connecter avec mon mot de passe").Click(); err != nil {
		return fmt.Errorf("switching to password login: %w", err)
	}

	if err := page.Locator("#username").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("#password").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("form button[type=submit]").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	if err := page.WaitForURL("https://portail.shiva.fr/clients", playwright.PageWaitForURLOptions{Timeout: playwright.Float(15000)}); err != nil {
		return fmt.Errorf("waiting for redirect: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	_, err := page.Goto("https://portail.shiva.fr/clients/mes-intervenants")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	// Nth(1) because there's a hidden button with the same selector displayed on mobile only.
	return pw.Download(page, outputDir, func() error {
		return page.Locator("tr .button-btn-download").Nth(1).Click()
	})
}

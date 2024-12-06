package shiva

import (
	"fmt"
	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/playwright-community/playwright-go"
	"io"
)

func Run(stdout, stderr io.Writer, headless bool, username, password, dir string) error {
	return pw.Run(stdout, stderr, headless, func(page playwright.Page) error {
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
	_, err := page.Goto("https://connect.shiva.fr/Account/Login")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	if err := page.Locator("#identifiantCtrl").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("#motDePasseCtrl").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("#connexion").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	if err := page.WaitForURL("https://portail.shiva.fr/clients", playwright.PageWaitForURLOptions{Timeout: playwright.Float(5000)}); err != nil {
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
	download, err := page.ExpectDownload(func() error {
		return page.Locator("tr .button-btn-download").Nth(1).Click()
	})
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	if err := download.SaveAs(outputDir + "/" + download.SuggestedFilename()); err != nil {
		return fmt.Errorf("saving download file: %w", err)
	}

	return nil
}

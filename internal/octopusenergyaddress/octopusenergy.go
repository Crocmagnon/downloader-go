package octopusenergyaddress

import (
	"fmt"
	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/playwright-community/playwright-go"
	"io"
	"regexp"
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
	_, err := page.Goto("https://www.octopusenergy.fr/connexion")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	_ = page.Locator("#didomi-notice-disagree-button").Click()

	if err := page.Locator("input[name=email]").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("input[name=password]").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("button[type=submit]").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	reg := regexp.MustCompile(`^https://www\.octopusenergy\.fr/espace-client/comptes/.*/logements/.*$`)
	if err := page.WaitForURL(reg, playwright.PageWaitForURLOptions{Timeout: playwright.Float(5000)}); err != nil {
		return fmt.Errorf("waiting for redirect: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	_, err := page.Goto(page.URL() + "/justificatif-de-domicile")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	download, err := page.ExpectDownload(func() error {
		return page.Locator("button[type=submit]").First().Click()
	})
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	if err := download.SaveAs(outputDir + "/" + download.SuggestedFilename()); err != nil {
		return fmt.Errorf("saving download file: %w", err)
	}

	return nil
}
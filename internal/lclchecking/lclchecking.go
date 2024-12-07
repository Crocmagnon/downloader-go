package lclchecking

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
	_, err := page.Goto("https://monespace.lcl.fr/connexion")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	// we don't care about this error, if the privacy policy is not there no need to reject
	_ = page.Locator("#popin_tc_privacy_button_2").Click(playwright.LocatorClickOptions{Timeout: playwright.Float(5000)})

	if err := page.Locator("#identifier").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator(".app-cta-button").First().Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	for _, char := range password {
		if err := page.Locator(fmt.Sprintf(".pad-button[value='%s']", string(char))).Click(); err != nil {
			return fmt.Errorf("clicking pad button: %w", err)
		}
	}

	if err := page.Locator(".app-cta-button").First().Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	err = page.WaitForURL("https://monespace.lcl.fr/synthese/compte", playwright.PageWaitForURLOptions{Timeout: playwright.Float(30000)})
	if err != nil {
		return fmt.Errorf("waiting for redirect: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	if _, err := page.Goto("https://monespace.lcl.fr/mes-documents/releves-de-compte-de-depot"); err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	return pw.Download(page, outputDir, func() error {
		return page.Locator("button.amount").First().Click()
	})
}

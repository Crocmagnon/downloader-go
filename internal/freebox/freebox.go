package freebox

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
	err := page.Context().Route("https://adsl.free.fr/facture_pdf.pl*", func(route playwright.Route) {
		resp, err := route.Fetch()
		if err != nil {
			_ = route.Continue()
			return
		}

		headers := resp.Headers()
		headers["content-disposition"] = `attachment;filename="facture_freebox.pdf"`
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Response: resp,
			Headers:  headers,
		})
	})
	if err != nil {
		return fmt.Errorf("setting up download route: %w", err)
	}

	return pw.Download(page, outputDir, func() error {
		return page.Locator("#widget_mesfactures .btn_download").First().Click()
	})
}

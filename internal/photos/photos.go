package photos

import (
	"context"
	"fmt"
	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/carlmjohnson/requests"
	"github.com/playwright-community/playwright-go"
	"io"
	"os"
	"path/filepath"
	"time"
)

func Run(stdout, stderr io.Writer, headless bool, dir string) error {
	return pw.Run(stdout, stderr, headless, pw.BrowserFirefox, func(page playwright.Page) error {
		return downloadFile(page, dir)
	})
}

func downloadFile(page playwright.Page, outputDir string) error {
	if err := downloadAndSave(page, outputDir); err != nil {
		return fmt.Errorf("downloading and saving: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	_, err := page.Goto("https://lumys-photo-library.example.com")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	if err := page.Locator("img").First().Click(); err != nil {
		return fmt.Errorf("clicking on first img: %w", err)
	}

	seen := make(map[string]struct{})

	for {
		filename, err := page.Locator(".photo-name").TextContent()
		if err != nil {
			return fmt.Errorf("getting filename: %w", err)
		}

		if _, ok := seen[filename]; ok {
			break
		}

		seen[filename] = struct{}{}

		src, err := page.Locator(".scroll img").First().GetAttribute("src")
		if err != nil {
			return fmt.Errorf("getting src attribute: %w", err)
		}

		f, err := os.OpenFile(filepath.Join(outputDir, filename), os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}

		err = requests.URL(src).
			ToWriter(f).
			Fetch(context.Background())
		if err != nil {
			return fmt.Errorf("fetching %s: %w", filename, err)
		}

		if err := page.Locator("[data-test=photoViewerNext]").First().Click(); err != nil {
			return fmt.Errorf("clicking on next page: %w", err)
		}

		time.Sleep(time.Second)
	}

	return nil
}

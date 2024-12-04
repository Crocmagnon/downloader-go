// Package pw provides utility functions to work with Playwright.
package pw

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"io"
)

// Run runs callback in a playwright context, handling resource (de)allocation.
func Run(stdout, stderr io.Writer, headless bool, callback func(playwright.Page) error) error {
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

	if err := callback(page); err != nil {
		return fmt.Errorf("callback: %w", err)
	}

	return nil
}

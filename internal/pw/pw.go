// Package pw provides utility functions to work with Playwright.
package pw

import (
	"encoding/json"
	"fmt"
	"github.com/playwright-community/playwright-go"
	"io"
	"os"
	"path/filepath"
)

type Browser int

const (
	BrowserChromium Browser = iota
	BrowserFirefox
)

const cookieFileName = "cookies.json"

// Run runs callback in a playwright context, handling resource (de)allocation.
func Run(stdout, stderr io.Writer, headless bool, brwsr Browser, callback func(playwright.Page) error) error {
	options := &playwright.RunOptions{
		Browsers: []string{"firefox", "chromium"},
		Stdout:   stdout,
		Stderr:   stderr,
	}
	err := playwright.Install(options)
	if err != nil {
		return fmt.Errorf("installing playwright: %w", err)
	}

	playw, err := playwright.Run(options)
	if err != nil {
		return fmt.Errorf("launching playwright: %w", err)
	}

	defer playw.Stop() //nolint:errcheck

	var browserType playwright.BrowserType

	switch brwsr {
	case BrowserChromium:
		browserType = playw.Chromium
	case BrowserFirefox:
		browserType = playw.Firefox
	default:
		browserType = playw.Firefox
	}

	browser, err := browserType.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		return fmt.Errorf("launching browser: %w", err)
	}

	defer browser.Close()

	context, err := browser.NewContext(playwright.BrowserNewContextOptions{})
	if err != nil {
		return fmt.Errorf("creating context: %w", err)
	}

	defer context.Close()

	if err := loadCookies(context, cookieFileName); err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to load cookies, continuing anyway: %v\n", err)
	}

	page, err := context.NewPage()
	if err != nil {
		return fmt.Errorf("creating page: %w", err)
	}

	defer page.Close()

	if err := callback(page); err != nil {
		saveScreenshot(page, "screenshots")
		return err
	}

	if err := saveCookies(context, cookieFileName); err != nil {
		return fmt.Errorf("saving cookies: %w", err)
	}

	return nil
}

func saveCookies(context playwright.BrowserContext, filename string) error {
	cookies, err := context.Cookies()
	if err != nil {
		return fmt.Errorf("getting cookies: %w", err)
	}

	optCookies := make([]playwright.OptionalCookie, 0, len(cookies))

	for _, cookie := range cookies {
		optCookies = append(optCookies, cookie.ToOptionalCookie())
	}

	asJSON, err := json.Marshal(optCookies)
	if err != nil {
		return fmt.Errorf("marshaling cookies: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating %s: %w", filename, err)
	}

	defer file.Close()

	if _, err := file.Write(asJSON); err != nil {
		return fmt.Errorf("writing cookies.json: %w", err)
	}

	return nil
}

func loadCookies(context playwright.BrowserContext, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filename, err)
	}

	defer file.Close()

	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}

	var cookies []playwright.OptionalCookie
	if err := json.Unmarshal(fileContent, &cookies); err != nil {
		return fmt.Errorf("unmarshaling cookies: %w", err)
	}

	if err := context.AddCookies(cookies); err != nil {
		return fmt.Errorf("adding cookies: %w", err)
	}

	return nil
}

func saveScreenshot(page playwright.Page, dir string) {
	img, err := page.Screenshot()
	if err != nil {
		return
	}

	const perm = 0o755
	_ = os.MkdirAll(dir, perm)

	file, err := os.Create(filepath.Join(dir, "screenshot.png"))
	if err != nil {
		return
	}

	defer file.Close()
	_, _ = file.Write(img)
}

func DownloadPDFPopup(page playwright.Page, outputDir, url, filename string, triggerPopup func() error) error {
	err := page.Context().Route(url, func(route playwright.Route) {
		resp, err := route.Fetch()
		if err != nil {
			return
		}

		headers := resp.Headers()
		headers["content-disposition"] = fmt.Sprintf(`attachment;filename="%s"`, filename)
		_ = route.Fulfill(playwright.RouteFulfillOptions{
			Response: resp,
			Headers:  headers,
		})
	})
	if err != nil {
		return fmt.Errorf("setting up download route: %w", err)
	}

	popup, err := page.ExpectPopup(triggerPopup)
	if err != nil {
		return fmt.Errorf("opening popup: %w", err)
	}

	if err := Download(popup, outputDir, func() error { return nil }); err != nil {
		return err
	}

	return nil
}

func Download(page playwright.Page, outputDir string, trigger func() error) error {
	download, err := page.ExpectDownload(trigger)
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	if err := download.SaveAs(outputDir + "/" + download.SuggestedFilename()); err != nil {
		return fmt.Errorf("saving file: %w", err)
	}

	return nil
}

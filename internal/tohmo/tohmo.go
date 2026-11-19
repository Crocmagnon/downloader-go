package tohmo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/mxschmitt/playwright-go"
)

const baseURL = "https://app.go-tohmo.com"

// The API rejects requests without the tenant header the web app sends.
var apiHeaders = map[string]string{"X-Tenant-Slug": "tohmo"}

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
	if _, err := page.Goto(baseURL + "/"); err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	if !strings.Contains(page.URL(), "/sign-in") {
		return nil // not redirected to sign-in, already logged in
	}

	if err := page.Locator("input[name=email]").Fill(username); err != nil {
		return fmt.Errorf("typing username: %w", err)
	}

	if err := page.Locator("input[name=password]").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("button[type=submit]").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	if err := page.Locator("input[name=email]").WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateDetached,
		Timeout: playwright.Float(10000),
	}); err != nil {
		return fmt.Errorf("waiting for login to complete: %w", err)
	}

	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	list, err := get(page, "/api/invoices")
	if err != nil {
		return fmt.Errorf("listing invoices: %w", err)
	}

	id, err := latestInvoiceID(list)
	if err != nil {
		return err
	}

	resp, err := page.Request().Get(baseURL+"/api/invoices/"+id+"/pdf", playwright.APIRequestContextGetOptions{Headers: apiHeaders})
	if err != nil {
		return fmt.Errorf("requesting pdf: %w", err)
	}

	if !resp.Ok() {
		return fmt.Errorf("requesting pdf: status %d", resp.Status())
	}

	pdf, err := resp.Body()
	if err != nil {
		return fmt.Errorf("reading pdf: %w", err)
	}

	name := "tohmo-" + id + ".pdf"
	if _, params, err := mime.ParseMediaType(resp.Headers()["content-disposition"]); err == nil && params["filename"] != "" {
		name = filepath.Base(params["filename"])
	}

	if err := os.WriteFile(filepath.Join(outputDir, name), pdf, 0o600); err != nil {
		return fmt.Errorf("saving pdf: %w", err)
	}

	return nil
}

func get(page playwright.Page, path string) ([]byte, error) {
	resp, err := page.Request().Get(baseURL+path, playwright.APIRequestContextGetOptions{Headers: apiHeaders})
	if err != nil {
		return nil, err
	}

	body, err := resp.Body()
	if err != nil {
		return nil, err
	}

	if !resp.Ok() {
		return nil, fmt.Errorf("status %d: %s", resp.Status(), body)
	}

	return body, nil
}

// latestInvoiceID returns the id of the first invoice in the list response.
// shortcut: the response shape wasn't visible without an account, so this accepts a bare array or
// the usual wrapper keys, and trusts the API to sort newest first like the UI. Pin it once seen.
func latestInvoiceID(body []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	var raw any
	if err := dec.Decode(&raw); err != nil {
		return "", fmt.Errorf("decoding invoices: %w", err)
	}

	items, _ := raw.([]any)
	if obj, ok := raw.(map[string]any); ok {
		for _, key := range []string{"data", "items", "results", "invoices"} {
			if arr, ok := obj[key].([]any); ok {
				items = arr
				break
			}
		}
	}

	if len(items) == 0 {
		return "", errors.New("no invoice found in: " + string(body))
	}

	first, _ := items[0].(map[string]any)
	if id, ok := first["id"]; ok && id != nil {
		return fmt.Sprint(id), nil
	}

	return "", errors.New("no id on first invoice: " + string(body))
}

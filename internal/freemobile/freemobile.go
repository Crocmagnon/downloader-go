package freemobile

import (
	"errors"
	"fmt"
	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/playwright-community/playwright-go"
	"io"
)

var (
	ErrInteractionRequired = errors.New("interaction is required")
	ErrInvalidMFA          = errors.New("invalid mfa")
)

func Run(stdout, stderr io.Writer, stdin io.Reader, headless bool, username, password, dir string, noInteraction bool) error {
	return pw.Run(stdout, stderr, headless, pw.BrowserFirefox, func(page playwright.Page) error {
		return downloadFile(
			page,
			username,
			password,
			dir,
			noInteraction,
			stdout,
			stdin,
		)
	})
}

func downloadFile(
	page playwright.Page,
	identifier, password, outputDir string,
	noInteraction bool,
	stdout io.Writer,
	stdin io.Reader,
) error {
	if err := login(page, identifier, password, noInteraction, stdout, stdin); err != nil {
		return fmt.Errorf("logging in: %w", err)
	}

	if err := navigate(page); err != nil {
		return fmt.Errorf("navigating: %w", err)
	}

	if err := downloadAndSave(page, outputDir); err != nil {
		return fmt.Errorf("downloading and saving: %w", err)
	}

	return nil
}

func login(page playwright.Page, identifier, password string, noInteraction bool, stdout io.Writer, stdin io.Reader) error {
	_, err := page.Goto("https://mobile.free.fr/account/v2/login/")
	if err != nil {
		return fmt.Errorf("going to: %w", err)
	}

	err = page.WaitForURL("https://mobile.free.fr/account/v2", playwright.PageWaitForURLOptions{Timeout: playwright.Float(2000)})
	if nil == err {
		return nil // already logged in
	}

	if err := page.Locator("#login-username").Fill(identifier); err != nil {
		return fmt.Errorf("typing identifier: %w", err)
	}

	if err := page.Locator("#login-password").Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}

	if err := page.Locator("#auth-connect").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	if err := handleMFA(page, noInteraction, stdout, stdin); err != nil {
		return fmt.Errorf("handling mfa: %w", err)
	}

	return nil
}

func handleMFA(page playwright.Page, noInteraction bool, stdout io.Writer, stdin io.Reader) error {
	mfaLoginValidate := page.Locator("#auth-2FA-validate")
	if err := playwright.NewPlaywrightAssertions().Locator(mfaLoginValidate).ToBeVisible(); err != nil {
		// no need for 2FA
		return nil
	}

	if noInteraction {
		return ErrInteractionRequired
	}

	_, _ = fmt.Fprint(stdout, "2FA code: ")

	var mfa string

	_, err := fmt.Fscanln(stdin, &mfa)
	if err != nil {
		return fmt.Errorf("reading 2FA code from input: %w", err)
	}

	if len(mfa) != 6 {
		return fmt.Errorf("%w, expected len 6, got %d", ErrInvalidMFA, len(mfa))
	}

	inputs := page.Locator("input[type=number]")

	for i, char := range mfa {
		if err := inputs.Nth(i).Fill(string(char)); err != nil {
			return fmt.Errorf("filling %dth input: %w", i, err)
		}
	}

	// remember me
	if err := page.Locator("span[role=checkbox]").Click(); err != nil {
		return fmt.Errorf("clicking remember me: %w", err)
	}

	if err := mfaLoginValidate.Click(); err != nil {
		return fmt.Errorf("validating mfa: %w", err)
	}
	return nil
}

func navigate(page playwright.Page) error {
	if err := page.Locator("[role=tablist] button").Nth(1).Click(); err != nil {
		return fmt.Errorf("clicking on invoices tab: %w", err)
	}
	return nil
}

func downloadAndSave(page playwright.Page, outputDir string) error {
	return pw.Download(page, outputDir, func() error {
		return page.Locator("[download]").First().Click()
	})
}

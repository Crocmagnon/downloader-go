package recettescabas

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/mxschmitt/playwright-go"
)

const (
	prefURL          = "https://www.recettesetcabas.com/espace-client/commandes-planifiees/modifier"
	prefFormSelector = `form[action*="validerPreference"]`
	waitTimeoutMs    = 15000
	cookieTimeoutMs  = 3000
	priceTimeoutMs   = 5000
)

// The checkboxes are display:none, so Playwright's SetChecked can't be used.
const setSelectionJS = `(changes) => {
  for (const [id, checked] of Object.entries(changes)) {
    const input = document.getElementById(id);
    if (!input) throw new Error('missing input ' + id);
    input.checked = checked;
  }
}`

// Native submit: bypasses the site's change/submit handlers, which could flip
// checkboxes back or keep the button disabled. Other fields are posted as-is.
const submitJS = `() => document.querySelector('` + prefFormSelector + `').submit()`

func NewService(stdout, stderr io.Writer, headless bool, username, password, configPath, haWebhook string) (Service, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return Service{}, err
	}

	browse := func(fn func(playwright.Page) error) error {
		return pw.Run(stdout, stderr, headless, pw.BrowserDefault, fn)
	}

	return Service{
		Config:    cfg,
		Log:       stderr,
		HAWebhook: haWebhook,
		site:      &sync.Mutex{},
		Fetch: func(context.Context) (Page, error) {
			var p Page
			err := browse(func(page playwright.Page) error {
				var err error
				p, err = load(page, username, password)
				return err
			})
			return p, err
		},
		Apply: func(_ context.Context, week string, desired map[int]bool) (Page, error) {
			var p Page
			err := browse(func(page playwright.Page) error {
				var err error
				p, err = apply(page, username, password, week, desired)
				return err
			})
			return p, err
		},
	}, nil
}

// login goes to the preferences page and logs in if the site redirected to the login form.
func login(page playwright.Page, username, password string) error {
	if _, err := page.Goto(prefURL); err != nil {
		return fmt.Errorf("going to preferences: %w", err)
	}

	if n, err := page.Locator(prefFormSelector).Count(); err == nil && n > 0 {
		return nil // already logged in thanks to cookies
	}

	// The cookie banner covers the login form; it may already be dismissed.
	_ = page.GetByText("Continuer sans accepter").Click(playwright.LocatorClickOptions{Timeout: playwright.Float(cookieTimeoutMs)})

	email := page.Locator("#cli_email:visible") // a hidden copy exists on the page
	if err := fill(email, username, "username"); err != nil {
		return err
	}

	if err := fill(page.Locator("#cli_mdp:visible"), password, "password"); err != nil {
		return err
	}

	if err := page.Locator("#button-connexion-1").Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
	}

	if err := email.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateDetached,
		Timeout: playwright.Float(waitTimeoutMs),
	}); err != nil {
		return fmt.Errorf("waiting for login to complete: %w", err)
	}

	return nil
}

// fill types value into l. Playwright's error call log echoes the typed value,
// so the error is replaced: credentials must never reach logs or Home Assistant.
func fill(l playwright.Locator, value, what string) error {
	if err := l.Fill(value); err != nil {
		return fmt.Errorf("typing %s failed (see screenshot)", what)
	}

	return nil
}

func load(page playwright.Page, username, password string) (Page, error) {
	if err := login(page, username, password); err != nil {
		return Page{}, fmt.Errorf("logging in: %w", err)
	}

	if _, err := page.Goto(prefURL); err != nil {
		return Page{}, fmt.Errorf("going to preferences: %w", err)
	}

	if err := page.Locator(prefFormSelector).WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateAttached,
		Timeout: playwright.Float(waitTimeoutMs),
	}); err != nil {
		return Page{}, fmt.Errorf("waiting for preferences form: %w", err)
	}

	// The site fills the price with a request after load. If it's late, the recap has no price.
	_, _ = page.WaitForFunction(`() => document.querySelector('#js_total_prix')?.textContent.trim()`, nil,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(priceTimeoutMs)})

	return Extract(page)
}

func setSelection(page playwright.Page, changes map[string]bool) error {
	arg := make(map[string]any, len(changes)) // playwright-go only serializes map[string]any
	for id, checked := range changes {
		arg[id] = checked
	}

	if _, err := page.Evaluate(setSelectionJS, arg); err != nil {
		return fmt.Errorf("setting selection: %w", err)
	}

	return nil
}

func apply(page playwright.Page, username, password, week string, desired map[int]bool) (Page, error) {
	p, err := load(page, username, password)
	if err != nil {
		return Page{}, err
	}

	if p.Week != week {
		return Page{}, ErrWeekChanged
	}

	changes := map[string]bool{}
	for n, checked := range desired {
		it, ok := find(p.Items, n)
		if !ok {
			return Page{}, fmt.Errorf("item %d not on the page", n)
		}
		changes[it.InputID] = checked
	}

	if err := setSelection(page, changes); err != nil {
		return Page{}, err
	}

	if _, err := page.ExpectResponse("**validerPreference**", func() error {
		_, err := page.Evaluate(submitJS)
		return err
	}); err != nil {
		return Page{}, fmt.Errorf("submitting preferences: %w", err)
	}

	after, err := load(page, username, password)
	if err != nil {
		return Page{}, fmt.Errorf("reloading after submit: %w", err)
	}

	for n, checked := range desired {
		if it, ok := find(after.Items, n); !ok || it.Selected != checked {
			return Page{}, fmt.Errorf("item %d: the site did not save the change", n)
		}
	}

	return after, nil
}

func find(items []Item, n int) (Item, bool) {
	for _, it := range items {
		if it.N == n {
			return it, true
		}
	}

	return Item{}, false
}

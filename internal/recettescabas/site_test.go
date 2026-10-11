package recettescabas

import (
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
)

func TestSetSelection(t *testing.T) {
	withFixture(t, func(page playwright.Page) {
		before, err := Extract(page)
		if err != nil {
			t.Fatal(err)
		}
		r3 := item(t, before, 3)

		if err := setSelection(page, map[string]bool{"rtype_23": false, r3.InputID: true}); err != nil {
			t.Fatal(err)
		}

		after, err := Extract(page)
		if err != nil {
			t.Fatal(err)
		}
		if item(t, after, 2).Selected || !item(t, after, 3).Selected {
			t.Errorf("selection not applied: 2=%v 3=%v", item(t, after, 2).Selected, item(t, after, 3).Selected)
		}

		if err := setSelection(page, map[string]bool{"nope": true}); err == nil {
			t.Error("expected an error for a missing input")
		}
	})
}

func TestFillErrorDoesNotLeakValue(t *testing.T) {
	withFixture(t, func(page playwright.Page) {
		page.SetDefaultTimeout(1000)
		// A hidden checkbox resolves but never becomes editable: Fill times out
		// after Playwright logged fill("<value>") in its call log.
		err := fill(page.Locator("#rtype_23"), "s3cret-value", "password")
		if err == nil {
			t.Fatal("expected an error")
		}
		if strings.Contains(err.Error(), "s3cret-value") {
			t.Errorf("error leaks the filled value: %v", err)
		}
	})
}

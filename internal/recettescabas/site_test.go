package recettescabas

import (
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

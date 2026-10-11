package recettescabas

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/mxschmitt/playwright-go"
)

// withFixture opens testdata/recettes.html in a headless browser.
// It skips when Playwright isn't available (e.g. inside the nix build sandbox).
func withFixture(t *testing.T, fn func(playwright.Page)) {
	t.Helper()

	if os.Getenv("PLAYWRIGHT_DRIVER_PATH") == "" {
		t.Skip("PLAYWRIGHT_DRIVER_PATH not set, run inside `nix develop`")
	}

	fixture, err := filepath.Abs("testdata/recettes.html")
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(t.TempDir()) // pw.Run writes cookies.json in the working directory

	err = pw.Run(io.Discard, io.Discard, true, pw.BrowserDefault, func(page playwright.Page) error {
		if _, err := page.Goto("file://" + fixture); err != nil {
			return err
		}
		fn(page)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func item(t *testing.T, p Page, n int) Item {
	t.Helper()
	for _, it := range p.Items {
		if it.N == n {
			return it
		}
	}
	t.Fatalf("item %d not found", n)
	return Item{}
}

func TestExtract(t *testing.T) {
	withFixture(t, func(page playwright.Page) {
		p, err := Extract(page)
		if err != nil {
			t.Fatal(err)
		}

		if p.Week != "19/10/2026" {
			t.Errorf("Week = %q", p.Week)
		}
		if p.Deadline != "Jeudi 15 Octobre 2026 à 23h00" {
			t.Errorf("Deadline = %q", p.Deadline)
		}
		if p.People != "Panier à cuisiner 4 pers." {
			t.Errorf("People = %q", p.People)
		}
		if p.Price != "" {
			t.Errorf("Price = %q, want empty (filled by JS on the live site)", p.Price)
		}

		var ns, selected []int
		for _, it := range p.Items {
			ns = append(ns, it.N)
			if it.Selected {
				selected = append(selected, it.N)
			}
		}
		if want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}; !slices.Equal(ns, want) {
			t.Errorf("numbers = %v, want %v", ns, want)
		}
		if want := []int{1, 2, 8, 9}; !slices.Equal(selected, want) {
			t.Errorf("selected = %v, want %v", selected, want)
		}

		r2 := item(t, p, 2)
		if r2.Name != "Filet mignon aux pommes - pommes de terre" || r2.Label != "viande 1" || r2.InputID != "rtype_23" || r2.Option {
			t.Errorf("recipe 2 = %+v", r2)
		}
		if !strings.Contains(r2.Ingredients, "pommes à cuire") {
			t.Errorf("recipe 2 ingredients = %q", r2.Ingredients)
		}
		if r4 := item(t, p, 4); !strings.Contains(r4.Ingredients, "champignons de Paris") {
			t.Errorf("recipe 4 ingredients = %q", r4.Ingredients)
		}

		for n, want := range map[int]string{
			12: "Tomme de Savoie ; gorgonzola ; saint Marcellin",
			13: "Poires et raisins",
			15: "Des chouquettes et un pain aux céréales de la Maison Bettant",
		} {
			if got := item(t, p, n).Selection; got != want {
				t.Errorf("option %d selection = %q, want %q", n, got, want)
			}
		}
		if got := item(t, p, 10).Selection; got != "" {
			t.Errorf("option 10 selection = %q, want empty", got)
		}

		o10 := item(t, p, 10)
		if o10.Name != "Tarte fondante aux noix" || o10.Label != "Kit dessert à cuisiner (12.95€)" || !o10.Option || o10.InputID != "opt_6" {
			t.Errorf("option 10 = %+v", o10)
		}
	})
}

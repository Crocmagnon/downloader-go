# Recettes & Cabas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Weekly Telegram recap of the Recettes & Cabas selection with ingredient flags, plus a Telegram bot whose inline buttons stage add/remove changes and apply them on the site.

**Architecture:** New package `internal/recettescabas` in this repo, exposed as `downloader-go recettes-cabas recap|bot`. Page data is extracted with one `page.Evaluate` (the recipe modals are already in the DOM); flags/exclusions come from a JSON config; the bot is stateless (pending changes live in the message's inline keyboard, the week in its text). Deployed on srv-1 via nixos-config.

**Tech Stack:** Go 1.25, `mxschmitt/playwright-go` (via `internal/pw`), `alecthomas/kong`, stdlib `net/http` for the Telegram Bot API.

**Spec:** `docs/superpowers/specs/2026-10-11-recettes-cabas-design.md`

## Global Constraints

- No new Go dependency: `go.mod` and the flake's `vendorHash` stay unchanged.
- No keyword list in Go code: flags and option exclusions come only from the JSON config (`excludeOptions`, `flags[].emoji`, `flags[].keywords`).
- Matching: lowercase + accents stripped, keyword must be a whole word optionally followed by `s` or `x`.
- `flags` match **ingredients only** (never the title); `excludeOptions` match **options only** (name + label).
- Options are numbered 10+ in page order **before** exclusions are applied.
- Secrets/env names: `RC_USERNAME`, `RC_PASSWORD`, `TELEGRAM_TOKEN`, `TELEGRAM_CHAT_ID`.
- Only `TELEGRAM_CHAT_ID` is served; other chats are ignored and their id logged.
- The repo is public: no personal data (name, address, phone) in any committed file, including the plan, fixtures and commit messages.
- User-facing (Telegram) text is French.
- Tests: `go test ./...` (never `-count=1`); lint: `golangci-lint run ./...`. Both must pass before each commit.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- `nix build` runs `go test` in a sandbox without Playwright browsers → browser tests must **skip**, not fail (Task 2, verified with `nix build`).
- A Telegram transport error is a `*url.Error` whose message contains the request URL, i.e. the **bot token** → it must never reach logs or the "❌ échec" message (Task 5 test).
- "Valider" tapped on an old recap after the site rolled over to the next week → nothing applied, user told to `/recap` (Task 6 test, Task 7 code).
- Updates from any other chat → ignored without side effects (Task 6 test).
- Malformed input: `/detail abc`, `/detail 42` (unknown or excluded wine), callback data `t:x` → a usage/unknown reply or a no-op, never a panic (Task 6 tests).

## File Structure

```
internal/recettescabas/
  config.go        Config, Flag, LoadConfig
  match.go         normalize, match (keyword matching rules)
  page.go          Item, Page, Hit, Extract (page.Evaluate), parse
  recap.go         Annotate (exclusions + flags), RecapText, DetailText
  keyboard.go      Button, NewKeyboard, Toggle, Reset, Pending
  telegram.go      Telegram client (getUpdates, sendMessage, editMessageText, answerCallbackQuery)
  bot.go           Service: SendRecap, Run, update handling
  site.go          login, load, apply, NewService (real Fetch/Apply via pw.Run)
  *_test.go
  testdata/recettes.html   sanitized fixture (form block only)
  testdata/config.json     example config
main.go            recettes-cabas recap|bot commands
README.md, CLAUDE.md       provider list
```

---

### Task 1: Config and keyword matching

**Files:**
- Create: `internal/recettescabas/config.go`, `internal/recettescabas/match.go`, `internal/recettescabas/testdata/config.json`
- Test: `internal/recettescabas/config_test.go`, `internal/recettescabas/match_test.go`

**Interfaces:**
- Produces: `type Flag struct{ Emoji string; Keywords []string }`, `type Config struct{ ExcludeOptions []string; Flags []Flag }`, `func LoadConfig(path string) (Config, error)`, `func match(text string, keywords []string) string` (returns the matched word of the normalized text, `""` if none).

- [ ] **Step 1: Write the example config** `internal/recettescabas/testdata/config.json`

```json
{
  "excludeOptions": ["château", "domaine", "vin", "rosé", "IGP", "AOC"],
  "flags": [
    {"emoji": "🍄", "keywords": ["champignon", "cèpe", "girolle", "shiitake", "pleurote", "morille", "trompette"]},
    {"emoji": "🥗", "keywords": ["salade", "laitue", "mâche", "roquette", "mesclun", "batavia", "sucrine", "romaine", "jeunes pousses"]}
  ]
}
```

- [ ] **Step 2: Write the failing tests**

`internal/recettescabas/match_test.go`:

```go
package recettescabas

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		want     string
	}{
		{"boulgour ; Champignons de Paris ; lardons", []string{"champignon"}, "champignons"},
		{"cèpes séchés", []string{"cepe"}, "cepes"},
		{"Cèpes séchés", []string{"cèpe"}, "cepes"},
		{"choux de Bruxelles", []string{"chou"}, "choux"},
		{"vinaigre balsamique", []string{"vin"}, ""},
		{"un verre de vin blanc", []string{"vin"}, "vin"},
		{"saladier en verre", []string{"salade"}, ""},
		{"poireaux ; salade verte", []string{"salade"}, "salade"},
		{"Mâche", []string{"mache"}, "mache"},
		{"jeunes pousses d'épinard", []string{"jeunes pousses"}, "jeunes pousses"},
		{"ŒUFS plein air", []string{"oeuf"}, "oeufs"},
		{"carottes", []string{"", "  "}, ""},
		{"carottes", nil, ""},
	}
	for _, tt := range tests {
		if got := match(tt.text, tt.keywords); got != tt.want {
			t.Errorf("match(%q, %q) = %q, want %q", tt.text, tt.keywords, got, tt.want)
		}
	}
}
```

`internal/recettescabas/config_test.go`:

```go
package recettescabas

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig("testdata/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ExcludeOptions) == 0 || len(cfg.Flags) != 2 || cfg.Flags[0].Emoji != "🍄" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"exclude_options": ["vin"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Error("expected an error for a misspelled field")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/recettescabas/`
Expected: FAIL to compile (`undefined: match`, `undefined: LoadConfig`).

- [ ] **Step 4: Implement** `internal/recettescabas/config.go`

```go
// Package recettescabas sends a weekly Recettes & Cabas recap to Telegram
// and applies the selection changes requested from the Telegram bot.
package recettescabas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Flag marks items whose ingredients contain one of Keywords with Emoji.
type Flag struct {
	Emoji    string   `json:"emoji"`
	Keywords []string `json:"keywords"`
}

type Config struct {
	// ExcludeOptions hides options whose name or label contains one of these keywords.
	ExcludeOptions []string `json:"excludeOptions"`
	Flags          []Flag   `json:"flags"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
```

`internal/recettescabas/match.go`:

```go
package recettescabas

import (
	"regexp"
	"strings"
)

var accents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i",
	"ô", "o", "ö", "o",
	"ù", "u", "û", "u", "ü", "u",
	"ÿ", "y", "ç", "c", "œ", "oe", "æ", "ae",
	" ", " ", "’", "'",
)

func normalize(s string) string {
	return accents.Replace(strings.ToLower(s))
}

// match returns the first word of text matching one of keywords, or "" if none does.
// A keyword matches a whole word, optionally followed by a plural "s" or "x",
// ignoring case and accents. The returned word is normalized.
func match(text string, keywords []string) string {
	text = normalize(text)

	for _, kw := range keywords {
		kw = strings.TrimSpace(normalize(kw))
		if kw == "" {
			continue
		}

		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(kw) + `[sx]?\b`)
		if found := re.FindString(text); found != "" {
			return found
		}
	}

	return ""
}
```

- [ ] **Step 5: Run tests and lint**

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS, no lint issue.

- [ ] **Step 6: Commit**

```bash
git add internal/recettescabas
git commit -m "recettescabas: config and keyword matching

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Sanitized fixture and page extraction

**Files:**
- Create: `internal/recettescabas/testdata/recettes.html`, `internal/recettescabas/page.go`
- Test: `internal/recettescabas/page_test.go`

**Interfaces:**
- Consumes: `pw.Run(stdout, stderr io.Writer, headless bool, brwsr pw.Browser, callback func(playwright.Page) error) error` (writes `cookies.json` in the working directory).
- Produces:

```go
type Hit struct{ Emoji, Word string }
type Item struct {
	N           int    // site's "Recette N" for recipes; 10+ in page order for options
	InputID     string // DOM id of the checkbox, e.g. "rtype_23", "opt_6"
	Name        string
	Label       string // "viande 1" for recipes, "Kit dessert à cuisiner (12.95€)" for options
	Option      bool
	Selected    bool
	Ingredients string
	Flags       []Hit  // set by Annotate (Task 3)
}
type Page struct {
	Week     string // "19/10/2026"
	Deadline string // "Jeudi 15 Octobre 2026 à 23h00"
	People   string // "Panier à cuisiner 4 pers."
	Price    string // "102,25", empty when the site's JS did not fill it
	Items    []Item // sorted by N
}
func Extract(page playwright.Page) (Page, error)
var weekRe *regexp.Regexp // matches "dd/mm/yyyy"
func withFixture(t *testing.T, fn func(playwright.Page)) // test helper, page_test.go
```

- [ ] **Step 1: Build the fixture from the saved page.** The source is `~/Downloads/recettes.html` (the page saved by Gabriel). Keep only the deadline paragraph (lines 372-373) and the form up to the first "Valider" button (lines 385-2158), drop every `<img>` (they point to remote URLs):

```bash
mkdir -p internal/recettescabas/testdata
src=~/Downloads/recettes.html
{
  echo '<!DOCTYPE html><html lang="fr"><head><meta charset="utf-8"></head><body>'
  sed -n '372,373p' "$src"
  sed -n '385,2158p' "$src"
  echo '</form></body></html>'
} | perl -0777 -pe 's/<img\b[^>]*>//gs' > internal/recettescabas/testdata/recettes.html
```

Sanity checks (all must hold, otherwise adjust the line ranges by looking at the source):

```bash
f=internal/recettescabas/testdata/recettes.html
grep -c 'prendront effet' $f           # 1
grep -c 'name="rtype_ids\[\]"' $f      # 9
grep -c 'name="opt_ids\[\]"' $f        # 11
grep -c '<img' $f                      # 0
grep -ci 'adresse principale\|js_chercher_relais' $f   # 0 (delivery block excluded)
```

- [ ] **Step 2: Privacy check.** Open the "Adresse principale" block of `~/Downloads/recettes.html` (after line 2158) to read the account holder's name, street and phone, then `grep -ci` each of them in the fixture: every count must be 0. Do **not** write those values in any file, command history kept in the repo, or commit message. Also skim the fixture for anything account-specific besides the selection itself.

- [ ] **Step 3: Write the failing test** `internal/recettescabas/page_test.go`

```go
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
		if want := []int{2, 4, 8, 9}; !slices.Equal(selected, want) {
			t.Errorf("selected = %v, want %v", selected, want)
		}

		r2 := item(t, p, 2)
		if r2.Name != "Filet mignon aux pommes - pommes de terre" || r2.Label != "viande 1" || r2.InputID != "rtype_23" || r2.Option {
			t.Errorf("recipe 2 = %+v", r2)
		}
		if !strings.Contains(r2.Ingredients, "pommes à cuire") {
			t.Errorf("recipe 2 ingredients = %q", r2.Ingredients)
		}
		if r1 := item(t, p, 1); !strings.Contains(r1.Ingredients, "champignons de Paris") {
			t.Errorf("recipe 1 ingredients = %q", r1.Ingredients)
		}

		o10 := item(t, p, 10)
		if o10.Name != "Tarte fondante aux noix" || o10.Label != "Kit dessert à cuisiner (12.95€)" || !o10.Option || o10.InputID != "opt_6" {
			t.Errorf("option 10 = %+v", o10)
		}
	})
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/recettescabas/ -run TestExtract`
Expected: FAIL to compile (`undefined: Extract`).

- [ ] **Step 5: Implement** `internal/recettescabas/page.go`

```go
package recettescabas

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mxschmitt/playwright-go"
)

// Hit is a flag found in an item's ingredients: the flag's emoji and the matched word.
type Hit struct {
	Emoji string
	Word  string
}

type Item struct {
	N           int    // site's "Recette N" for recipes; 10+ in page order for options
	InputID     string // DOM id of the checkbox, e.g. "rtype_23", "opt_6"
	Name        string
	Label       string
	Option      bool
	Selected    bool
	Ingredients string
	Flags       []Hit // set by Annotate
}

type Page struct {
	Week     string // "19/10/2026"
	Deadline string // "Jeudi 15 Octobre 2026 à 23h00"
	People   string
	Price    string // empty when the site's JS did not fill it
	Items    []Item // sorted by N
}

const firstOptionN = 10

var (
	weekRe   = regexp.MustCompile(`\d{2}/\d{2}/\d{4}`)
	recipeRe = regexp.MustCompile(`(?i)^recette\s+(\d+)\s*-\s*(.*)$`)
)

// extractJS reads the preferences form. Recipe/option details (the modal content)
// are already in the DOM, so nothing needs to be clicked.
const extractJS = `() => {
  const text = el => el ? el.textContent.replace(/\s+/g, ' ').trim() : '';
  const deadline = [...document.querySelectorAll('p')].find(p => p.textContent.includes('prendront effet'));
  return {
    week: text(document.querySelector('.box_recettes h3')),
    deadline: text(deadline && deadline.querySelector('strong')),
    people: text(document.querySelector('#art_id option:checked')),
    price: text(document.querySelector('#js_cabas_prix')),
    items: [...document.querySelectorAll('span.checkbox_formule')]
      .filter(span => span.querySelector('input[type=checkbox]'))
      .map(span => {
        const input = span.querySelector('input[type=checkbox]');
        const link = span.querySelector('a[data-src]');
        const detail = link && document.querySelector(link.dataset.src);
        const ingredients = detail ? [...detail.querySelectorAll('.h5')]
          .filter(h => h.textContent.includes('Ingrédients'))
          .map(h => text(h.nextElementSibling))
          .join(' ; ') : '';
        return {
          inputId: input.id,
          name: text(link),
          label: text(document.querySelector('label[for="' + input.id + '"]')),
          option: span.dataset.ref === 'Option',
          selected: input.checked,
          ingredients,
        };
      }),
  };
}`

type rawItem struct {
	InputID     string `json:"inputId"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Option      bool   `json:"option"`
	Selected    bool   `json:"selected"`
	Ingredients string `json:"ingredients"`
}

type rawPage struct {
	Week     string    `json:"week"`
	Deadline string    `json:"deadline"`
	People   string    `json:"people"`
	Price    string    `json:"price"`
	Items    []rawItem `json:"items"`
}

// Extract reads the preferences page currently loaded in page.
func Extract(page playwright.Page) (Page, error) {
	res, err := page.Evaluate(extractJS)
	if err != nil {
		return Page{}, fmt.Errorf("evaluating extraction script: %w", err)
	}

	data, err := json.Marshal(res)
	if err != nil {
		return Page{}, fmt.Errorf("marshaling extraction result: %w", err)
	}

	var raw rawPage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Page{}, fmt.Errorf("unmarshaling extraction result: %w", err)
	}

	return parse(raw)
}

func parse(raw rawPage) (Page, error) {
	week := weekRe.FindString(raw.Week)
	if week == "" {
		return Page{}, fmt.Errorf("week not found in %q", raw.Week)
	}

	deadline, _, _ := strings.Cut(raw.Deadline, " pour ")
	p := Page{Week: week, Deadline: deadline, People: raw.People, Price: raw.Price}

	optionN := firstOptionN
	for _, r := range raw.Items {
		it := Item{
			InputID: r.InputID, Name: r.Name, Label: r.Label,
			Option: r.Option, Selected: r.Selected, Ingredients: r.Ingredients,
		}

		if r.Option {
			it.N = optionN
			optionN++
		} else {
			m := recipeRe.FindStringSubmatch(r.Label)
			if m == nil {
				return Page{}, fmt.Errorf("unexpected recipe label %q", r.Label)
			}
			it.N, _ = strconv.Atoi(m[1]) // \d+ always parses
			it.Label = m[2]
		}

		p.Items = append(p.Items, it)
	}

	if len(p.Items) == 0 {
		return Page{}, fmt.Errorf("no recipe or option found on the page")
	}

	slices.SortStableFunc(p.Items, func(a, b Item) int { return a.N - b.N })

	return p, nil
}
```

- [ ] **Step 6: Run tests and lint** (inside `nix develop`, so Playwright is available)

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS. If `TestExtract` reports a wrong value, fix `extractJS`/`parse` against the fixture, never the expected values (they come from the live page screenshot).

- [ ] **Step 7: Check the sandbox skip**

Run: `nix build`
Expected: builds successfully (`TestExtract` is skipped because `PLAYWRIGHT_DRIVER_PATH` is unset in the sandbox). `vendorHash` must not need changing.

- [ ] **Step 8: Commit**

```bash
git add internal/recettescabas
git commit -m "recettescabas: extract recipes and options from the preferences page

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Annotation and recap text

**Files:**
- Create: `internal/recettescabas/recap.go`
- Test: `internal/recettescabas/recap_test.go`

**Interfaces:**
- Consumes: `Config`, `match` (Task 1); `Page`, `Item`, `Hit`, `withFixture`, `item` (Task 2).
- Produces: `func Annotate(p Page, cfg Config) Page`, `func RecapText(p Page) string`, `func DetailText(it Item) string`.

- [ ] **Step 1: Write the failing tests** `internal/recettescabas/recap_test.go`

```go
package recettescabas

import (
	"slices"
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
)

func testConfig() Config {
	return Config{
		ExcludeOptions: []string{"château", "domaine", "rosé"},
		Flags: []Flag{
			{Emoji: "🍄", Keywords: []string{"champignon"}},
			{Emoji: "🥗", Keywords: []string{"salade", "mesclun"}},
		},
	}
}

func TestAnnotate(t *testing.T) {
	p := Page{Items: []Item{
		{N: 1, Name: "Gratin champignons", Ingredients: "boulgour ; mascarpone"},           // title only: no flag
		{N: 2, Name: "Filet mignon rosé", Ingredients: "porc ; champignons de Paris"},     // recipe with an exclusion word: kept
		{N: 6, Name: "Quiche", Ingredients: "poireaux ; salade verte ; champignon"},       // two flags
		{N: 10, Name: "Tarte", Label: "Kit dessert (12.95€)", Option: true},
		{N: 18, Name: "Château Puech-Haut", Label: "IGP Pays d'Oc", Option: true},        // excluded by name
		{N: 19, Name: "Saint-Joseph Rouge", Label: "DOMAINE GILLES (16.90€)", Option: true}, // excluded by label
	}}

	got := Annotate(p, testConfig())

	var ns []int
	for _, it := range got.Items {
		ns = append(ns, it.N)
	}
	if want := []int{1, 2, 6, 10}; !slices.Equal(ns, want) {
		t.Fatalf("items = %v, want %v", ns, want)
	}
	if len(got.Items[0].Flags) != 0 {
		t.Errorf("title must not be matched: %+v", got.Items[0].Flags)
	}
	if want := []Hit{{"🍄", "champignons"}}; !slices.Equal(got.Items[1].Flags, want) {
		t.Errorf("item 2 flags = %+v", got.Items[1].Flags)
	}
	if want := []Hit{{"🍄", "champignon"}, {"🥗", "salade"}}; !slices.Equal(got.Items[2].Flags, want) {
		t.Errorf("item 6 flags = %+v", got.Items[2].Flags)
	}
}

func TestAnnotateWithoutRulesFlagsNothing(t *testing.T) {
	p := Page{Items: []Item{{N: 1, Ingredients: "champignons de Paris"}}}
	if got := Annotate(p, Config{}); len(got.Items[0].Flags) != 0 {
		t.Errorf("flags = %+v, want none: keywords must come from the config only", got.Items[0].Flags)
	}
}

func TestRecapText(t *testing.T) {
	p := Page{
		Week: "19/10/2026", Deadline: "Jeudi 15 Octobre 2026 à 23h00",
		People: "Panier à cuisiner 4 pers.", Price: "102,25",
		Items: []Item{
			{N: 1, Name: "Gratin", Label: "top chrono", Flags: []Hit{{"🍄", "champignons"}}},
			{N: 2, Name: "Filet mignon", Label: "viande 1", Selected: true},
			{N: 9, Name: "Poulet", Label: "viande 2", Selected: true, Flags: []Hit{{"🍄", "champignons"}}},
			{N: 10, Name: "Tarte", Label: "Kit dessert (12.95€)", Option: true},
		},
	}

	want := `🧺 Semaine du 19/10/2026 · modifiable jusqu'au Jeudi 15 Octobre 2026 à 23h00
Panier à cuisiner 4 pers. · 102,25 €
⚠️ Sélection : 🍄 9

✅ Sélection
2. Filet mignon (viande 1)
9. 🍄 Poulet (viande 2) — champignons

➕ Disponibles
1. 🍄 Gratin (top chrono) — champignons
10. Tarte (Kit dessert (12.95€))`

	if got := RecapText(p); got != want {
		t.Errorf("RecapText =\n%s\n\nwant\n%s", got, want)
	}
}

func TestRecapTextWithoutPriceNorAlert(t *testing.T) {
	p := Page{Week: "19/10/2026", Deadline: "Jeudi", People: "4 pers.", Items: []Item{{N: 2, Name: "Filet", Label: "viande 1", Selected: true}}}
	got := RecapText(p)
	if strings.Contains(got, "€") || strings.Contains(got, "⚠️") || strings.Contains(got, "Disponibles") {
		t.Errorf("unexpected content:\n%s", got)
	}
}

func TestDetailText(t *testing.T) {
	it := Item{N: 6, Name: "Quiche", Label: "protéinée", Ingredients: "poireaux ; salade verte", Flags: []Hit{{"🥗", "salade"}}}
	want := "6. 🥗 Quiche (protéinée) — salade\n\npoireaux ; salade verte"
	if got := DetailText(it); got != want {
		t.Errorf("DetailText = %q, want %q", got, want)
	}
}

func TestAnnotateFixture(t *testing.T) {
	cfg, err := LoadConfig("testdata/config.json")
	if err != nil {
		t.Fatal(err)
	}
	withFixture(t, func(page playwright.Page) {
		p, err := Extract(page)
		if err != nil {
			t.Fatal(err)
		}
		p = Annotate(p, cfg)

		if got := item(t, p, 1).Flags; !slices.Equal(got, []Hit{{"🍄", "champignons"}}) {
			t.Errorf("recipe 1 flags = %+v", got)
		}
		if got := item(t, p, 6).Flags; !slices.Equal(got, []Hit{{"🥗", "salade"}}) {
			t.Errorf("recipe 6 flags = %+v", got)
		}
		if got := item(t, p, 7).Flags; !slices.Equal(got, []Hit{{"🥗", "mesclun"}}) {
			t.Errorf("recipe 7 flags = %+v", got)
		}
		if got := item(t, p, 9).Flags; !slices.Equal(got, []Hit{{"🍄", "champignons"}}) {
			t.Errorf("recipe 9 flags = %+v (mushrooms hidden behind a title without them)", got)
		}
		// Recipe 5's pantry has "vin blanc": exclusions never apply to recipes.
		item(t, p, 5)

		var options []int
		for _, it := range p.Items {
			if it.Option {
				options = append(options, it.N)
			}
		}
		// 18, 19 and 20 are the wines.
		if want := []int{10, 11, 12, 13, 14, 15, 16, 17}; !slices.Equal(options, want) {
			t.Errorf("options = %v, want %v", options, want)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/recettescabas/ -run 'Annotate|Recap|Detail'`
Expected: FAIL to compile (`undefined: Annotate`).

- [ ] **Step 3: Implement** `internal/recettescabas/recap.go`

```go
package recettescabas

import (
	"fmt"
	"strconv"
	"strings"
)

// Annotate drops the options matching cfg.ExcludeOptions (name or label) and sets
// the Flags of every remaining item from its ingredients.
func Annotate(p Page, cfg Config) Page {
	items := make([]Item, 0, len(p.Items))

	for _, it := range p.Items {
		if it.Option && match(it.Name+" "+it.Label, cfg.ExcludeOptions) != "" {
			continue
		}

		it.Flags = nil
		for _, f := range cfg.Flags {
			if word := match(it.Ingredients, f.Keywords); word != "" {
				it.Flags = append(it.Flags, Hit{Emoji: f.Emoji, Word: word})
			}
		}

		items = append(items, it)
	}

	p.Items = items

	return p
}

func RecapText(p Page) string {
	var b strings.Builder

	fmt.Fprintf(&b, "🧺 Semaine du %s · modifiable jusqu'au %s\n", p.Week, p.Deadline)
	b.WriteString(p.People)
	if p.Price != "" {
		fmt.Fprintf(&b, " · %s €", p.Price)
	}

	var alerts []string
	for _, it := range p.Items {
		if it.Selected && len(it.Flags) > 0 {
			alerts = append(alerts, emojis(it)+" "+strconv.Itoa(it.N))
		}
	}
	if len(alerts) > 0 {
		b.WriteString("\n⚠️ Sélection : " + strings.Join(alerts, ", "))
	}

	section := func(title string, selected bool) {
		var lines []string
		for _, it := range p.Items {
			if it.Selected == selected {
				lines = append(lines, line(it))
			}
		}
		if len(lines) > 0 {
			b.WriteString("\n\n" + title + "\n" + strings.Join(lines, "\n"))
		}
	}
	section("✅ Sélection", true)
	section("➕ Disponibles", false)

	return b.String()
}

func DetailText(it Item) string {
	ingredients := it.Ingredients
	if ingredients == "" {
		ingredients = "(pas d'ingrédients indiqués)"
	}

	return line(it) + "\n\n" + ingredients
}

// line renders "9. 🍄 Poulet (viande 2) — champignons".
func line(it Item) string {
	s := strconv.Itoa(it.N) + ". "
	if len(it.Flags) > 0 {
		s += emojis(it) + " "
	}
	s += fmt.Sprintf("%s (%s)", it.Name, it.Label)

	if len(it.Flags) > 0 {
		words := make([]string, 0, len(it.Flags))
		for _, h := range it.Flags {
			words = append(words, h.Word)
		}
		s += " — " + strings.Join(words, ", ")
	}

	return s
}

func emojis(it Item) string {
	var s string
	for _, h := range it.Flags {
		s += h.Emoji
	}

	return s
}
```

- [ ] **Step 4: Run tests and lint**

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/recettescabas
git commit -m "recettescabas: flag ingredients, exclude options, render recap

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Inline keyboard (staged changes)

**Files:**
- Create: `internal/recettescabas/keyboard.go`
- Test: `internal/recettescabas/keyboard_test.go`

**Interfaces:**
- Consumes: `Item` (Task 2).
- Produces:

```go
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}
const cbToggle = "t:"; const cbValidate = "v"; const cbCancel = "x"
func NewKeyboard(items []Item) [][]Button
func Toggle(kb [][]Button, n int) [][]Button
func Reset(kb [][]Button) [][]Button
func Pending(kb [][]Button) map[int]bool // item N → selection state to apply
```

Button labels: `N` (not selected), `➕N` (add pending), `✅N` (selected), `➖N` (removal pending). Five per row. The action row `[✔️ Valider (k)] [✖️ Annuler]` exists only when k > 0.

- [ ] **Step 1: Write the failing tests** `internal/recettescabas/keyboard_test.go`

```go
package recettescabas

import (
	"maps"
	"testing"
)

func texts(kb [][]Button) [][]string {
	var out [][]string
	for _, row := range kb {
		var r []string
		for _, b := range row {
			r = append(r, b.Text)
		}
		out = append(out, r)
	}
	return out
}

func equalRows(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func sixItems() []Item {
	return []Item{{N: 1}, {N: 2, Selected: true}, {N: 3}, {N: 4, Selected: true}, {N: 5}, {N: 10}}
}

func TestNewKeyboard(t *testing.T) {
	kb := NewKeyboard(sixItems())
	want := [][]string{{"1", "✅2", "3", "✅4", "5"}, {"10"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
	if kb[1][0].CallbackData != "t:10" {
		t.Errorf("callback = %q", kb[1][0].CallbackData)
	}
	if len(Pending(kb)) != 0 {
		t.Error("fresh keyboard has no pending change")
	}
}

func TestToggleStagesAndUnstages(t *testing.T) {
	kb := Toggle(Toggle(NewKeyboard(sixItems()), 3), 2)

	want := [][]string{{"1", "➖2", "➕3", "✅4", "5"}, {"10"}, {"✔️ Valider (2)", "✖️ Annuler"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Fatalf("keyboard = %v, want %v", got, want)
	}
	if got, want := Pending(kb), map[int]bool{2: false, 3: true}; !maps.Equal(got, want) {
		t.Errorf("pending = %v, want %v", got, want)
	}
	if kb[2][0].CallbackData != cbValidate || kb[2][1].CallbackData != cbCancel {
		t.Errorf("action callbacks = %q, %q", kb[2][0].CallbackData, kb[2][1].CallbackData)
	}

	// Pressing again cancels the staged removal of 2: back to selected.
	kb = Toggle(kb, 2)
	want = [][]string{{"1", "✅2", "➕3", "✅4", "5"}, {"10"}, {"✔️ Valider (1)", "✖️ Annuler"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
}

func TestReset(t *testing.T) {
	kb := Reset(Toggle(Toggle(NewKeyboard(sixItems()), 3), 2))
	want := [][]string{{"1", "✅2", "3", "✅4", "5"}, {"10"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
}

func TestToggleUnknownNumberIsNoop(t *testing.T) {
	kb := Toggle(NewKeyboard(sixItems()), 42)
	if len(Pending(kb)) != 0 {
		t.Error("toggling an unknown number must not stage anything")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/recettescabas/ -run 'Keyboard|Toggle|Reset'`
Expected: FAIL to compile (`undefined: NewKeyboard`).

- [ ] **Step 3: Implement** `internal/recettescabas/keyboard.go`

```go
package recettescabas

import (
	"fmt"
	"strconv"
	"strings"
)

// Button is a Telegram inline keyboard button.
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

const (
	cbToggle   = "t:"
	cbValidate = "v"
	cbCancel   = "x"
	perRow     = 5
)

// cell is the state of one item button. The keyboard is the only place pending
// changes are stored: they're read back from button labels on each callback.
type cell struct {
	n        int
	selected bool
	pending  bool
}

func NewKeyboard(items []Item) [][]Button {
	cells := make([]cell, 0, len(items))
	for _, it := range items {
		cells = append(cells, cell{n: it.N, selected: it.Selected})
	}

	return build(cells)
}

func Toggle(kb [][]Button, n int) [][]Button {
	cells := cellsOf(kb)
	for i := range cells {
		if cells[i].n == n {
			cells[i].pending = !cells[i].pending
		}
	}

	return build(cells)
}

func Reset(kb [][]Button) [][]Button {
	cells := cellsOf(kb)
	for i := range cells {
		cells[i].pending = false
	}

	return build(cells)
}

// Pending returns, for each item with a staged change, the selection state to apply.
func Pending(kb [][]Button) map[int]bool {
	desired := map[int]bool{}
	for _, c := range cellsOf(kb) {
		if c.pending {
			desired[c.n] = !c.selected
		}
	}

	return desired
}

func build(cells []cell) [][]Button {
	var rows [][]Button

	pending := 0
	for i, c := range cells {
		if i%perRow == 0 {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], Button{Text: label(c), CallbackData: cbToggle + strconv.Itoa(c.n)})

		if c.pending {
			pending++
		}
	}

	if pending > 0 {
		rows = append(rows, []Button{
			{Text: fmt.Sprintf("✔️ Valider (%d)", pending), CallbackData: cbValidate},
			{Text: "✖️ Annuler", CallbackData: cbCancel},
		})
	}

	return rows
}

func label(c cell) string {
	n := strconv.Itoa(c.n)

	switch {
	case c.selected && c.pending:
		return "➖" + n
	case c.selected:
		return "✅" + n
	case c.pending:
		return "➕" + n
	default:
		return n
	}
}

func cellsOf(kb [][]Button) []cell {
	var cells []cell

	for _, row := range kb {
		for _, b := range row {
			nStr, ok := strings.CutPrefix(b.CallbackData, cbToggle)
			if !ok {
				continue
			}

			n, err := strconv.Atoi(nStr)
			if err != nil {
				continue
			}

			c := cell{n: n}
			switch {
			case strings.HasPrefix(b.Text, "➖"):
				c.selected, c.pending = true, true
			case strings.HasPrefix(b.Text, "✅"):
				c.selected = true
			case strings.HasPrefix(b.Text, "➕"):
				c.pending = true
			}

			cells = append(cells, c)
		}
	}

	return cells
}
```

- [ ] **Step 4: Run tests and lint**

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/recettescabas
git commit -m "recettescabas: inline keyboard with staged changes

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Telegram client

**Files:**
- Create: `internal/recettescabas/telegram.go`
- Test: `internal/recettescabas/telegram_test.go`

**Interfaces:**
- Consumes: `Button` (Task 4).
- Produces:

```go
type Telegram struct {
	Token   string
	BaseURL string // "https://api.telegram.org", overridden in tests
	HTTP    *http.Client
}
func NewTelegram(token string) Telegram
type Update struct{ UpdateID int; Message *Message; CallbackQuery *CallbackQuery }
type Message struct{ MessageID int; Chat Chat; Text string; ReplyMarkup *Markup }
type Chat struct{ ID int64 }
type CallbackQuery struct{ ID string; Message *Message; Data string }
type Markup struct{ InlineKeyboard [][]Button }
func (t Telegram) GetUpdates(ctx context.Context, offset int) ([]Update, error)
func (t Telegram) Send(ctx context.Context, chatID int64, text string, kb [][]Button) error
func (t Telegram) Edit(ctx context.Context, chatID int64, messageID int, text string, kb [][]Button) error // nil kb removes the keyboard
func (t Telegram) Answer(ctx context.Context, callbackID string) error
```

- [ ] **Step 1: Write the failing tests** `internal/recettescabas/telegram_test.go`

```go
package recettescabas

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tgCall struct {
	Method string
	Body   string
}

// fakeTelegram records every call and answers each with the given result JSON.
func fakeTelegram(t *testing.T, result string) (Telegram, *[]tgCall) {
	t.Helper()
	var calls []tgCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		if !strings.HasPrefix(r.URL.Path, "/botSECRET/") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		calls = append(calls, tgCall{Method: method, Body: string(body)})
		_, _ = io.WriteString(w, `{"ok":true,"result":`+result+`}`)
	}))
	t.Cleanup(srv.Close)
	tg := NewTelegram("SECRET")
	tg.BaseURL = srv.URL
	return tg, &calls
}

func TestSendWithKeyboard(t *testing.T) {
	tg, calls := fakeTelegram(t, `{}`)
	kb := [][]Button{{{Text: "✅2", CallbackData: "t:2"}}}
	if err := tg.Send(context.Background(), 42, "salut", kb); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	for _, want := range []string{`"chat_id":42`, `"text":"salut"`, `"inline_keyboard":[[{"text":"✅2","callback_data":"t:2"}]]`} {
		if c.Method != "sendMessage" || !strings.Contains(c.Body, want) {
			t.Errorf("call = %+v, missing %s", c, want)
		}
	}
}

func TestEditWithoutKeyboardOmitsMarkup(t *testing.T) {
	tg, calls := fakeTelegram(t, `{}`)
	if err := tg.Edit(context.Background(), 42, 7, "⏳", nil); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.Method != "editMessageText" || !strings.Contains(c.Body, `"message_id":7`) || strings.Contains(c.Body, "reply_markup") {
		t.Errorf("call = %+v", c)
	}
}

func TestGetUpdates(t *testing.T) {
	tg, calls := fakeTelegram(t, `[{"update_id":5,"callback_query":{"id":"cb","data":"t:3","message":{"message_id":7,"chat":{"id":42},"text":"x","reply_markup":{"inline_keyboard":[[{"text":"3","callback_data":"t:3"}]]}}}}]`)
	updates, err := tg.GetUpdates(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains((*calls)[0].Body, `"offset":5`) {
		t.Errorf("body = %s", (*calls)[0].Body)
	}
	q := updates[0].CallbackQuery
	if updates[0].UpdateID != 5 || q.Data != "t:3" || q.Message.Chat.ID != 42 || q.Message.ReplyMarkup.InlineKeyboard[0][0].Text != "3" {
		t.Errorf("updates = %+v", updates)
	}
}

func TestAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/editMessageText") {
			_, _ = io.WriteString(w, `{"ok":false,"description":"Bad Request: message is not modified"}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`)
	}))
	defer srv.Close()
	tg := NewTelegram("SECRET")
	tg.BaseURL = srv.URL

	if err := tg.Edit(context.Background(), 1, 1, "x", nil); err != nil {
		t.Errorf("'message is not modified' must not be an error, got %v", err)
	}
	if err := tg.Send(context.Background(), 1, "x", nil); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Errorf("err = %v", err)
	}
}

func TestTransportErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // connection refused
	tg := NewTelegram("SECRET")
	tg.BaseURL = srv.URL

	err := tg.Send(context.Background(), 1, "x", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error leaks the bot token: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/recettescabas/ -run 'Send|Edit|GetUpdates|APIErrors|Transport'`
Expected: FAIL to compile (`undefined: NewTelegram`).

- [ ] **Step 3: Implement** `internal/recettescabas/telegram.go`

```go
package recettescabas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const pollTimeout = 50 // seconds, Telegram long polling

// Telegram is a minimal Bot API client.
type Telegram struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
}

func NewTelegram(token string) Telegram {
	return Telegram{
		Token:   token,
		BaseURL: "https://api.telegram.org",
		HTTP:    &http.Client{Timeout: (pollTimeout + 20) * time.Second},
	}
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID   int     `json:"message_id"`
	Chat        Chat    `json:"chat"`
	Text        string  `json:"text"`
	ReplyMarkup *Markup `json:"reply_markup"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Markup struct {
	InlineKeyboard [][]Button `json:"inline_keyboard"`
}

func markup(kb [][]Button) *Markup {
	if len(kb) == 0 {
		return nil
	}

	return &Markup{InlineKeyboard: kb}
}

func (t Telegram) GetUpdates(ctx context.Context, offset int) ([]Update, error) {
	var updates []Update
	err := t.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         pollTimeout,
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)

	return updates, err
}

func (t Telegram) Send(ctx context.Context, chatID int64, text string, kb [][]Button) error {
	return t.call(ctx, "sendMessage", struct {
		ChatID      int64   `json:"chat_id"`
		Text        string  `json:"text"`
		ReplyMarkup *Markup `json:"reply_markup,omitempty"`
	}{chatID, text, markup(kb)}, nil)
}

// Edit replaces a message's text and keyboard; a nil kb removes the keyboard.
func (t Telegram) Edit(ctx context.Context, chatID int64, messageID int, text string, kb [][]Button) error {
	return t.call(ctx, "editMessageText", struct {
		ChatID      int64   `json:"chat_id"`
		MessageID   int     `json:"message_id"`
		Text        string  `json:"text"`
		ReplyMarkup *Markup `json:"reply_markup,omitempty"`
	}{chatID, messageID, text, markup(kb)}, nil)
}

func (t Telegram) Answer(ctx context.Context, callbackID string) error {
	return t.call(ctx, "answerCallbackQuery", map[string]string{"callback_query_id": callbackID}, nil)
}

func (t Telegram) call(ctx context.Context, method string, params, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("%s: marshaling params: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.BaseURL+"/bot"+t.Token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: creating request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.HTTP.Do(req)
	if err != nil {
		// *url.Error's message contains the URL, which contains the token.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}

		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("%s: decoding response: %w", method, err)
	}

	if !env.OK {
		if strings.Contains(env.Description, "message is not modified") {
			return nil
		}

		return fmt.Errorf("%s: %s", method, env.Description)
	}

	if result != nil {
		if err := json.Unmarshal(env.Result, result); err != nil {
			return fmt.Errorf("%s: decoding result: %w", method, err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Run tests and lint**

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/recettescabas
git commit -m "recettescabas: minimal Telegram Bot API client

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Bot service (recap, commands, callbacks)

**Files:**
- Create: `internal/recettescabas/bot.go`
- Test: `internal/recettescabas/bot_test.go`

**Interfaces:**
- Consumes: `Annotate`, `RecapText`, `DetailText` (Task 3); `NewKeyboard`, `Toggle`, `Reset`, `Pending`, `cbToggle`, `cbValidate`, `cbCancel` (Task 4); `Telegram`, `Update`, `Message`, `CallbackQuery` (Task 5); `weekRe` (Task 2); `fakeTelegram`, `tgCall` (Task 5 tests).
- Produces:

```go
var ErrWeekChanged = errors.New("week changed")
type Service struct {
	TG     Telegram
	ChatID int64
	Config Config
	Fetch  func(ctx context.Context) (Page, error)                                  // logs in, returns the current page
	Apply  func(ctx context.Context, week string, desired map[int]bool) (Page, error) // returns the page after applying; ErrWeekChanged if the site's week != week
	Log    io.Writer
}
func (s Service) SendRecap(ctx context.Context) error
func (s Service) Run(ctx context.Context) error // long polling until ctx is done
```

- [ ] **Step 1: Write the failing tests** `internal/recettescabas/bot_test.go`

```go
package recettescabas

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"strings"
	"testing"
)

const testChat = 42

func botPage() Page {
	return Page{
		Week: "19/10/2026", Deadline: "Jeudi 15 Octobre 2026 à 23h00", People: "4 pers.",
		Items: []Item{
			{N: 1, Name: "Gratin", Label: "top chrono", Ingredients: "champignons de Paris"},
			{N: 2, Name: "Filet", Label: "viande 1", Selected: true, Ingredients: "porc"},
			{N: 3, Name: "Poisson", Label: "poisson", Ingredients: "cabillaud"},
		},
	}
}

type fakes struct {
	fetches     int
	applyWeek   string
	applyWanted map[int]bool
	applyErr    error
}

func newService(t *testing.T) (Service, *fakes, *[]tgCall, *bytes.Buffer) {
	tg, calls := fakeTelegram(t, `{}`)
	f := &fakes{}
	var log bytes.Buffer
	s := Service{
		TG: tg, ChatID: testChat, Config: testConfig(), Log: &log,
		Fetch: func(context.Context) (Page, error) { f.fetches++; return botPage(), nil },
		Apply: func(_ context.Context, week string, desired map[int]bool) (Page, error) {
			f.applyWeek, f.applyWanted = week, desired
			if f.applyErr != nil {
				return Page{}, f.applyErr
			}
			p := botPage()
			p.Items[2].Selected = true
			return p, nil
		},
	}
	return s, f, calls, &log
}

func recapMessage(kb [][]Button) *Message {
	p := Annotate(botPage(), testConfig())
	return &Message{MessageID: 7, Chat: Chat{ID: testChat}, Text: RecapText(p), ReplyMarkup: &Markup{InlineKeyboard: kb}}
}

func callback(data string, msg *Message) Update {
	return Update{CallbackQuery: &CallbackQuery{ID: "cb", Data: data, Message: msg}}
}

func last(calls *[]tgCall) tgCall { return (*calls)[len(*calls)-1] }

func TestSendRecap(t *testing.T) {
	s, _, calls, _ := newService(t)
	if err := s.SendRecap(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := last(calls)
	for _, want := range []string{`"chat_id":42`, "Semaine du 19/10/2026", "🍄 Gratin", `"callback_data":"t:3"`} {
		if c.Method != "sendMessage" || !strings.Contains(c.Body, want) {
			t.Errorf("call = %+v, missing %s", c, want)
		}
	}
}

func TestIgnoresOtherChats(t *testing.T) {
	s, f, calls, log := newService(t)
	s.handle(context.Background(), Update{Message: &Message{Chat: Chat{ID: 999}, Text: "/recap"}})
	other := recapMessage(NewKeyboard(botPage().Items))
	other.Chat.ID = 999
	s.handle(context.Background(), callback(cbValidate, other))

	if len(*calls) != 0 || f.fetches != 0 || f.applyWanted != nil {
		t.Errorf("calls = %+v, fetches = %d, apply = %v", *calls, f.fetches, f.applyWanted)
	}
	if !strings.Contains(log.String(), "999") {
		t.Errorf("log = %q, want the ignored chat id", log.String())
	}
}

func TestToggleCallbackEditsKeyboardOnly(t *testing.T) {
	s, f, calls, _ := newService(t)
	s.handle(context.Background(), callback("t:3", recapMessage(NewKeyboard(botPage().Items))))

	if (*calls)[0].Method != "answerCallbackQuery" {
		t.Errorf("first call = %s, want answerCallbackQuery", (*calls)[0].Method)
	}
	c := last(calls)
	if c.Method != "editMessageText" || !strings.Contains(c.Body, "➕3") || !strings.Contains(c.Body, "Valider (1)") {
		t.Errorf("call = %+v", c)
	}
	if f.fetches != 0 || f.applyWanted != nil {
		t.Error("toggling must not touch the site")
	}
}

func TestGarbageCallbackIsNoop(t *testing.T) {
	s, _, calls, _ := newService(t)
	s.handle(context.Background(), callback("t:x", recapMessage(NewKeyboard(botPage().Items))))
	s.handle(context.Background(), callback("zzz", recapMessage(nil)))
	for _, c := range *calls {
		if c.Method != "answerCallbackQuery" {
			t.Errorf("unexpected call %+v", c)
		}
	}
}

func TestValidateAppliesAndShowsNewRecap(t *testing.T) {
	s, f, calls, _ := newService(t)
	kb := Toggle(NewKeyboard(botPage().Items), 3)
	s.handle(context.Background(), callback(cbValidate, recapMessage(kb)))

	if f.applyWeek != "19/10/2026" || !maps.Equal(f.applyWanted, map[int]bool{3: true}) {
		t.Errorf("apply(%q, %v)", f.applyWeek, f.applyWanted)
	}
	c := last(calls)
	if c.Method != "editMessageText" || !strings.Contains(c.Body, "✅3") || strings.Contains(c.Body, "Valider") {
		t.Errorf("final edit = %+v", c)
	}
}

func TestValidateWeekChanged(t *testing.T) {
	s, f, calls, _ := newService(t)
	f.applyErr = ErrWeekChanged
	s.handle(context.Background(), callback(cbValidate, recapMessage(Toggle(NewKeyboard(botPage().Items), 3))))

	c := last(calls)
	if c.Method != "editMessageText" || !strings.Contains(c.Body, "La semaine a changé") || strings.Contains(c.Body, "inline_keyboard") {
		t.Errorf("final edit = %+v", c)
	}
}

func TestValidateFailureRestoresKeyboard(t *testing.T) {
	s, f, calls, _ := newService(t)
	f.applyErr = errors.New("boom")
	s.handle(context.Background(), callback(cbValidate, recapMessage(Toggle(NewKeyboard(botPage().Items), 3))))

	var restored, reported bool
	for _, c := range *calls {
		restored = restored || (c.Method == "editMessageText" && strings.Contains(c.Body, "➕3"))
		reported = reported || (c.Method == "sendMessage" && strings.Contains(c.Body, "❌ échec : boom"))
	}
	if !restored || !reported {
		t.Errorf("restored = %v, reported = %v, calls = %+v", restored, reported, *calls)
	}
}

func TestDetail(t *testing.T) {
	tests := []struct {
		text    string
		want    string
		fetches int
	}{
		{"/detail 1", "champignons de Paris", 1},
		{"/detail 42", "Élément 42 inconnu", 1},
		{"/detail abc", "Usage : /detail N", 0},
		{"/detail", "Usage : /detail N", 0},
		{"bonjour", "/recap", 0},
	}
	for _, tt := range tests {
		s, f, calls, _ := newService(t)
		s.handle(context.Background(), Update{Message: &Message{Chat: Chat{ID: testChat}, Text: tt.text}})
		if c := last(calls); c.Method != "sendMessage" || !strings.Contains(c.Body, tt.want) || f.fetches != tt.fetches {
			t.Errorf("%q: call = %+v, fetches = %d", tt.text, c, f.fetches)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/recettescabas/ -run 'SendRecap|Ignores|Callback|Validate|Detail'`
Expected: FAIL to compile (`undefined: Service`).

- [ ] **Step 3: Implement** `internal/recettescabas/bot.go`

```go
package recettescabas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ErrWeekChanged is returned by Apply when the site shows another week than the recap.
var ErrWeekChanged = errors.New("week changed")

const helpText = "/recap — renvoie le récap de la semaine\n/detail N — ingrédients de l'élément N"

type Service struct {
	TG     Telegram
	ChatID int64
	Config Config
	// Fetch logs in and returns the current preferences page.
	Fetch func(ctx context.Context) (Page, error)
	// Apply sets the selection state of each item in desired and returns the page
	// as the site saved it. It returns ErrWeekChanged if the site's week isn't week.
	Apply func(ctx context.Context, week string, desired map[int]bool) (Page, error)
	Log   io.Writer
}

func (s Service) SendRecap(ctx context.Context) error {
	p, err := s.Fetch(ctx)
	if err != nil {
		return err
	}

	p = Annotate(p, s.Config)

	return s.TG.Send(ctx, s.ChatID, RecapText(p), NewKeyboard(p.Items))
}

// Run handles updates until ctx is done.
func (s Service) Run(ctx context.Context) error {
	offset := 0

	for {
		updates, err := s.TG.GetUpdates(ctx, offset)
		if ctx.Err() != nil {
			return nil //nolint:nilerr // shutting down
		}

		if err != nil {
			s.report(err)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}

			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			s.handle(ctx, u)
		}
	}
}

func (s Service) handle(ctx context.Context, u Update) {
	switch {
	case u.CallbackQuery != nil && u.CallbackQuery.Message != nil:
		if id := u.CallbackQuery.Message.Chat.ID; id != s.ChatID {
			s.ignore(id)
			return
		}

		s.callback(ctx, u.CallbackQuery)
	case u.Message != nil:
		if id := u.Message.Chat.ID; id != s.ChatID {
			s.ignore(id)
			return
		}

		s.message(ctx, u.Message)
	}
}

func (s Service) message(ctx context.Context, m *Message) {
	cmd, arg, _ := strings.Cut(strings.TrimSpace(m.Text), " ")

	switch cmd {
	case "/recap":
		if err := s.SendRecap(ctx); err != nil {
			s.fail(ctx, err)
		}
	case "/detail":
		s.detail(ctx, arg)
	default:
		s.report(s.TG.Send(ctx, s.ChatID, helpText, nil))
	}
}

func (s Service) detail(ctx context.Context, arg string) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil {
		s.report(s.TG.Send(ctx, s.ChatID, "Usage : /detail N", nil))
		return
	}

	p, err := s.Fetch(ctx)
	if err != nil {
		s.fail(ctx, err)
		return
	}

	for _, it := range Annotate(p, s.Config).Items {
		if it.N == n {
			s.report(s.TG.Send(ctx, s.ChatID, DetailText(it), nil))
			return
		}
	}

	s.report(s.TG.Send(ctx, s.ChatID, fmt.Sprintf("Élément %d inconnu.", n), nil))
}

func (s Service) callback(ctx context.Context, q *CallbackQuery) {
	s.report(s.TG.Answer(ctx, q.ID))

	msg := q.Message

	var kb [][]Button
	if msg.ReplyMarkup != nil {
		kb = msg.ReplyMarkup.InlineKeyboard
	}

	switch {
	case strings.HasPrefix(q.Data, cbToggle):
		n, err := strconv.Atoi(strings.TrimPrefix(q.Data, cbToggle))
		if err != nil {
			return
		}

		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, msg.Text, Toggle(kb, n)))
	case q.Data == cbCancel:
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, msg.Text, Reset(kb)))
	case q.Data == cbValidate:
		s.validate(ctx, msg, kb)
	}
}

func (s Service) validate(ctx context.Context, msg *Message, kb [][]Button) {
	desired := Pending(kb)
	if len(desired) == 0 {
		return
	}

	s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, "⏳ application…", nil))

	// An empty week (unexpected message text) never matches the site's: nothing is applied.
	p, err := s.Apply(ctx, weekRe.FindString(msg.Text), desired)

	switch {
	case errors.Is(err, ErrWeekChanged):
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID,
			msg.Text+"\n\n⚠️ La semaine a changé : rien n'a été appliqué. Utilise /recap.", nil))
	case err != nil:
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, msg.Text, kb))
		s.fail(ctx, err)
	default:
		p = Annotate(p, s.Config)
		s.report(s.TG.Edit(ctx, s.ChatID, msg.MessageID, RecapText(p), NewKeyboard(p.Items)))
	}
}

func (s Service) fail(ctx context.Context, err error) {
	s.report(err)
	s.report(s.TG.Send(ctx, s.ChatID, "❌ échec : "+err.Error(), nil))
}

func (s Service) report(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(s.Log, "recettes-cabas: %v\n", err)
	}
}

func (s Service) ignore(chatID int64) {
	_, _ = fmt.Fprintf(s.Log, "recettes-cabas: ignoring update from chat %d\n", chatID)
}
```

- [ ] **Step 4: Run tests and lint**

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS. (If the linter flags the `nolint:nilerr` directive as unused, drop it.)

- [ ] **Step 5: Commit**

```bash
git add internal/recettescabas
git commit -m "recettescabas: Telegram bot service

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Live site (login, apply) and CLI — paired with Gabriel

**Not delegable to a subagent alone:** Steps 1 and 7 need Gabriel's credentials, a real Telegram bot and an interactive browser. Run this task in the main session.

**Files:**
- Create: `internal/recettescabas/site.go`
- Test: `internal/recettescabas/site_test.go`
- Modify: `main.go` (new command), `README.md`, `CLAUDE.md` (provider list)

**Interfaces:**
- Consumes: `pw.Run` ; `Extract`, `Page`, `Item`, `withFixture`, `item` (Task 2) ; `LoadConfig` (Task 1) ; `NewTelegram` (Task 5) ; `Service`, `ErrWeekChanged` (Task 6).
- Produces: `func NewService(stdout, stderr io.Writer, headless bool, username, password, configPath, token string, chatID int64) (Service, error)`; CLI `downloader-go recettes-cabas recap|bot`.

- [ ] **Step 1: Discover the login flow (with Gabriel).** In `nix develop`, Gabriel runs:

```bash
node "$PLAYWRIGHT_DRIVER_PATH/package/cli.js" codegen 'https://www.recettesetcabas.com/?Action=espaceClientAbonnement'
```

and logs in, then reaches "Modifier mes préférences". From the generated code, note: the preferences page URL (confirm `prefURL`), the login page URL if different, and the selectors of the username field, password field and submit button. Don't paste credentials anywhere.

- [ ] **Step 2: Write the failing test** `internal/recettescabas/site_test.go` (offline: only the checkbox writing, never the submit, the fixture's form posts to the real site)

```go
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
		r1 := item(t, before, 1)

		if err := setSelection(page, map[string]bool{"rtype_23": false, r1.InputID: true}); err != nil {
			t.Fatal(err)
		}

		after, err := Extract(page)
		if err != nil {
			t.Fatal(err)
		}
		if item(t, after, 2).Selected || !item(t, after, 1).Selected {
			t.Errorf("selection not applied: 1=%v 2=%v", item(t, after, 1).Selected, item(t, after, 2).Selected)
		}

		if err := setSelection(page, map[string]bool{"nope": true}); err == nil {
			t.Error("expected an error for a missing input")
		}
	})
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/recettescabas/ -run TestSetSelection`
Expected: FAIL to compile (`undefined: setSelection`).

- [ ] **Step 4: Implement** `internal/recettescabas/site.go`. Set `prefURL` and the three `login*Selector` constants to the values found in Step 1. Keep the "already logged in" check: `pw.Run` reuses `cookies.json`.

```go
package recettescabas

import (
	"context"
	"fmt"
	"io"

	"github.com/Crocmagnon/downloader-go/internal/pw"
	"github.com/mxschmitt/playwright-go"
)

// Values confirmed against the live site (Task 7 Step 1).
const (
	prefURL              = "https://www.recettesetcabas.com/?Action=espaceClientAbonnement"
	prefFormSelector     = `form[action*="validerPreference"]`
	loginUserSelector    = "…from Step 1…"
	loginPassSelector    = "…from Step 1…"
	loginSubmitSelector  = "…from Step 1…"
	waitTimeoutMs        = 15000
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

func NewService(stdout, stderr io.Writer, headless bool, username, password, configPath, token string, chatID int64) (Service, error) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return Service{}, err
	}

	browse := func(fn func(playwright.Page) error) error {
		return pw.Run(stdout, stderr, headless, pw.BrowserDefault, fn)
	}

	return Service{
		TG:     NewTelegram(token),
		ChatID: chatID,
		Config: cfg,
		Log:    stderr,
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

func login(page playwright.Page, username, password string) error {
	if _, err := page.Goto(prefURL); err != nil {
		return fmt.Errorf("going to preferences: %w", err)
	}

	if n, err := page.Locator(prefFormSelector).Count(); err == nil && n > 0 {
		return nil // already logged in thanks to cookies
	}

	if err := page.Locator(loginUserSelector).Fill(username); err != nil {
		return fmt.Errorf("typing username: %w", err)
	}
	if err := page.Locator(loginPassSelector).Fill(password); err != nil {
		return fmt.Errorf("typing password: %w", err)
	}
	if err := page.Locator(loginSubmitSelector).Click(); err != nil {
		return fmt.Errorf("clicking login button: %w", err)
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

	if err := page.Locator(prefFormSelector).WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(waitTimeoutMs)}); err != nil {
		return Page{}, fmt.Errorf("waiting for preferences form: %w", err)
	}

	return Extract(page)
}

func setSelection(page playwright.Page, changes map[string]bool) error {
	if _, err := page.Evaluate(setSelectionJS, changes); err != nil {
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
```

Note: `page.ExpectResponse`'s exact signature in playwright-go v0.6100 must be checked (`go doc github.com/mxschmitt/playwright-go Page.ExpectResponse`); adapt the call, not the behavior (wait for the POST to `validerPreference` to complete before reloading).

- [ ] **Step 5: Run tests and lint**

Run: `go test ./internal/recettescabas/ && golangci-lint run ./...`
Expected: PASS.

- [ ] **Step 6: Wire the CLI** in `main.go`. Add the import `"github.com/Crocmagnon/downloader-go/internal/recettescabas"`, plus `"context"`, `"os/signal"`, `"syscall"`, and next to the other commands:

```go
type RecettesCabasFlags struct {
	Username       string `required:"" short:"u" env:"RC_USERNAME" help:"Recettes & Cabas username"`
	Password       string `required:"" short:"p" env:"RC_PASSWORD" help:"Recettes & Cabas password"`
	Config         string `required:"" type:"existingfile" help:"JSON config file (flags and excluded options)."`
	TelegramToken  string `required:"" env:"TELEGRAM_TOKEN" help:"Telegram bot token"`
	TelegramChatID int64  `required:"" env:"TELEGRAM_CHAT_ID" help:"The only Telegram chat the bot talks to"`
}

func (f RecettesCabasFlags) service(ctx *Context) (recettescabas.Service, error) {
	return recettescabas.NewService(os.Stdout, os.Stderr, ctx.Headless, f.Username, f.Password, f.Config, f.TelegramToken, f.TelegramChatID)
}

type RecettesCabasRecapCmd struct {
	RecettesCabasFlags `embed:""`
}

func (r *RecettesCabasRecapCmd) Run(ctx *Context) error {
	svc, err := r.service(ctx)
	if err != nil {
		return err
	}
	return svc.SendRecap(context.Background())
}

type RecettesCabasBotCmd struct {
	RecettesCabasFlags `embed:""`
}

func (r *RecettesCabasBotCmd) Run(ctx *Context) error {
	svc, err := r.service(ctx)
	if err != nil {
		return err
	}
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return svc.Run(sigCtx)
}

type RecettesCabasCmd struct {
	Recap RecettesCabasRecapCmd `cmd:"" help:"Send the weekly recap to Telegram."`
	Bot   RecettesCabasBotCmd   `cmd:"" help:"Run the Telegram bot that applies selection changes."`
}
```

and in `Cli`, before `Photos`:

```go
	RecettesCabas        RecettesCabasCmd        `cmd:"" help:"Recettes & Cabas weekly recap and Telegram bot."`
```

Run: `go build -o /dev/null . && go run . recettes-cabas recap --help`
Expected: help lists `--config`, `--telegram-token ($TELEGRAM_TOKEN)`, etc.

- [ ] **Step 7: Live check (with Gabriel).** Gabriel creates a bot with @BotFather and exports `RC_USERNAME`, `RC_PASSWORD`, `TELEGRAM_TOKEN` in his shell (never in a file in the repo).
  1. Get the chat id: `TELEGRAM_CHAT_ID=0 go run . --headless recettes-cabas bot --config internal/recettescabas/testdata/config.json`, send any message to the bot, read `ignoring update from chat <id>` in the output, stop the bot, `export TELEGRAM_CHAT_ID=<id>`.
  2. `go run . recettes-cabas recap --config internal/recettescabas/testdata/config.json` (headful): the recap arrives, price is filled, wines are hidden, flags match the page.
  3. `go run . --headless recettes-cabas bot --config …`: toggle a cheap option on and off (pending state shows, Annuler resets), then stage one real change Gabriel wants, Valider → the new recap shows it, and the site's preferences page agrees. Revert if it was only a test.
  4. `/detail 6` and `/recap` answer.
  If the submit doesn't stick (step 3 reports "the site did not save the change"), switch `apply` to clicking the visible submit button (`page.Locator("button.js-btn-submit:not(.disabled)").Last().Click()`) after dispatching `change` events, and re-check.

- [ ] **Step 8: Docs.** In `README.md` and `CLAUDE.md`, add `recettescabas` to the provider list: "Recettes & Cabas: weekly Telegram recap with ingredient flags, plus a bot to change the selection (`recettes-cabas recap|bot`, see `internal/recettescabas/testdata/config.json` for the config format)".

- [ ] **Step 9: Run tests, lint, nix build, commit**

Run: `go test ./... && golangci-lint run ./... && nix build`
Expected: all PASS.

```bash
git add internal/recettescabas main.go README.md CLAUDE.md
git commit -m "recettescabas: live site login/apply and recettes-cabas command

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Deploy on srv-1 (nixos-config) — paired with Gabriel

**Prerequisite:** the `recettes-cabas` branch is merged and pushed to `main` on codeberg (the nixos-config flake input `downloader` points to `git+https://codeberg.org/Crocmagnon/downloader-go`).

**Files (in `../nixos-config`, on a new branch):**
- Create: `hosts/srv-1/services/recettes-cabas.nix`
- Modify: `hosts/srv-1/services/default.nix` (import), `hosts/srv-1/age/secrets.nix` and `hosts/srv-1/age/default.nix` (new secret, mirroring `downloader-creds-env`), `flake.lock`

- [ ] **Step 1: Update the input**

Run (in nixos-config): `nix flake update downloader`

- [ ] **Step 2: Create** `hosts/srv-1/services/recettes-cabas.nix`

```nix
{
  pkgs,
  lib,
  config,
  ...
}:
let
  configFile = pkgs.writeText "recettes-cabas.json" (
    builtins.toJSON {
      excludeOptions = [
        "château"
        "domaine"
        "vin"
        "rosé"
        "IGP"
        "AOC"
      ];
      flags = [
        {
          emoji = "🍄";
          keywords = [
            "champignon"
            "cèpe"
            "girolle"
            "shiitake"
            "pleurote"
            "morille"
            "trompette"
          ];
        }
        {
          emoji = "🥗";
          keywords = [
            "salade"
            "laitue"
            "mâche"
            "roquette"
            "mesclun"
            "batavia"
            "sucrine"
            "romaine"
            "jeunes pousses"
          ];
        }
      ];
    }
  );
  stateDir = "/var/lib/recettes-cabas";
  cmd = sub: "${pkgs.downloader-go}/bin/downloader-go --headless recettes-cabas ${sub} --config ${configFile}";
  # Both units share the dynamic user (same name) and its state directory (cookies, screenshots).
  common = {
    after = [ "network-online.target" ];
    environment.HOME = stateDir;
    serviceConfig = {
      User = "recettes-cabas";
      DynamicUser = true;
      StateDirectory = "recettes-cabas";
      WorkingDirectory = stateDir;
      EnvironmentFile = config.age.secrets."recettes-cabas-env".path;
    };
  };
in
{
  systemd.timers.recettes-cabas-recap = {
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "Wed 18:00";
      Persistent = true;
    };
  };

  systemd.services.recettes-cabas-recap = lib.recursiveUpdate common {
    restartIfChanged = false;
    wants = [
      "network-online.target"
      "healthchecks@recettes-cabas-recap:start.service"
    ];
    onSuccess = [ "healthchecks@recettes-cabas-recap:success.service" ];
    onFailure = [ "healthchecks@recettes-cabas-recap:failure.service" ];
    serviceConfig = {
      Type = "oneshot";
      ExecStart = cmd "recap";
    };
  };

  systemd.services.recettes-cabas-bot = lib.recursiveUpdate common {
    wantedBy = [ "multi-user.target" ];
    wants = [ "network-online.target" ];
    serviceConfig = {
      ExecStart = cmd "bot";
      Restart = "always";
      RestartSec = 30;
    };
  };
}
```

- [ ] **Step 3: Import it** in `hosts/srv-1/services/default.nix` (add `./recettes-cabas.nix` to `imports`).

- [ ] **Step 4: Secret (Gabriel).** Declare `recettes-cabas-env` in `hosts/srv-1/age/secrets.nix` and `hosts/srv-1/age/default.nix` exactly like `downloader-creds-env` (same recipients; default root owner is fine, systemd reads `EnvironmentFile` as root). Gabriel creates its content with agenix:

```
RC_USERNAME=…
RC_PASSWORD=…
TELEGRAM_TOKEN=…
TELEGRAM_CHAT_ID=…
```

- [ ] **Step 5: Build** following nixos-config's `CLAUDE.md` (build srv-1's configuration before deploying). Expected: builds.

- [ ] **Step 6: Commit, PR, deploy.** Commit on the branch, open a PR on nixos-config, Gabriel deploys. After deploy:

```bash
systemctl start recettes-cabas-recap.service   # a recap arrives on Telegram
systemctl status recettes-cabas-bot.service    # active (running)
journalctl -u recettes-cabas-bot -n 20
```

Expected: recap received, a button press is answered, healthchecks shows `recettes-cabas-recap` green.

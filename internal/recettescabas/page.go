package recettescabas

import (
	"encoding/json/v2"
	"errors"
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
	Selection   string // "this week's selection" of an option (fruits, cheeses…), if any
	Flags       []Hit  // set by Annotate
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
	recipeRe = regexp.MustCompile(`(?i)^recette\s+(\d+)\s*[-–—]\s*(.*)$`)
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
        const weekly = detail && [...detail.querySelectorAll('header p')]
          .find(p => p.textContent.toLowerCase().includes('lection de la semaine'));
        return {
          inputId: input.id,
          selection: text(weekly && weekly.querySelector('strong, b')),
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
	Selection   string `json:"selection"`
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
			Option: r.Option, Selected: r.Selected, Ingredients: r.Ingredients, Selection: r.Selection,
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
		return Page{}, errors.New("no recipe or option found on the page")
	}

	slices.SortStableFunc(p.Items, func(a, b Item) int { return a.N - b.N })

	return p, nil
}

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
		{N: 1, Name: "Gratin champignons", Ingredients: "boulgour ; mascarpone"},      // title only: no flag
		{N: 2, Name: "Filet mignon rosé", Ingredients: "porc ; champignons de Paris"}, // recipe with an exclusion word: kept
		{N: 6, Name: "Quiche", Ingredients: "poireaux ; salade verte ; champignon"},   // two flags
		{N: 10, Name: "Tarte", Label: "Kit dessert (12.95€)", Option: true},
		{N: 18, Name: "Château Puech-Haut", Label: "IGP Pays d'Oc", Option: true},           // excluded by name
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

		if got := item(t, p, 4).Flags; !slices.Equal(got, []Hit{{"🍄", "champignons"}}) {
			t.Errorf("recipe 4 flags = %+v", got)
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

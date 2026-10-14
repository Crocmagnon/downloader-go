package recettescabas

import (
	"slices"
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

func TestAnnotateMatchesPantryToo(t *testing.T) {
	p := Page{Items: []Item{{N: 1, Ingredients: "riz", Pantry: "salade verte"}}}
	if got := Annotate(p, testConfig()).Items[0].Flags; !slices.Equal(got, []Hit{{"🥗", "salade"}}) {
		t.Errorf("flags = %+v", got)
	}
}

func TestAnnotateWithoutRulesFlagsNothing(t *testing.T) {
	p := Page{Items: []Item{{N: 1, Ingredients: "champignons de Paris"}}}
	if got := Annotate(p, Config{}); len(got.Items[0].Flags) != 0 {
		t.Errorf("flags = %+v, want none: keywords must come from the config only", got.Items[0].Flags)
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

func TestLabelSuffixDropsRepeatedName(t *testing.T) {
	tests := []struct {
		it   Item
		want string
	}{
		{Item{Name: "Fruits Petit Format", Label: "Fruits Petit Format (9.10€)", Option: true}, "9.10€"},
		{Item{Name: "Assortiment de desserts lactés de la ferme", Label: "ASSORTIMENT DE DESSERTS LACTÉS DE LA FERME (10.70€)", Option: true}, "10.70€"},
		{Item{Name: "Tarte fondante aux noix", Label: "Kit dessert à cuisiner (12.95€)", Option: true}, "Kit dessert à cuisiner (12.95€)"},
		{Item{Name: "Filet mignon", Label: "viande 1"}, "viande 1"},
	}
	for _, tt := range tests {
		if got := labelSuffix(tt.it); got != tt.want {
			t.Errorf("labelSuffix(%q, %q) = %q, want %q", tt.it.Name, tt.it.Label, got, tt.want)
		}
	}
}

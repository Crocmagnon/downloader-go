package recettescabas

import (
	"maps"
	"slices"
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
	return slices.EqualFunc(a, b, slices.Equal)
}

func fourItems() []Item {
	return []Item{
		{N: 1, Name: "Wok de légumes, nouilles soba"},
		{N: 2, Name: "Filet mignon aux pommes - pommes de terre", Selected: true},
		{N: 4, Name: "Gratin champignons-boulgour", Flags: []Hit{{"🍄", "champignons"}}},
		{N: 10, Name: "Tarte fondante aux noix", Selection: "pas dans le bouton"},
	}
}

func TestNewKeyboard(t *testing.T) {
	kb := NewKeyboard(fourItems(), false)
	want := [][]string{
		{"1. Wok de légumes, nouilles soba"},
		{"✅ 2. Filet mignon aux pommes"},
		{"4. 🍄 Gratin champignons-boulgour"},
		{"10. Tarte fondante aux noix"},
		{"🌐 Ouvrir sur le site"},
	}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
	if kb[3][0].CallbackData != "t:10" {
		t.Errorf("callback = %q", kb[3][0].CallbackData)
	}
	if len(Pending(kb)) != 0 {
		t.Error("fresh keyboard has no pending change")
	}
	if site := kb[4][0]; site.URL != prefURL || site.CallbackData != "" {
		t.Errorf("site button = %+v", site)
	}
}

func TestToggleStagesAndUnstages(t *testing.T) {
	kb := Toggle(Toggle(NewKeyboard(fourItems(), false), 4), 2)

	want := [][]string{
		{"1. Wok de légumes, nouilles soba"},
		{"➖ 2. Filet mignon aux pommes"},
		{"➕ 4. 🍄 Gratin champignons-boulgour"},
		{"10. Tarte fondante aux noix"},
		{"✔️ Valider (2)", "✖️ Annuler"},
		{"🌐 Ouvrir sur le site"},
	}
	if got := texts(kb); !equalRows(got, want) {
		t.Fatalf("keyboard = %v, want %v", got, want)
	}
	if got, want := Pending(kb), map[int]bool{2: false, 4: true}; !maps.Equal(got, want) {
		t.Errorf("pending = %v, want %v", got, want)
	}
	if kb[4][0].CallbackData != cbValidate || kb[4][1].CallbackData != cbCancel {
		t.Errorf("action callbacks = %q, %q", kb[4][0].CallbackData, kb[4][1].CallbackData)
	}

	// Pressing again cancels the staged removal of 2: back to selected, title kept.
	kb = Toggle(kb, 2)
	if got := kb[1][0].Text; got != "✅ 2. Filet mignon aux pommes" {
		t.Errorf("button 2 = %q", got)
	}
	if got, want := Pending(kb), map[int]bool{4: true}; !maps.Equal(got, want) {
		t.Errorf("pending = %v, want %v", got, want)
	}
}

func TestDetailKeyboard(t *testing.T) {
	kb := DetailKeyboard(fourItems(), false)
	want := [][]string{
		{"1. Wok de légumes, nouilles soba"},
		{"2. Filet mignon aux pommes"},
		{"4. 🍄 Gratin champignons-boulgour"},
		{"10. Tarte fondante aux noix"},
	}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
	if kb[3][0].CallbackData != "d:10" {
		t.Errorf("callback = %q", kb[3][0].CallbackData)
	}
}

func TestReset(t *testing.T) {
	kb := Reset(Toggle(Toggle(NewKeyboard(fourItems(), false), 4), 2))
	if got, want := texts(kb), texts(NewKeyboard(fourItems(), false)); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
}

func TestToggleUnknownNumberIsNoop(t *testing.T) {
	kb := Toggle(NewKeyboard(fourItems(), false), 42)
	if len(Pending(kb)) != 0 {
		t.Error("toggling an unknown number must not stage anything")
	}
}

func TestCompactKeyboard(t *testing.T) {
	items := []Item{
		{N: 1, Name: "Wok"}, {N: 2, Name: "Filet", Selected: true}, {N: 3}, {N: 4, Flags: []Hit{{"🍄", "champignons"}}}, {N: 5},
		{N: 6, Flags: []Hit{{"🥗", "salade"}}}, {N: 9, Selected: true, Flags: []Hit{{"🍄", "champignons"}}},
		{N: 10}, {N: 11},
	}
	kb := Toggle(Toggle(NewKeyboard(items, true), 4), 9)
	want := [][]string{{"1", "✅ 2", "3", "➕ 4🍄", "5"}, {"6🥗", "➖ 9🍄"}, {"10", "11"}, {"✔️ Valider (2)", "✖️ Annuler"}, {"🌐 Ouvrir sur le site"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
	if got, want := Pending(kb), map[int]bool{4: true, 9: false}; !maps.Equal(got, want) {
		t.Errorf("pending = %v, want %v", got, want)
	}

	if got, want := texts(DetailKeyboard(items[:3], true)), [][]string{{"1", "2", "3"}}; !equalRows(got, want) {
		t.Errorf("detail keyboard = %v, want %v", got, want)
	}
}

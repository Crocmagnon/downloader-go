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

func sixItems() []Item {
	return []Item{{N: 1}, {N: 2, Selected: true}, {N: 3}, {N: 4, Selected: true}, {N: 5}, {N: 10}}
}

func TestNewKeyboard(t *testing.T) {
	kb := NewKeyboard(sixItems())
	want := [][]string{{"1", "✅2", "3", "✅4", "5"}, {"10"}, {"🌐 Ouvrir sur le site"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
	if kb[1][0].CallbackData != "t:10" {
		t.Errorf("callback = %q", kb[1][0].CallbackData)
	}
	if len(Pending(kb)) != 0 {
		t.Error("fresh keyboard has no pending change")
	}
	if site := kb[2][0]; site.URL != prefURL || site.CallbackData != "" {
		t.Errorf("site button = %+v", site)
	}
}

func TestToggleStagesAndUnstages(t *testing.T) {
	kb := Toggle(Toggle(NewKeyboard(sixItems()), 3), 2)

	want := [][]string{{"1", "➖2", "➕3", "✅4", "5"}, {"10"}, {"✔️ Valider (2)", "✖️ Annuler"}, {"🌐 Ouvrir sur le site"}}
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
	want = [][]string{{"1", "✅2", "➕3", "✅4", "5"}, {"10"}, {"✔️ Valider (1)", "✖️ Annuler"}, {"🌐 Ouvrir sur le site"}}
	if got := texts(kb); !equalRows(got, want) {
		t.Errorf("keyboard = %v, want %v", got, want)
	}
}

func TestReset(t *testing.T) {
	kb := Reset(Toggle(Toggle(NewKeyboard(sixItems()), 3), 2))
	want := [][]string{{"1", "✅2", "3", "✅4", "5"}, {"10"}, {"🌐 Ouvrir sur le site"}}
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

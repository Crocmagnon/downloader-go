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

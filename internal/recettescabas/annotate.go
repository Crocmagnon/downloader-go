package recettescabas

import (
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
			if word := match(it.Ingredients+" ; "+it.Pantry, f.Keywords); word != "" {
				it.Flags = append(it.Flags, Hit{Emoji: f.Emoji, Word: word})
			}
		}

		items = append(items, it)
	}

	p.Items = items

	return p
}

// alerts returns the selected items that carry a flag.
func alerts(p Page) []Item {
	var out []Item
	for _, it := range p.Items {
		if it.Selected && len(it.Flags) > 0 {
			out = append(out, it)
		}
	}

	return out
}

func emojis(it Item) string {
	var s string
	for _, h := range it.Flags {
		s += h.Emoji
	}

	return s
}

// labelSuffix returns the item's label, minus its leading copy of the name: option labels
// often repeat it ("Fruits Petit Format (9.10€)" for "Fruits Petit Format").
func labelSuffix(it Item) string {
	rest, ok := strings.CutPrefix(normalize(it.Label), normalize(it.Name))
	if !ok {
		return it.Label
	}

	// normalize keeps the rune count (except œ/æ, rare in labels), so the tail of the
	// original label lines up with rest.
	runes := []rune(it.Label)

	tail := strings.TrimSpace(string(runes[max(0, len(runes)-len([]rune(rest))):]))

	return strings.TrimSuffix(strings.TrimPrefix(tail, "("), ")") // Home Assistant shows the label on its own
}

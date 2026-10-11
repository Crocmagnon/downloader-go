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

package recettescabas

import (
	"fmt"
	"html"
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
			if word := match(it.Ingredients+" ; "+it.Pantry, f.Keywords); word != "" {
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

	// Texts are sent as HTML: everything from the site is escaped.
	esc := html.EscapeString

	fmt.Fprintf(&b, "<b>🧺 Semaine du %s</b>\nModifiable jusqu'au %s\n", esc(p.Week), esc(p.Deadline))
	b.WriteString(esc(p.People))
	if p.Price != "" {
		fmt.Fprintf(&b, " · %s €", esc(p.Price))
	}

	var warnings []string
	for _, it := range alerts(p) {
		warnings = append(warnings, emojis(it)+" "+strconv.Itoa(it.N))
	}
	if len(warnings) > 0 {
		b.WriteString("\n⚠️ Sélection : " + strings.Join(warnings, ", "))
	}

	section := func(title string, option, selected bool) {
		var lines []string
		for _, it := range p.Items {
			if it.Option == option && it.Selected == selected {
				lines = append(lines, line(it))
			}
		}
		if len(lines) > 0 {
			b.WriteString("\n\n<b>" + title + "</b>\n" + strings.Join(lines, "\n"))
		}
	}
	section("✅ Recettes sélectionnées", false, true)
	section("➕ Recettes disponibles", false, false)
	section("✅ Options sélectionnées", true, true)
	section("➕ Options disponibles", true, false)

	return b.String()
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

func DetailText(it Item) string {
	s := line(it)
	for _, b := range []struct{ title, text string }{
		{"Ingrédients du cabas", it.Ingredients},
		{"Ingrédients du placard", it.Pantry},
	} {
		if b.text != "" {
			s += "\n\n<b>" + b.title + "</b>\n" + html.EscapeString(b.text)
		}
	}

	if it.Ingredients == "" && it.Pantry == "" {
		s += "\n\n(pas d'ingrédients indiqués)"
	}

	return s
}

// line renders "9. 🍄 Poulet (viande 2) — <b>champignons</b>" as HTML.
func line(it Item) string {
	s := strconv.Itoa(it.N) + ". "
	if len(it.Flags) > 0 {
		s += emojis(it) + " "
	}
	s += html.EscapeString(it.Name)
	if label := labelSuffix(it); label != "" {
		s += " (" + html.EscapeString(label) + ")"
	}

	if len(it.Flags) > 0 {
		words := make([]string, 0, len(it.Flags))
		for _, h := range it.Flags {
			words = append(words, "<b>"+html.EscapeString(h.Word)+"</b>")
		}
		s += " — " + strings.Join(words, ", ")
	}

	if it.Selection != "" {
		s += "\n    ↳ " + html.EscapeString(it.Selection)
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

	return strings.TrimSuffix(strings.TrimPrefix(tail, "("), ")") // line adds its own parentheses
}

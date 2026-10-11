package recettescabas

import (
	"fmt"
	"strconv"
	"strings"
)

// Button is a Telegram inline keyboard button: either a callback or a link.
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitzero"`
	URL          string `json:"url,omitzero"`
}

const (
	cbToggle   = "t:"
	cbValidate = "v"
	cbCancel   = "x"
	cbBusy     = "-" // "⏳ application…" placeholder, ignored
	cbDetail   = "d:"
	cbStyle    = "s:" // /affichage: "s:n" numbers, "s:t" titles
	perRow     = 5    // buttons per row with numbers only
)

// cell is the state of one item button. The keyboard is the only place pending
// changes are stored: they're read back from button labels on each callback.
type cell struct {
	n        int
	selected bool
	pending  bool
	title    string // what follows the number: ". 🍄 Gratin champignons", or just "🍄" when compact
}

// NewKeyboard shows each item with its title, or with its number only when compact.
func NewKeyboard(items []Item, compact bool) [][]Button {
	return build(cellsFrom(items, compact))
}

// DetailKeyboard lets /detail pick an item with a tap.
func DetailKeyboard(items []Item, compact bool) [][]Button {
	return grid(cellsFrom(items, compact), func(c cell) Button {
		return Button{Text: strconv.Itoa(c.n) + c.title, CallbackData: cbDetail + strconv.Itoa(c.n)}
	})
}

func cellsFrom(items []Item, compact bool) []cell {
	cells := make([]cell, 0, len(items))
	for _, it := range items {
		title := buttonTitle(it)
		if compact {
			title = emojis(it)
		}

		cells = append(cells, cell{n: it.N, selected: it.Selected, title: title})
	}

	return cells
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
	rows := grid(cells, func(c cell) Button {
		return Button{Text: label(c), CallbackData: cbToggle + strconv.Itoa(c.n)}
	})

	pending := 0
	for _, c := range cells {
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

	return append(rows, []Button{{Text: "🌐 Ouvrir sur le site", URL: prefURL}})
}

// grid lays out one button per row when they have titles, which need the whole width.
// Numbers only go perRow per row, options starting on their own row.
func grid(cells []cell, button func(cell) Button) [][]Button {
	var rows [][]Button

	for i, c := range cells {
		newOptions := c.n >= firstOptionN && (i == 0 || cells[i-1].n < firstOptionN)
		if len(rows) == 0 || hasTitle(c) || len(rows[len(rows)-1]) == perRow || newOptions {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], button(c))
	}

	return rows
}

// hasTitle tells the two layouts apart, which also works on a keyboard read back from Telegram.
func hasTitle(c cell) bool {
	return strings.HasPrefix(c.title, ". ")
}

// buttonTitle renders ". 🍄 Gratin champignons-boulgour": flags, then the name up to its
// first " - " (the side dish), which Telegram would truncate anyway.
func buttonTitle(it Item) string {
	name, _, _ := strings.Cut(it.Name, " - ")
	if f := emojis(it); f != "" {
		return ". " + f + " " + name
	}

	return ". " + name
}

func label(c cell) string {
	n := strconv.Itoa(c.n) + c.title

	switch {
	case c.selected && c.pending:
		return "➖ " + n
	case c.selected:
		return "✅ " + n
	case c.pending:
		return "➕ " + n
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

			// The state emoji has no digit, so the first nStr is the number.
			_, title, _ := strings.Cut(b.Text, nStr)
			c := cell{n: n, title: title}
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

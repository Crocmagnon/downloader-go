package recettescabas

import (
	"fmt"
	"strconv"
	"strings"
)

// Button is a Telegram inline keyboard button.
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

const (
	cbToggle   = "t:"
	cbValidate = "v"
	cbCancel   = "x"
	perRow     = 5
)

// cell is the state of one item button. The keyboard is the only place pending
// changes are stored: they're read back from button labels on each callback.
type cell struct {
	n        int
	selected bool
	pending  bool
}

func NewKeyboard(items []Item) [][]Button {
	cells := make([]cell, 0, len(items))
	for _, it := range items {
		cells = append(cells, cell{n: it.N, selected: it.Selected})
	}

	return build(cells)
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
	var rows [][]Button

	pending := 0
	for i, c := range cells {
		if i%perRow == 0 {
			rows = append(rows, nil)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], Button{Text: label(c), CallbackData: cbToggle + strconv.Itoa(c.n)})

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

	return rows
}

func label(c cell) string {
	n := strconv.Itoa(c.n)

	switch {
	case c.selected && c.pending:
		return "➖" + n
	case c.selected:
		return "✅" + n
	case c.pending:
		return "➕" + n
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

			c := cell{n: n}
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

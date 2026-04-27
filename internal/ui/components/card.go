package components

import (
	"strings"

	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// Card is a bordered, padded box used for entity headers (workflow,
// activity, resource).
type Card struct {
	Title string
	Lines []CardLine
	Width int
}

// CardLine is one labeled value in a card.
type CardLine struct {
	Label string
	Value string
}

func (c Card) View() string {
	if c.Width <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(styles.Title.Render(c.Title))
	b.WriteString("\n")
	for _, l := range c.Lines {
		b.WriteString(styles.Faint.Render(rightPad(l.Label, 14)))
		b.WriteString(l.Value)
		b.WriteString("\n")
	}
	return styles.Card.Width(c.Width - 2).Render(strings.TrimRight(b.String(), "\n"))
}

func rightPad(s string, w int) string {
	if len(s) >= w {
		return s + " "
	}
	return s + strings.Repeat(" ", w-len(s))
}

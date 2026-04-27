package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// Help renders a modal-style overlay listing global + screen bindings.
type Help struct {
	Globals []key.Binding
	Screen  []key.Binding
	Width   int
	Height  int
}

func (h Help) View() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(styles.Title.Render("Keybindings"))
	b.WriteString("\n\n")

	if len(h.Screen) > 0 {
		b.WriteString(styles.Bold.Render("Screen") + "\n")
		writeBindings(&b, h.Screen)
		b.WriteString("\n")
	}
	b.WriteString(styles.Bold.Render("Global") + "\n")
	writeBindings(&b, h.Globals)
	b.WriteString("\n")
	b.WriteString(styles.Faint.Render("Press ? or Esc to close"))

	rendered := box.Render(b.String())
	if h.Width <= 0 || h.Height <= 0 {
		return rendered
	}
	return lipgloss.Place(h.Width, h.Height, lipgloss.Center, lipgloss.Center, rendered)
}

func writeBindings(b *strings.Builder, bindings []key.Binding) {
	for _, k := range bindings {
		hh := k.Help()
		if hh.Key == "" {
			continue
		}
		b.WriteString("  ")
		b.WriteString(styles.Bold.Render(pad(hh.Key, 12)))
		b.WriteString("  ")
		b.WriteString(hh.Desc)
		b.WriteString("\n")
	}
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

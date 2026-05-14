package components

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// Footer renders the bottom chrome — keybinding hints with optional
// transient toast or persistent perf strip.
type Footer struct {
	Bindings  []key.Binding
	Width     int
	Toast     string
	ToastErr  bool
	ToastTime time.Time
	ToastTTL  time.Duration
	ModeHint  string // e.g., ":command" or "/filter"
	Perf      string // right-aligned status when --perf is on; toast wins
}

func (f Footer) View() string {
	if f.Width <= 0 {
		return ""
	}

	if f.ModeHint != "" {
		return lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(styles.Border).
			Width(f.Width).
			Render(f.ModeHint)
	}

	hints := f.renderBindings()

	right := ""
	if f.Toast != "" && time.Since(f.ToastTime) < f.ToastTTL {
		toastStyle := styles.ToastOK
		if f.ToastErr {
			toastStyle = styles.ToastErr
		}
		right = toastStyle.Render(f.Toast)
	} else if f.Perf != "" {
		right = styles.Faint.Render(f.Perf)
	}
	if right != "" {
		gap := f.Width - lipgloss.Width(hints) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		hints = hints + strings.Repeat(" ", gap) + right
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(styles.Border).
		Width(f.Width).
		Render(hints)
}

func (f Footer) renderBindings() string {
	if len(f.Bindings) == 0 {
		return styles.Hint.Render("?·help  q·quit")
	}
	parts := []string{}
	for _, b := range f.Bindings {
		h := b.Help()
		if h.Key == "" {
			continue
		}
		parts = append(parts, styles.Hint.Render(h.Key+"·"+h.Desc))
	}
	parts = append(parts, styles.Hint.Render("?·help"), styles.Hint.Render("q·quit"))
	return strings.Join(parts, "  ")
}

// Package components has the chrome shared by every screen.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// Header renders the top chrome strip with context + status counts.
type Header struct {
	BaseURL string
	Counts  api.StatusCounts
	Width   int
	Title   string // current screen title (right-aligned)
}

func (h Header) View() string {
	if h.Width <= 0 {
		return ""
	}

	left := lipgloss.NewStyle().Bold(true).Render("orchard") + styles.Faint.Render(" • "+h.BaseURL)

	parts := []string{}
	for _, s := range []api.Status{
		api.StatusPending,
		api.StatusRunning,
		api.StatusFinished,
		api.StatusCanceled,
		api.StatusFailed,
	} {
		if c, ok := h.Counts[s]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s%d", styles.StatusGlyphColored(s), c))
		}
	}
	mid := strings.Join(parts, " ")

	right := ""
	if h.Title != "" {
		right = styles.Faint.Render("[" + h.Title + "]")
	}

	bar := joinSpread(h.Width, left, mid, right)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(styles.Border).
		Width(h.Width).
		Render(bar)
}

// joinSpread places left, mid (centered), and right inside width.
func joinSpread(width int, left, mid, right string) string {
	lw := lipgloss.Width(left)
	mw := lipgloss.Width(mid)
	rw := lipgloss.Width(right)

	gap := width - lw - mw - rw
	if gap < 2 {
		// Too narrow to show all three; drop the right segment first.
		gap = width - lw - mw
		if gap < 2 {
			return truncRight(left, width)
		}
		return left + strings.Repeat(" ", gap) + mid
	}
	leftGap := gap / 2
	rightGap := gap - leftGap
	return left + strings.Repeat(" ", leftGap) + mid + strings.Repeat(" ", rightGap) + right
}

func truncRight(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	// Lipgloss-aware truncation could use ansi.Truncate, but for a
	// header that overflows we'd rather just hide the surplus.
	r := []rune(s)
	if len(r) > w {
		r = r[:w]
	}
	return string(r)
}

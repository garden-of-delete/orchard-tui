// Package styles centralizes lipgloss styles and status color/glyph mappings.
package styles

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/api"
)

// Adaptive colors so the palette works in both light and dark terminals.
var (
	Primary   = lipgloss.AdaptiveColor{Light: "#0288D1", Dark: "#4FC3F7"}
	Subtle    = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	Border    = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"}
	Highlight = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F9FAFB"}
	Success   = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#66BB6A"}
	Error     = lipgloss.AdaptiveColor{Light: "#D32F2F", Dark: "#EF5350"}
	Warning   = lipgloss.AdaptiveColor{Light: "#ED7C02", Dark: "#FFA726"}
	Info      = lipgloss.AdaptiveColor{Light: "#0288D1", Dark: "#29B6F6"}
	Muted     = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
)

// Common reusable styles.
var (
	Title    = lipgloss.NewStyle().Bold(true).Foreground(Highlight)
	Bold     = lipgloss.NewStyle().Bold(true)
	Faint    = lipgloss.NewStyle().Foreground(Subtle)
	Hint     = lipgloss.NewStyle().Foreground(Subtle)
	HeaderBG = lipgloss.NewStyle().Bold(true).Foreground(Highlight)
	Card     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Border).Padding(0, 1)
	ToastErr = lipgloss.NewStyle().Foreground(Error).Bold(true)
	ToastOK  = lipgloss.NewStyle().Foreground(Success)
)

// StatusGlyph returns the single-rune indicator for a status.
func StatusGlyph(s api.Status) string {
	switch s {
	case api.StatusPending:
		return "⏸"
	case api.StatusActivating, api.StatusDeactivating, api.StatusShuttingDown, api.StatusCanceling:
		return "…"
	case api.StatusRunning:
		return "▶"
	case api.StatusFinished:
		return "✓"
	case api.StatusFailed, api.StatusCascadeFailed, api.StatusTimeout:
		return "✗"
	case api.StatusCanceled:
		return "✖"
	case api.StatusDeleted:
		return "·"
	}
	return "?"
}

// StatusColor returns the adaptive color used for a status.
func StatusColor(s api.Status) lipgloss.AdaptiveColor {
	switch s {
	case api.StatusPending:
		return Muted
	case api.StatusActivating, api.StatusDeactivating, api.StatusShuttingDown, api.StatusCanceling:
		return Subtle
	case api.StatusRunning:
		return Info
	case api.StatusFinished:
		return Success
	case api.StatusFailed, api.StatusCascadeFailed, api.StatusTimeout:
		return Error
	case api.StatusCanceled:
		return Warning
	case api.StatusDeleted:
		return Subtle
	}
	return Subtle
}

// StatusPill renders "<glyph> <status>" colored.
func StatusPill(s api.Status) string {
	return lipgloss.NewStyle().
		Foreground(StatusColor(s)).
		Render(StatusGlyph(s) + " " + string(s))
}

// StatusGlyphColored returns just the glyph in the status color.
func StatusGlyphColored(s api.Status) string {
	return lipgloss.NewStyle().Foreground(StatusColor(s)).Render(StatusGlyph(s))
}

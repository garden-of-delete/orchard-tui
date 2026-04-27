// Package poll provides ticker helpers for screen-driven polling.
package poll

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Tick returns a tea.Cmd that fires after d, producing the message returned
// by mk. d <= 0 returns nil (no polling).
func Tick(d time.Duration, mk func(time.Time) tea.Msg) tea.Cmd {
	if d <= 0 {
		return nil
	}
	return tea.Tick(d, mk)
}

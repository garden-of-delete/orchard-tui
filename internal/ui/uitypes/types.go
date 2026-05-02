// Package uitypes is a leaf package shared between the ui orchestrator and
// individual screens. Keeping these declarations here avoids an import
// cycle (the ui package imports screens to construct them; screens emit
// messages defined here).
package uitypes

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/garden-of-delete/orchard-tui/internal/api"
)

// Screen is implemented by every push-able view.
type Screen interface {
	tea.Model

	ID() string                  // stable id for tick correlation
	Title() string               // breadcrumb-friendly label
	Refresh() tea.Cmd            // re-fetch data immediately
	PollInterval() time.Duration // 0 disables auto-polling
	KeyMap() []key.Binding       // screen-local keybindings
	SetSize(w, h int)            // body area resize
}

// Mode describes the App's current input mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeCommand
	ModeFilter
	ModeHelp
)

// ToastLevel categorizes transient footer messages.
type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastOK
	ToastErr
)

// PollTickMsg fires when a screen's poll interval elapses.
type PollTickMsg struct {
	ScreenID string
	Time     time.Time
}

// CountsTickMsg fires for the background header-counts refresher.
type CountsTickMsg struct{ Time time.Time }

// CountsLoadedMsg carries the result of a /v1/stats/counts call. Seq
// matches the App's countsSeq at issue time; older loaded msgs are
// dropped on arrival to avoid stale-overwrites.
type CountsLoadedMsg struct {
	Seq    int
	Counts api.StatusCounts
	Err    error
}

// PushScreenMsg asks the App to push a new screen.
type PushScreenMsg struct{ Screen Screen }

// PopScreenMsg asks the App to pop the active screen.
type PopScreenMsg struct{}

// ReplaceScreenMsg swaps the top screen.
type ReplaceScreenMsg struct{ Screen Screen }

// ToastMsg displays a transient message in the footer.
type ToastMsg struct {
	Level ToastLevel
	Text  string
	TTL   time.Duration
}

// ClearToastMsg fires after a toast's TTL elapses. The App carries a
// monotonic toast ID and only clears the visible toast if ID matches
// the current one — so a newer toast displayed before the old one's TTL
// elapses isn't wiped by the older toast's stale clear message.
type ClearToastMsg struct{ ID int }

// FilterEnterMsg fires when the filter input opens. Screens should
// snapshot their current filter so FilterCancelMsg can restore it.
type FilterEnterMsg struct{}

// FilterChangedMsg notifies the active screen as the user types.
type FilterChangedMsg struct{ Query string }

// FilterCommittedMsg fires when the user presses enter on the filter input.
type FilterCommittedMsg struct{ Query string }

// FilterCancelMsg fires when the user esc's the filter input without
// committing. Screens should restore the filter snapshot taken at
// FilterEnterMsg — this preserves the previously-committed filter
// rather than discarding it (vim-like cancel semantics).
type FilterCancelMsg struct{}

// FilterClearedMsg tells the active screen to drop its filter
// unconditionally (used when esc is pressed at the root with no screen
// to pop — clears any committed filter).
type FilterClearedMsg struct{}

// RequestRefreshMsg asks the active screen to fetch immediately.
type RequestRefreshMsg struct{}

// Convenience tea.Cmd constructors.

func Push(s Screen) tea.Cmd    { return func() tea.Msg { return PushScreenMsg{Screen: s} } }
func Pop() tea.Cmd             { return func() tea.Msg { return PopScreenMsg{} } }
func Replace(s Screen) tea.Cmd { return func() tea.Msg { return ReplaceScreenMsg{Screen: s} } }
func Toast(l ToastLevel, t string) tea.Cmd {
	return func() tea.Msg { return ToastMsg{Level: l, Text: t, TTL: 3 * time.Second} }
}

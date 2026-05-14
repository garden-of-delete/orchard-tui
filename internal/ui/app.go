// Package ui is the orchard-tui Bubble Tea root model. It owns chrome,
// global keybindings, the navigation stack, and the background header
// counts ticker. Per-screen logic lives in internal/ui/screens.
package ui

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/config"
	"github.com/garden-of-delete/orchard-tui/internal/format"
	"github.com/garden-of-delete/orchard-tui/internal/ui/components"
	"github.com/garden-of-delete/orchard-tui/internal/ui/nav"
	"github.com/garden-of-delete/orchard-tui/internal/ui/poll"
	"github.com/garden-of-delete/orchard-tui/internal/ui/screens"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

const (
	headerHeight     = 2
	footerHeight     = 2
	chromeReserveAll = headerHeight + footerHeight
)

// App is the root tea.Model.
type App struct {
	cfg    config.Config
	client *api.Client

	stack   nav.Stack[uitypes.Screen]
	counts  api.StatusCounts
	mode    uitypes.Mode
	cmdbar  components.CmdBar
	filter  components.CmdBar // re-uses cmdbar with prefix "/"
	w, h    int
	focused bool

	toast    string
	toastErr bool
	toastAt  time.Time
	toastTTL time.Duration
	toastID  int // monotonic; matched against ClearToastMsg.ID

	// countsErrShown latches so we toast once per outage, not on every retry.
	// Reset on the next successful counts fetch.
	countsErrShown bool
	countsSeq      int // monotonic per-fetch seq for /v1/stats/counts

	globals []key.Binding

	perfEnabled bool
	perfPrev    perfSnap
	perfCur     perfSnap
}

type perfSnap struct {
	at         time.Time
	requests   uint64
	bytes      uint64
	heapMB     uint64
	goroutines int
}

// New creates the root model.
func New(cfg config.Config) *App {
	a := &App{
		cfg:    cfg,
		client: api.New(cfg.Host, cfg.APIKey),
		cmdbar: components.NewCmdBar(":"),
		filter: components.NewCmdBar("/"),
		// focused defaults to true: terminals that don't emit focus
		// events (basic xterm, some serial consoles) leave us at this
		// initial value forever, which is the safe choice — polling
		// keeps running. Modern terminals (kitty/iTerm/alacritty/etc.)
		// will toggle this via tea.WithReportFocus().
		focused: true,
	}
	a.globals = a.globalBindings()

	first := screens.NewWorkflows(a.client, nil, cfg.PollFast, cfg.PollMedium, cfg.PollSlow)
	a.stack.Push(first)
	return a
}

// EnablePerf turns on the perf strip in the footer (--perf flag).
func (a *App) EnablePerf() { a.perfEnabled = true }

// Init starts the active screen and the background counts ticker.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		a.activeInitCmd(),
		a.activeTickCmd(),
		a.fetchCountsCmd(),
		a.countsTickCmd(),
	}
	return tea.Batch(cmds...)
}

// Update is the central message router.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {

	case tea.WindowSizeMsg:
		a.w, a.h = m.Width, m.Height
		a.cmdbar.SetWidth(m.Width)
		a.filter.SetWidth(m.Width)
		a.propagateSize()
		return a, nil

	case tea.FocusMsg:
		a.focused = true
		return a, nil
	case tea.BlurMsg:
		a.focused = false
		return a, nil

	case uitypes.PushScreenMsg:
		a.stack.Push(m.Screen)
		a.propagateSize()
		return a, tea.Batch(m.Screen.Init(), a.tickFor(m.Screen))

	case uitypes.PopScreenMsg:
		if a.stack.Len() > 1 {
			a.stack.Pop()
			a.propagateSize()
			if top, ok := a.stack.Top(); ok {
				return a, a.tickFor(top)
			}
		}
		return a, nil

	case uitypes.ReplaceScreenMsg:
		a.stack.Replace(m.Screen)
		a.propagateSize()
		return a, tea.Batch(m.Screen.Init(), a.tickFor(m.Screen))

	case uitypes.ToastMsg:
		a.toastID++
		a.toast = m.Text
		a.toastErr = m.Level == uitypes.ToastErr
		a.toastAt = time.Now()
		a.toastTTL = m.TTL
		return a, a.scheduleToastClear(a.toastID, m.TTL)

	case uitypes.ClearToastMsg:
		// Only clear if no newer toast has overwritten ours.
		if m.ID == a.toastID {
			a.toast = ""
		}
		return a, nil

	case uitypes.CountsTickMsg:
		if a.perfEnabled {
			a.refreshPerf()
		}
		if a.focused {
			return a, tea.Batch(a.fetchCountsCmd(), a.countsTickCmd())
		}
		return a, a.countsTickCmd()

	case uitypes.CountsLoadedMsg:
		if m.Seq < a.countsSeq {
			return a, nil // stale
		}
		if m.Err != nil {
			if a.countsErrShown {
				return a, nil
			}
			a.countsErrShown = true
			return a, uitypes.Toast(uitypes.ToastErr, "header counts: "+truncErr(m.Err))
		}
		a.counts = m.Counts
		a.countsErrShown = false
		return a, nil

	case uitypes.PollTickMsg:
		// Pause polling on blur; just continue the ticker so we resume
		// on focus.
		if !a.focused {
			top, _ := a.stack.Top()
			if top != nil && top.ID() == m.ScreenID {
				return a, a.tickFor(top)
			}
			return a, nil
		}
		return a, a.delegate(msg)

	case tea.KeyMsg:
		if cmd, handled := a.handleKey(m); handled {
			return a, cmd
		}
		return a, a.delegate(msg)
	}

	return a, a.delegate(msg)
}

// View composes header + body + footer (or mode-specific bottom bar).
func (a *App) View() string {
	if a.w == 0 || a.h == 0 {
		return ""
	}

	header := components.Header{
		BaseURL: a.cfg.Host,
		Counts:  a.counts,
		Width:   a.w,
		Title:   a.activeTitle(),
	}.View()

	body := a.renderBody()
	if a.mode == uitypes.ModeHelp {
		// Render help inside the body area so the header (status counts)
		// and footer remain visible behind it.
		body = components.Help{
			Globals: a.globals,
			Screen:  a.activeKeyMap(),
			Width:   a.w,
			Height:  a.bodyHeight(),
		}.View()
	}

	footer := a.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// bodyHeight returns the height available for screen body content, after
// reserving rows for the header and footer chrome.
func (a *App) bodyHeight() int {
	h := a.h - chromeReserveAll
	if h < 1 {
		h = 1
	}
	return h
}

// --- chrome rendering ---

func (a *App) renderBody() string {
	body := ""
	if top, ok := a.stack.Top(); ok {
		body = top.View()
	}
	return body
}

func (a *App) renderFooter() string {
	switch a.mode {
	case uitypes.ModeCommand:
		return components.Footer{
			Width:    a.w,
			ModeHint: a.cmdbar.View(),
		}.View()
	case uitypes.ModeFilter:
		return components.Footer{
			Width:    a.w,
			ModeHint: a.filter.View(),
		}.View()
	}
	return components.Footer{
		Width:     a.w,
		Bindings:  a.activeKeyMap(),
		Toast:     a.toast,
		ToastErr:  a.toastErr,
		ToastTime: a.toastAt,
		ToastTTL:  a.toastTTL,
		Perf:      a.perfLine(),
	}.View()
}

func (a *App) refreshPerf() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s := a.client.Stats()
	a.perfPrev = a.perfCur
	a.perfCur = perfSnap{
		at:         time.Now(),
		requests:   s.Requests,
		bytes:      s.ResponseBytes,
		heapMB:     ms.HeapAlloc / (1024 * 1024),
		goroutines: runtime.NumGoroutine(),
	}
}

func (a *App) perfLine() string {
	if !a.perfEnabled {
		return ""
	}
	if a.perfCur.at.IsZero() {
		return "perf: warming up"
	}
	elapsed := a.perfCur.at.Sub(a.perfPrev.at).Seconds()
	if elapsed < 0.1 {
		return fmt.Sprintf("perf: heap=%dMB gor=%d", a.perfCur.heapMB, a.perfCur.goroutines)
	}
	reqRate := float64(a.perfCur.requests-a.perfPrev.requests) / elapsed
	byteRate := float64(a.perfCur.bytes-a.perfPrev.bytes) / elapsed / 1024
	return fmt.Sprintf("perf: %.1freq/s %.1fKB/s heap=%dMB gor=%d",
		reqRate, byteRate, a.perfCur.heapMB, a.perfCur.goroutines)
}

func (a *App) activeKeyMap() []key.Binding {
	if top, ok := a.stack.Top(); ok {
		return top.KeyMap()
	}
	return nil
}

// activeScreenBindsKey reports whether the active screen's KeyMap
// advertises the given key (e.g., "/"). Used to gate global keys
// that only make sense when the screen subscribes to their messages.
func (a *App) activeScreenBindsKey(want string) bool {
	for _, b := range a.activeKeyMap() {
		for _, k := range b.Keys() {
			if k == want {
				return true
			}
		}
	}
	return false
}

func (a *App) activeTitle() string {
	if top, ok := a.stack.Top(); ok {
		return top.Title()
	}
	return ""
}

// --- key handling ---

func (a *App) handleKey(m tea.KeyMsg) (tea.Cmd, bool) {
	switch a.mode {
	case uitypes.ModeCommand:
		switch m.String() {
		case "esc":
			a.cmdbar.Reset()
			a.mode = uitypes.ModeNormal
			return nil, true
		case "enter":
			cmd := a.parseCommand(a.cmdbar.Value())
			a.cmdbar.Reset()
			a.mode = uitypes.ModeNormal
			return cmd, true
		case "tab":
			completed, matches := CompleteCommand(a.cmdbar.Value())
			a.cmdbar.SetValue(completed)
			if len(matches) > 1 {
				return uitypes.Toast(uitypes.ToastInfo, strings.Join(matches, "  ")), true
			}
			return nil, true
		}
		cmd := a.cmdbar.Update(m)
		return cmd, true

	case uitypes.ModeFilter:
		switch m.String() {
		case "esc":
			a.filter.Reset()
			a.mode = uitypes.ModeNormal
			// Restore the screen's pre-filter snapshot rather than
			// clearing — vim-like cancel.
			return func() tea.Msg { return uitypes.FilterCancelMsg{} }, true
		case "enter":
			q := a.filter.Value()
			a.filter.Reset()
			a.mode = uitypes.ModeNormal
			return func() tea.Msg { return uitypes.FilterCommittedMsg{Query: q} }, true
		}
		cmd := a.filter.Update(m)
		// Propagate live filter value as it changes.
		val := a.filter.Value()
		return tea.Batch(cmd, func() tea.Msg { return uitypes.FilterChangedMsg{Query: val} }), true

	case uitypes.ModeHelp:
		switch m.String() {
		case "esc", "?", "q":
			a.mode = uitypes.ModeNormal
			return nil, true
		}
		return nil, true
	}

	// ModeNormal
	switch m.String() {
	case "ctrl+c", "q":
		return tea.Quit, true
	case ":":
		a.cmdbar.Reset()
		a.cmdbar.SetWidth(a.w)
		a.mode = uitypes.ModeCommand
		return nil, true
	case "/":
		// Only enter filter mode if the active screen advertises `/`
		// in its key map — otherwise the input bar would intercept
		// keystrokes that the screen has nothing to do with.
		if !a.activeScreenBindsKey("/") {
			return nil, true
		}
		a.filter.Reset()
		a.filter.SetWidth(a.w)
		a.mode = uitypes.ModeFilter
		// Tell the active screen to snapshot its filter so esc can restore.
		return func() tea.Msg { return uitypes.FilterEnterMsg{} }, true
	case "?":
		a.mode = uitypes.ModeHelp
		return nil, true
	case "esc":
		// Pop screen (back) if there's something to pop, else clear filter.
		if a.stack.Len() > 1 {
			return uitypes.Pop(), true
		}
		return func() tea.Msg { return uitypes.FilterClearedMsg{} }, true
	case "r":
		// Only refresh if the active screen advertises `r` — JSON
		// viewer (and any future static content) doesn't.
		if !a.activeScreenBindsKey("r") {
			return nil, true
		}
		return func() tea.Msg { return uitypes.RequestRefreshMsg{} }, true
	case "1":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusPending})), true
	case "2":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusRunning})), true
	case "3":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusFinished})), true
	case "4":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusCanceled})), true
	case "5":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusFailed, api.StatusCascadeFailed, api.StatusTimeout})), true
	case "0":
		return uitypes.Replace(a.makeWorkflows(nil)), true
	case "s":
		return uitypes.Push(screens.NewStats(a.client, a.cfg.PollSlow)), true
	}
	return nil, false
}

func (a *App) globalBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/cancel")),
		key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "all workflows")),
		key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "pending")),
		key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "running")),
		key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "finished")),
		key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "canceled")),
		key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "failed")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stats")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// --- delegation + lifecycle ---

func (a *App) delegate(msg tea.Msg) tea.Cmd {
	top, ok := a.stack.Top()
	if !ok {
		return nil
	}
	updated, cmd := top.Update(msg)
	if s, ok := updated.(uitypes.Screen); ok {
		a.stack.Replace(s)
	}
	return cmd
}

func (a *App) propagateSize() {
	if a.w == 0 || a.h == 0 {
		return
	}
	if top, ok := a.stack.Top(); ok {
		top.SetSize(a.w, a.bodyHeight())
	}
}

func (a *App) activeInitCmd() tea.Cmd {
	if top, ok := a.stack.Top(); ok {
		return top.Init()
	}
	return nil
}

func (a *App) activeTickCmd() tea.Cmd {
	if top, ok := a.stack.Top(); ok {
		return a.tickFor(top)
	}
	return nil
}

func (a *App) tickFor(s uitypes.Screen) tea.Cmd {
	id := s.ID()
	return poll.Tick(s.PollInterval(), func(t time.Time) tea.Msg {
		return uitypes.PollTickMsg{ScreenID: id, Time: t}
	})
}

func (a *App) fetchCountsCmd() tea.Cmd {
	a.countsSeq++
	seq := a.countsSeq
	client := a.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, err := client.GetCounts(ctx, 0)
		return uitypes.CountsLoadedMsg{Seq: seq, Counts: c, Err: err}
	}
}

func (a *App) countsTickCmd() tea.Cmd {
	return tea.Tick(a.cfg.PollMedium, func(t time.Time) tea.Msg {
		return uitypes.CountsTickMsg{Time: t}
	})
}

func (a *App) scheduleToastClear(id int, ttl time.Duration) tea.Cmd {
	if ttl <= 0 {
		return nil
	}
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return uitypes.ClearToastMsg{ID: id}
	})
}

// truncErr renders an error message short enough to fit comfortably in
// the footer toast strip on most terminals. Sanitizes first so any
// control bytes from underlying error strings (e.g., wrapped errors
// containing API response bodies) can't inject ANSI sequences, and
// truncates by rune count to avoid splitting multibyte characters.
func truncErr(err error) string {
	return format.Trunc(format.Sanitize(err.Error()), 80)
}

// Package screens contains the App's individual content views.
package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/format"
	"github.com/garden-of-delete/orchard-tui/internal/ui/poll"
	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

// Workflows lists workflows, optionally filtered to a status set.
type Workflows struct {
	id       string
	client   *api.Client
	statuses []api.Status
	pollGap  time.Duration

	rows    []api.Workflow // last-fetched (sorted)
	visible []api.Workflow // after filter
	filter  string

	tbl     table.Model
	spin    spinner.Model
	loading bool
	err     error
	w, h    int
}

// workflowsLoadedMsg carries the result of a fetch.
type workflowsLoadedMsg struct {
	id   string
	rows []api.Workflow
	err  error
}

// NewWorkflows builds a Workflows screen filtered to the given statuses.
// Pass nil for statuses to show everything.
func NewWorkflows(client *api.Client, statuses []api.Status, fastPoll, slowPoll time.Duration) *Workflows {
	w := &Workflows{
		id:       fmt.Sprintf("workflows-%d", time.Now().UnixNano()),
		client:   client,
		statuses: statuses,
		pollGap:  pickPoll(statuses, fastPoll, slowPoll),
	}

	cols := []table.Column{
		{Title: "ID", Width: 32},
		{Title: "NAME", Width: 24},
		{Title: "STATUS", Width: 12},
		{Title: "CREATED", Width: 12},
		{Title: "ACTIVATED", Width: 12},
		{Title: "TERMINATED", Width: 12},
	}
	tbl := table.New(table.WithColumns(cols), table.WithFocused(true), table.WithHeight(10))
	tbl.SetStyles(workflowsTableStyles())
	w.tbl = tbl

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	w.spin = sp

	return w
}

func pickPoll(statuses []api.Status, fast, slow time.Duration) time.Duration {
	for _, s := range statuses {
		if s == api.StatusRunning || s == api.StatusPending {
			return fast
		}
	}
	if len(statuses) == 0 {
		// "All" view: balance freshness against load.
		return slow
	}
	return slow
}

// --- Screen interface ---

func (w *Workflows) ID() string                  { return w.id }
func (w *Workflows) Title() string               { return "workflows" + statusSuffix(w.statuses) }
func (w *Workflows) PollInterval() time.Duration { return w.pollGap }

func (w *Workflows) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
	}
}

func (w *Workflows) SetSize(width, height int) {
	w.w, w.h = width, height
	w.layout()
}

func (w *Workflows) Init() tea.Cmd {
	w.loading = true
	return tea.Batch(w.spin.Tick, w.fetchCmd())
}

func (w *Workflows) Refresh() tea.Cmd { return w.fetchCmd() }

func (w *Workflows) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case workflowsLoadedMsg:
		if m.id != w.id {
			return w, nil
		}
		w.loading = false
		if m.err != nil {
			w.err = m.err
		} else {
			w.err = nil
			w.rows = m.rows
			sort.SliceStable(w.rows, func(i, j int) bool {
				return w.rows[i].CreatedAt.Time.After(w.rows[j].CreatedAt.Time)
			})
			w.applyFilter()
		}
		return w, nil

	case uitypes.PollTickMsg:
		if m.ScreenID != w.id {
			return w, nil
		}
		return w, tea.Batch(w.fetchCmd(), w.tickCmd())

	case uitypes.RequestRefreshMsg:
		w.loading = true
		return w, tea.Batch(w.spin.Tick, w.fetchCmd())

	case uitypes.FilterChangedMsg:
		w.filter = m.Query
		w.applyFilter()
		return w, nil

	case uitypes.FilterCommittedMsg:
		w.filter = m.Query
		w.applyFilter()
		return w, nil

	case uitypes.FilterClearedMsg:
		w.filter = ""
		w.applyFilter()
		return w, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		w.spin, cmd = w.spin.Update(m)
		return w, cmd

	case tea.KeyMsg:
		switch m.String() {
		case "enter":
			if wf, ok := w.selectedWorkflow(); ok {
				return w, uitypes.Push(NewWorkflowDetail(w.client, wf.ID, w.pollGap, w.pollGap*5))
			}
			return w, nil
		}
	}

	var cmd tea.Cmd
	w.tbl, cmd = w.tbl.Update(msg)
	return w, cmd
}

func (w *Workflows) View() string {
	if w.w == 0 {
		return ""
	}
	var head string
	switch {
	case w.err != nil:
		head = lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + w.err.Error())
	case w.loading && len(w.rows) == 0:
		head = w.spin.View() + " loading workflows…"
	case len(w.visible) == 0 && w.filter != "":
		head = styles.Faint.Render(fmt.Sprintf("No matches for /%s", w.filter))
	case len(w.visible) == 0:
		head = styles.Faint.Render("No workflows yet — start one with the orchard API.")
	default:
		head = styles.Faint.Render(fmt.Sprintf("%d workflow%s%s", len(w.visible), plural(len(w.visible)), filterNote(w.filter)))
	}
	return head + "\n" + w.tbl.View()
}

// --- helpers ---

func (w *Workflows) tickCmd() tea.Cmd {
	id := w.id
	return poll.Tick(w.pollGap, func(t time.Time) tea.Msg {
		return uitypes.PollTickMsg{ScreenID: id, Time: t}
	})
}

func (w *Workflows) fetchCmd() tea.Cmd {
	id := w.id
	client := w.client
	statuses := w.statuses
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		rows, err := client.ListWorkflows(ctx, api.ListWorkflowsOpts{
			Statuses: statuses,
			OrderBy:  api.OrderByCreatedAt,
			Order:    api.OrderDesc,
		})
		return workflowsLoadedMsg{id: id, rows: rows, err: err}
	}
}

func (w *Workflows) selectedWorkflow() (api.Workflow, bool) {
	idx := w.tbl.Cursor()
	if idx < 0 || idx >= len(w.visible) {
		return api.Workflow{}, false
	}
	return w.visible[idx], true
}

func (w *Workflows) applyFilter() {
	if w.filter == "" {
		w.visible = w.rows
	} else {
		needle := strings.ToLower(w.filter)
		w.visible = w.visible[:0]
		for _, r := range w.rows {
			if strings.Contains(strings.ToLower(r.ID), needle) ||
				strings.Contains(strings.ToLower(r.Name), needle) ||
				strings.Contains(strings.ToLower(string(r.Status)), needle) {
				w.visible = append(w.visible, r)
			}
		}
	}
	w.refreshTable()
}

func (w *Workflows) refreshTable() {
	now := time.Now().UTC()
	rows := make([]table.Row, 0, len(w.visible))
	for _, wf := range w.visible {
		rows = append(rows, table.Row{
			format.Trunc(wf.ID, 32),
			format.Trunc(wf.Name, 24),
			styles.StatusPill(wf.Status),
			format.RelTime(wf.CreatedAt.Time, now),
			activatedRel(wf, now),
			terminatedRel(wf, now),
		})
	}
	w.tbl.SetRows(rows)
	if w.tbl.Cursor() >= len(rows) {
		if len(rows) > 0 {
			w.tbl.SetCursor(len(rows) - 1)
		}
	}
}

func activatedRel(wf api.Workflow, now time.Time) string {
	if wf.ActivatedAt == nil {
		return "—"
	}
	return format.RelTime(wf.ActivatedAt.Time, now)
}

func terminatedRel(wf api.Workflow, now time.Time) string {
	if wf.TerminatedAt == nil {
		return "—"
	}
	return format.RelTime(wf.TerminatedAt.Time, now)
}

func (w *Workflows) layout() {
	if w.h <= 0 || w.w <= 0 {
		return
	}
	// Reserve 1 row for the header line above the table.
	tableHeight := w.h - 1
	if tableHeight < 4 {
		tableHeight = 4
	}
	w.tbl.SetHeight(tableHeight)
	w.tbl.SetWidth(w.w)
}

func statusSuffix(s []api.Status) string {
	if len(s) == 0 {
		return "(all)"
	}
	parts := make([]string, len(s))
	for i, st := range s {
		parts[i] = string(st)
	}
	return "(" + strings.Join(parts, ",") + ")"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func filterNote(f string) string {
	if f == "" {
		return ""
	}
	return " · /" + f
}

func workflowsTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.Border).
		BorderBottom(true).
		Bold(true).
		Foreground(styles.Highlight)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(styles.Primary).
		Bold(false)
	return s
}

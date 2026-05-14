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

	// Poll cadences carried forward to pushed child screens. pollGap is
	// this screen's own auto-refresh interval, derived from statuses via
	// pickPoll.
	pollFast   time.Duration
	pollMedium time.Duration
	pollSlow   time.Duration
	pollGap    time.Duration

	rows      []api.Workflow // last-fetched (sorted)
	visible   []api.Workflow // after filter
	filter    string
	preFilter string   // snapshot taken on FilterEnterMsg; restored on FilterCancelMsg
	fetchSeq  int      // monotonic per-fetch seq; loaded msgs older than this are dropped
	sortMode  sortMode // default: byStatus (red statuses at top)

	tbl     table.Model
	spin    spinner.Model
	loading bool
	err     error
	w, h    int
}

type sortMode int

const (
	sortByStatus sortMode = iota
	sortByCreated
)

// workflowsPerPage is how many rows the screen requests in a single
// fetch. Orchard's server-side default is 50; we ask for substantially
// more so most installations never hit the cap. When a fetch returns
// exactly this many rows, the screen shows a "may be truncated" hint.
const workflowsPerPage = 200

// workflowsLoadedMsg carries the result of a fetch.
type workflowsLoadedMsg struct {
	id   string
	seq  int // matches the screen's fetchSeq at issue time
	rows []api.Workflow
	err  error
}

// NewWorkflows builds a Workflows screen filtered to the given statuses.
// Pass nil for statuses to show everything.
//
// Cadence picked by pickPoll:
//   - filter contains running or pending → fast
//   - empty filter ("all") → medium
//   - filter is purely terminal statuses → slow
func NewWorkflows(client *api.Client, statuses []api.Status, fastPoll, mediumPoll, slowPoll time.Duration) *Workflows {
	w := &Workflows{
		id:         fmt.Sprintf("workflows-%d", time.Now().UnixNano()),
		client:     client,
		statuses:   statuses,
		pollFast:   fastPoll,
		pollMedium: mediumPoll,
		pollSlow:   slowPoll,
		pollGap:    pickPoll(statuses, fastPoll, mediumPoll, slowPoll),
	}

	tbl := table.New(table.WithColumns(w.computeColumns(0)), table.WithFocused(true), table.WithHeight(10))
	tbl.SetStyles(workflowsTableStyles())
	w.tbl = tbl

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	w.spin = sp

	return w
}

func pickPoll(statuses []api.Status, fast, medium, slow time.Duration) time.Duration {
	for _, s := range statuses {
		if s == api.StatusRunning || s == api.StatusPending {
			return fast
		}
	}
	if len(statuses) == 0 {
		// "All" view mixes active and terminal entities — keep it fresh
		// enough that newly-created workflows show up promptly.
		return medium
	}
	// Filter is purely terminal (finished/canceled/failed/...).
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
		key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort")),
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
		if m.id != w.id || m.seq < w.fetchSeq {
			return w, nil // stale (older fetch overtaken by a newer one)
		}
		w.loading = false
		if m.err != nil {
			w.err = m.err
		} else {
			w.err = nil
			w.rows = m.rows
			w.sortRows()
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

	case uitypes.FilterEnterMsg:
		w.preFilter = w.filter
		return w, nil

	case uitypes.FilterChangedMsg:
		w.filter = m.Query
		w.applyFilter()
		return w, nil

	case uitypes.FilterCommittedMsg:
		w.filter = m.Query
		w.preFilter = ""
		w.applyFilter()
		return w, nil

	case uitypes.FilterCancelMsg:
		w.filter = w.preFilter
		w.preFilter = ""
		w.applyFilter()
		return w, nil

	case uitypes.FilterClearedMsg:
		w.filter = ""
		w.preFilter = ""
		w.applyFilter()
		return w, nil

	case spinner.TickMsg:
		if !w.loading {
			return w, nil // don't keep the cmd tree alive when nothing's spinning
		}
		var cmd tea.Cmd
		w.spin, cmd = w.spin.Update(m)
		return w, cmd

	case tea.KeyMsg:
		switch m.String() {
		case "enter":
			if wf, ok := w.selectedWorkflow(); ok {
				return w, uitypes.Push(NewWorkflowDetail(w.client, wf.ID, w.pollFast))
			}
			return w, nil
		case "S":
			if w.sortMode == sortByStatus {
				w.sortMode = sortByCreated
			} else {
				w.sortMode = sortByStatus
			}
			w.sortRows()
			w.applyFilter()
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
		head = lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + format.Sanitize(w.err.Error()))
	case w.loading && len(w.rows) == 0:
		head = w.spin.View() + " loading workflows…"
	case len(w.visible) == 0 && w.filter != "":
		head = styles.Faint.Render(fmt.Sprintf("No matches for /%s", format.Sanitize(w.filter)))
	case len(w.visible) == 0:
		head = styles.Faint.Render("No workflows yet — start one with the orchard API.")
	default:
		summary := fmt.Sprintf("%d workflow%s%s · sort:%s (S)",
			len(w.visible), plural(len(w.visible)), filterNote(w.filter), sortLabel(w.sortMode))
		if len(w.rows) >= workflowsPerPage {
			summary += fmt.Sprintf("  (showing first %d — narrow with /, a status filter, or :wf <id>)", workflowsPerPage)
		}
		head = styles.Faint.Render(summary)
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
	w.fetchSeq++
	seq := w.fetchSeq
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
			PerPage:  workflowsPerPage,
		})
		return workflowsLoadedMsg{id: id, seq: seq, rows: rows, err: err}
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
		w.refreshTable()
		return
	}
	// Allocate a fresh slice rather than reusing w.visible's backing
	// array. When the previous applyFilter call took the empty-filter
	// branch, w.visible aliased w.rows; reslicing to [:0] and then
	// appending would silently corrupt w.rows in place.
	needle := strings.ToLower(w.filter)
	filtered := make([]api.Workflow, 0, len(w.rows))
	for _, r := range w.rows {
		if strings.Contains(strings.ToLower(r.ID), needle) ||
			strings.Contains(strings.ToLower(r.Name), needle) ||
			strings.Contains(strings.ToLower(string(r.Status)), needle) {
			filtered = append(filtered, r)
		}
	}
	w.visible = filtered
	w.refreshTable()
}

func (w *Workflows) refreshTable() {
	now := time.Now().UTC()
	cols := w.tbl.Columns()
	idW, nameW := cols[0].Width, cols[1].Width
	rows := make([]table.Row, 0, len(w.visible))
	for _, wf := range w.visible {
		rows = append(rows, table.Row{
			format.Trunc(format.Sanitize(wf.ID), idW),
			format.Trunc(format.Sanitize(wf.Name), nameW),
			styles.StatusPill(wf.Status),
			format.RelTime(wf.CreatedAt.Time, now),
			activatedRel(wf, now),
			terminatedRel(wf, now),
		})
	}
	w.tbl.SetRows(rows)
	clampCursor(&w.tbl, len(rows))
}

func (w *Workflows) sortRows() {
	switch w.sortMode {
	case sortByStatus:
		sort.SliceStable(w.rows, func(i, j int) bool {
			pi, pj := statusPriority(w.rows[i].Status), statusPriority(w.rows[j].Status)
			if pi != pj {
				return pi < pj
			}
			return w.rows[i].CreatedAt.Time.After(w.rows[j].CreatedAt.Time)
		})
	case sortByCreated:
		sort.SliceStable(w.rows, func(i, j int) bool {
			return w.rows[i].CreatedAt.Time.After(w.rows[j].CreatedAt.Time)
		})
	}
}

func statusPriority(s api.Status) int {
	switch s {
	case api.StatusFailed, api.StatusCascadeFailed, api.StatusTimeout:
		return 0
	case api.StatusCanceled, api.StatusCanceling:
		return 1
	case api.StatusRunning, api.StatusActivating, api.StatusDeactivating, api.StatusShuttingDown:
		return 2
	case api.StatusPending:
		return 3
	case api.StatusFinished:
		return 4
	case api.StatusDeleted:
		return 5
	}
	return 6
}

func sortLabel(m sortMode) string {
	if m == sortByStatus {
		return "status"
	}
	return "created"
}

// cellPad is the per-side padding bubbles/table adds to every cell.
const cellPad = 2

// clampCursor fixes the -1 cursor that bubbles/table leaves after SetRows(nil).
func clampCursor(tbl *table.Model, n int) {
	if n == 0 {
		return
	}
	c := tbl.Cursor()
	if c < 0 {
		tbl.SetCursor(0)
	} else if c >= n {
		tbl.SetCursor(n - 1)
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
	w.tbl.SetColumns(w.computeColumns(w.w))
	w.refreshTable() // re-truncate cells against new ID/NAME widths
}

func (w *Workflows) computeColumns(width int) []table.Column {
	// maxID covers "wf-" + a 36-char UUID; anything wider just wastes space
	// that NAME can use instead.
	const (
		statusW = 14
		timeW   = 10
		minID   = 20
		maxID   = 40
		minName = 12
		nCols   = 6
	)
	if width <= 0 {
		width = 80
	}
	fixed := statusW + 3*timeW
	flex := width - fixed - 2 - cellPad*nCols
	if flex < minID+minName {
		flex = minID + minName
	}
	idW := maxID
	if flex-minName < idW {
		idW = flex - minName
	}
	if idW < minID {
		idW = minID
	}
	nameW := flex - idW
	if nameW < minName {
		nameW = minName
	}
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "NAME", Width: nameW},
		{Title: "STATUS", Width: statusW},
		{Title: "CREATED", Width: timeW},
		{Title: "ACTIVATED", Width: timeW},
		{Title: "TERMINATED", Width: timeW},
	}
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
	return " · /" + format.Sanitize(f)
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

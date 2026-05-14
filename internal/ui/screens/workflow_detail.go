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
	"github.com/garden-of-delete/orchard-tui/internal/ui/components"
	"github.com/garden-of-delete/orchard-tui/internal/ui/poll"
	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

// WorkflowTab selects which sub-table is shown.
type WorkflowTab int

const (
	TabActivities WorkflowTab = iota
	TabResources
)

// WorkflowDetail shows a workflow header and a toggle between activities
// and resources tables.
type WorkflowDetail struct {
	id         string
	client     *api.Client
	workflowID string
	pollFast   time.Duration

	tab      WorkflowTab
	sortMode sortMode

	workflow   *api.Workflow
	activities []api.Activity
	resources  []api.Resource

	tbl     table.Model
	spin    spinner.Model
	loading bool
	err     error
	// Separate per-tab seq counters so a fresh fetch on one tab doesn't
	// invalidate an in-flight fetch on the other when the user toggles
	// rapidly. Single shared counter would drop the older tab's response
	// even when the user has tabbed back to it.
	fetchSeqAct int
	fetchSeqRes int

	w, h int
}

type workflowActivitiesLoadedMsg struct {
	id       string
	seq      int
	response *api.ActivitiesResponse
	err      error
}

type workflowResourcesLoadedMsg struct {
	id       string
	seq      int
	response *api.ResourcesResponse
	err      error
}

// NewWorkflowDetail constructs a workflow detail screen. Only the fast
// poll interval is needed: the screen polls at `fast` while the workflow
// is non-terminal, and stops polling entirely once it terminates.
func NewWorkflowDetail(client *api.Client, workflowID string, fast time.Duration) *WorkflowDetail {
	d := &WorkflowDetail{
		id:         fmt.Sprintf("wfdetail-%d", time.Now().UnixNano()),
		client:     client,
		workflowID: workflowID,
		pollFast:   fast,
		tab:        TabActivities,
	}
	d.tbl = table.New(table.WithColumns(activitiesColumns(0)), table.WithFocused(true), table.WithHeight(10))
	d.tbl.SetStyles(workflowsTableStyles())

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	d.spin = sp
	return d
}

func (d *WorkflowDetail) ID() string { return d.id }
func (d *WorkflowDetail) Title() string {
	return "workflow " + format.Sanitize(d.workflowID)
}
func (d *WorkflowDetail) PollInterval() time.Duration { return d.pickPoll() }

// pickPoll returns the auto-refresh interval for this screen. When the
// workflow has reached a terminal state, polling stops entirely (returns
// zero) — the entity won't transition further on its own, so any
// continued fetches would be pure waste. Users can still press `r`.
func (d *WorkflowDetail) pickPoll() time.Duration {
	if d.workflow == nil || !d.workflow.Status.IsTerminal() {
		return d.pollFast
	}
	return 0
}

func (d *WorkflowDetail) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "activities/resources")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
		key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

func (d *WorkflowDetail) SetSize(width, height int) {
	d.w, d.h = width, height
	d.layout()
}

func (d *WorkflowDetail) Init() tea.Cmd {
	d.loading = true
	return tea.Batch(d.spin.Tick, d.fetchCurrent())
}

func (d *WorkflowDetail) Refresh() tea.Cmd { return d.fetchCurrent() }

func (d *WorkflowDetail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {

	case workflowActivitiesLoadedMsg:
		if m.id != d.id || m.seq < d.fetchSeqAct {
			return d, nil // stale
		}
		d.loading = false
		if m.err != nil {
			d.err = m.err
		} else {
			d.err = nil
			d.workflow = &m.response.Workflow
			d.activities = m.response.Activities
			if d.tab == TabActivities {
				d.refreshTable()
			}
		}
		return d, nil

	case workflowResourcesLoadedMsg:
		if m.id != d.id || m.seq < d.fetchSeqRes {
			return d, nil // stale
		}
		d.loading = false
		if m.err != nil {
			d.err = m.err
		} else {
			d.err = nil
			d.workflow = &m.response.Workflow
			d.resources = m.response.Resources
			if d.tab == TabResources {
				d.refreshTable()
			}
		}
		return d, nil

	case uitypes.PollTickMsg:
		if m.ScreenID != d.id {
			return d, nil
		}
		return d, tea.Batch(d.fetchCurrent(), d.tickCmd())

	case uitypes.RequestRefreshMsg:
		d.loading = true
		return d, tea.Batch(d.spin.Tick, d.fetchCurrent())

	case spinner.TickMsg:
		if !d.loading {
			return d, nil
		}
		var cmd tea.Cmd
		d.spin, cmd = d.spin.Update(m)
		return d, cmd

	case tea.KeyMsg:
		switch m.String() {
		case "tab":
			d.tbl.SetRows(nil) // avoid renderRow panic when column count drops
			if d.tab == TabActivities {
				d.tab = TabResources
				d.tbl.SetColumns(resourcesColumns(d.w))
			} else {
				d.tab = TabActivities
				d.tbl.SetColumns(activitiesColumns(d.w))
			}
			d.refreshTable()
			// If we have no cached data for the new tab yet, show the
			// spinner while the fetch is in flight rather than an empty
			// table with no feedback.
			fetch := d.fetchCurrent()
			noCache := (d.tab == TabActivities && len(d.activities) == 0) ||
				(d.tab == TabResources && len(d.resources) == 0)
			if noCache {
				d.loading = true
				return d, tea.Batch(d.spin.Tick, fetch)
			}
			return d, fetch
		case "enter":
			return d, d.openSelected()
		case "S":
			if d.sortMode == sortByStatus {
				d.sortMode = sortByCreated
			} else {
				d.sortMode = sortByStatus
			}
			d.refreshTable()
			return d, nil
		}
	}

	var cmd tea.Cmd
	d.tbl, cmd = d.tbl.Update(msg)
	return d, cmd
}

func (d *WorkflowDetail) View() string {
	if d.w == 0 {
		return ""
	}

	header := d.headerCard()
	tabs := d.tabsLine()

	var body string
	switch {
	case d.err != nil:
		body = lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + format.Sanitize(d.err.Error()))
	case d.loading && d.workflow == nil:
		body = d.spin.View() + " loading…"
	default:
		body = d.tbl.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, "", tabs, body)
}

// --- helpers ---

func (d *WorkflowDetail) headerCard() string {
	now := time.Now().UTC()
	if d.workflow == nil {
		return components.Card{
			Title: "Workflow " + format.Sanitize(d.workflowID),
			Lines: []components.CardLine{{Label: "status", Value: styles.Faint.Render("loading…")}},
			Width: d.w,
		}.View()
	}
	wf := d.workflow
	statusLine := styles.StatusPill(wf.Status)
	if !wf.Status.IsTerminal() && wf.ActivatedAt != nil {
		statusLine += "  " + styles.Faint.Render("("+format.Between(wf.ActivatedAt.Time, time.Time{}, now)+")")
	} else if wf.Status.IsTerminal() && wf.ActivatedAt != nil && wf.TerminatedAt != nil {
		statusLine += "  " + styles.Faint.Render("("+format.Between(wf.ActivatedAt.Time, wf.TerminatedAt.Time, now)+")")
	}
	return components.Card{
		Title: format.Sanitize(wf.Name),
		Lines: []components.CardLine{
			{Label: "id", Value: format.Sanitize(wf.ID)},
			{Label: "status", Value: statusLine},
			{Label: "created", Value: wf.CreatedAt.Time.UTC().Format(time.RFC3339)},
			{Label: "activated", Value: optTime(wf.ActivatedAt)},
			{Label: "terminated", Value: optTime(wf.TerminatedAt)},
		},
		Width: d.w,
	}.View()
}

func (d *WorkflowDetail) tabsLine() string {
	act := "Activities"
	res := "Resources"
	on := lipgloss.NewStyle().Bold(true).Foreground(styles.Primary)
	off := styles.Faint
	trailing := styles.Faint.Render("   tab to switch · sort:" + sortLabel(d.sortMode))

	if d.tab == TabActivities {
		return on.Render("["+act+"]") + "  " + off.Render(res) + trailing
	}
	return off.Render(act) + "  " + on.Render("["+res+"]") + trailing
}

func (d *WorkflowDetail) tickCmd() tea.Cmd {
	id := d.id
	return poll.Tick(d.pickPoll(), func(t time.Time) tea.Msg {
		return uitypes.PollTickMsg{ScreenID: id, Time: t}
	})
}

func (d *WorkflowDetail) fetchCurrent() tea.Cmd {
	if d.tab == TabActivities {
		return d.fetchActivities()
	}
	return d.fetchResources()
}

func (d *WorkflowDetail) fetchActivities() tea.Cmd {
	d.fetchSeqAct++
	seq := d.fetchSeqAct
	id := d.id
	client := d.client
	wfID := d.workflowID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		resp, err := client.GetActivities(ctx, wfID)
		return workflowActivitiesLoadedMsg{id: id, seq: seq, response: resp, err: err}
	}
}

func (d *WorkflowDetail) fetchResources() tea.Cmd {
	d.fetchSeqRes++
	seq := d.fetchSeqRes
	id := d.id
	client := d.client
	wfID := d.workflowID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		resp, err := client.GetResources(ctx, wfID)
		return workflowResourcesLoadedMsg{id: id, seq: seq, response: resp, err: err}
	}
}

func (d *WorkflowDetail) openSelected() tea.Cmd {
	idx := d.tbl.Cursor()
	if d.tab == TabActivities {
		if idx < 0 || idx >= len(d.activities) {
			return nil
		}
		a := d.activities[idx]
		return uitypes.Push(NewActivityDetail(d.client, a.WorkflowID, a.ActivityID, d.pollFast))
	}
	if idx < 0 || idx >= len(d.resources) {
		return nil
	}
	r := d.resources[idx]
	return uitypes.Push(NewResourceDetail(d.client, r.WorkflowID, r.ResourceID, d.pollFast))
}

func (d *WorkflowDetail) refreshTable() {
	now := time.Now().UTC()
	cols := d.tbl.Columns()
	if d.tab == TabActivities {
		sortActivities(d.activities, d.sortMode)
		nameW, typeW, resW := cols[1].Width, cols[2].Width, cols[4].Width
		rows := make([]table.Row, 0, len(d.activities))
		for _, a := range d.activities {
			rows = append(rows, table.Row{
				format.Sanitize(a.ActivityID),
				format.Trunc(format.Sanitize(a.Name), nameW),
				format.Trunc(format.Sanitize(shortType(a.ActivityType)), typeW),
				string(a.Status),
				format.Trunc(format.Sanitize(a.ResourceID), resW),
				format.RelTime(a.CreatedAt.Time, now),
				optRel(a.ActivatedAt, now),
				optRel(a.TerminatedAt, now),
			})
		}
		d.tbl.SetRows(rows)
	} else {
		sortResources(d.resources, d.sortMode)
		nameW, typeW := cols[1].Width, cols[2].Width
		rows := make([]table.Row, 0, len(d.resources))
		for _, r := range d.resources {
			rows = append(rows, table.Row{
				format.Sanitize(r.ResourceID),
				format.Trunc(format.Sanitize(r.Name), nameW),
				format.Trunc(format.Sanitize(shortType(r.ResourceType)), typeW),
				string(r.Status),
				format.RelTime(r.CreatedAt.Time, now),
				optRel(r.ActivatedAt, now),
				optRel(r.TerminatedAt, now),
			})
		}
		d.tbl.SetRows(rows)
	}
	clampCursor(&d.tbl, len(d.tbl.Rows()))
}

func sortActivities(xs []api.Activity, mode sortMode) {
	sort.SliceStable(xs, func(i, j int) bool {
		if mode == sortByStatus {
			pi, pj := statusPriority(xs[i].Status), statusPriority(xs[j].Status)
			if pi != pj {
				return pi < pj
			}
		}
		return xs[i].CreatedAt.Time.After(xs[j].CreatedAt.Time)
	})
}

func sortResources(xs []api.Resource, mode sortMode) {
	sort.SliceStable(xs, func(i, j int) bool {
		if mode == sortByStatus {
			pi, pj := statusPriority(xs[i].Status), statusPriority(xs[j].Status)
			if pi != pj {
				return pi < pj
			}
		}
		return xs[i].CreatedAt.Time.After(xs[j].CreatedAt.Time)
	})
}

func (d *WorkflowDetail) layout() {
	if d.h <= 0 || d.w <= 0 {
		return
	}
	cardH := 8 // bordered card with 5 lines + title + padding
	tabsH := 2
	tableH := d.h - cardH - tabsH
	if tableH < 4 {
		tableH = 4
	}
	d.tbl.SetHeight(tableH)
	d.tbl.SetWidth(d.w)
	if d.tab == TabActivities {
		d.tbl.SetColumns(activitiesColumns(d.w))
	} else {
		d.tbl.SetColumns(resourcesColumns(d.w))
	}
	d.refreshTable()
}

func activitiesColumns(width int) []table.Column {
	const (
		idW, statusW, timeW = 6, 14, 10
		minRES, maxRES      = 6, 40
		minName, minType    = 14, 12
		maxType             = 25
		nCols               = 8
	)
	if width <= 0 {
		width = 80
	}
	fixed := idW + statusW + 3*timeW
	flex := width - fixed - 2 - cellPad*nCols
	floor := minRES + minName + minType
	if flex < floor {
		flex = floor
	}
	resW := maxRES
	if flex-minName-minType < resW {
		resW = flex - minName - minType
	}
	if resW < minRES {
		resW = minRES
	}
	typeW := maxType
	if flex-resW-minName < typeW {
		typeW = flex - resW - minName
	}
	if typeW < minType {
		typeW = minType
	}
	nameW := flex - resW - typeW
	if nameW < minName {
		nameW = minName
	}
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "NAME", Width: nameW},
		{Title: "TYPE", Width: typeW},
		{Title: "STATUS", Width: statusW},
		{Title: "RES", Width: resW},
		{Title: "CREATED", Width: timeW},
		{Title: "ACTIVATED", Width: timeW},
		{Title: "TERMINATED", Width: timeW},
	}
}

func resourcesColumns(width int) []table.Column {
	const (
		idW, statusW, timeW = 6, 14, 10
		minName, minType    = 14, 14
		nCols               = 7
	)
	if width <= 0 {
		width = 80
	}
	fixed := idW + statusW + 3*timeW
	flex := width - fixed - 2 - cellPad*nCols
	if flex < minName+minType {
		flex = minName + minType
	}
	nameW := flex * 50 / 100
	if nameW < minName {
		nameW = minName
	}
	typeW := flex - nameW
	if typeW < minType {
		typeW = minType
	}
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "NAME", Width: nameW},
		{Title: "TYPE", Width: typeW},
		{Title: "STATUS", Width: statusW},
		{Title: "CREATED", Width: timeW},
		{Title: "ACTIVATED", Width: timeW},
		{Title: "TERMINATED", Width: timeW},
	}
}

func shortType(t string) string {
	// "aws.activity.EmrActivity" → "EmrActivity"
	idx := strings.LastIndexByte(t, '.')
	if idx >= 0 && idx+1 < len(t) {
		return t[idx+1:]
	}
	return t
}

func optTime(t *api.OrchardTime) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return t.Time.UTC().Format(time.RFC3339)
}

func optRel(t *api.OrchardTime, now time.Time) string {
	if t == nil {
		return "—"
	}
	return format.RelTime(t.Time, now)
}

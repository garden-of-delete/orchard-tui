package screens

import (
	"context"
	"fmt"
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
	pollSlow   time.Duration

	tab WorkflowTab

	workflow   *api.Workflow
	activities []api.Activity
	resources  []api.Resource

	tbl     table.Model
	spin    spinner.Model
	loading bool
	err     error

	w, h int
}

type workflowActivitiesLoadedMsg struct {
	id       string
	response *api.ActivitiesResponse
	err      error
}

type workflowResourcesLoadedMsg struct {
	id       string
	response *api.ResourcesResponse
	err      error
}

func NewWorkflowDetail(client *api.Client, workflowID string, fast, slow time.Duration) *WorkflowDetail {
	d := &WorkflowDetail{
		id:         fmt.Sprintf("wfdetail-%d", time.Now().UnixNano()),
		client:     client,
		workflowID: workflowID,
		pollFast:   fast,
		pollSlow:   slow,
		tab:        TabActivities,
	}
	d.tbl = table.New(table.WithColumns(activitiesColumns()), table.WithFocused(true), table.WithHeight(10))
	d.tbl.SetStyles(workflowsTableStyles())

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	d.spin = sp
	return d
}

func (d *WorkflowDetail) ID() string                  { return d.id }
func (d *WorkflowDetail) Title() string               { return "workflow " + d.workflowID }
func (d *WorkflowDetail) PollInterval() time.Duration { return d.pickPoll() }

func (d *WorkflowDetail) pickPoll() time.Duration {
	if d.workflow == nil || !d.workflow.Status.IsTerminal() {
		return d.pollFast
	}
	return d.pollSlow
}

func (d *WorkflowDetail) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "activities/resources")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
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
		if m.id != d.id {
			return d, nil
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
		if m.id != d.id {
			return d, nil
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
		var cmd tea.Cmd
		d.spin, cmd = d.spin.Update(m)
		return d, cmd

	case tea.KeyMsg:
		switch m.String() {
		case "tab":
			if d.tab == TabActivities {
				d.tab = TabResources
				d.tbl.SetColumns(resourcesColumns())
			} else {
				d.tab = TabActivities
				d.tbl.SetColumns(activitiesColumns())
			}
			d.refreshTable()
			return d, d.fetchCurrent()
		case "enter":
			return d, d.openSelected()
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
		body = lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + d.err.Error())
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
			Title: "Workflow " + d.workflowID,
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
		Title: wf.Name,
		Lines: []components.CardLine{
			{Label: "id", Value: wf.ID},
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

	if d.tab == TabActivities {
		return on.Render("["+act+"]") + "  " + off.Render(res) + styles.Faint.Render("   tab to switch")
	}
	return off.Render(act) + "  " + on.Render("["+res+"]") + styles.Faint.Render("   tab to switch")
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
	id := d.id
	client := d.client
	wfID := d.workflowID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		resp, err := client.GetActivities(ctx, wfID)
		return workflowActivitiesLoadedMsg{id: id, response: resp, err: err}
	}
}

func (d *WorkflowDetail) fetchResources() tea.Cmd {
	id := d.id
	client := d.client
	wfID := d.workflowID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		resp, err := client.GetResources(ctx, wfID)
		return workflowResourcesLoadedMsg{id: id, response: resp, err: err}
	}
}

func (d *WorkflowDetail) openSelected() tea.Cmd {
	idx := d.tbl.Cursor()
	if d.tab == TabActivities {
		if idx < 0 || idx >= len(d.activities) {
			return nil
		}
		a := d.activities[idx]
		return uitypes.Push(NewActivityDetail(d.client, a.WorkflowID, a.ActivityID, d.pollFast, d.pollSlow))
	}
	if idx < 0 || idx >= len(d.resources) {
		return nil
	}
	r := d.resources[idx]
	return uitypes.Push(NewResourceDetail(d.client, r.WorkflowID, r.ResourceID, d.pollFast, d.pollSlow))
}

func (d *WorkflowDetail) refreshTable() {
	now := time.Now().UTC()
	if d.tab == TabActivities {
		rows := make([]table.Row, 0, len(d.activities))
		for _, a := range d.activities {
			rows = append(rows, table.Row{
				a.ActivityID,
				format.Trunc(a.Name, 24),
				shortType(a.ActivityType),
				styles.StatusPill(a.Status),
				format.Trunc(a.ResourceID, 6),
				format.RelTime(a.CreatedAt.Time, now),
				optRel(a.ActivatedAt, now),
				optRel(a.TerminatedAt, now),
			})
		}
		d.tbl.SetRows(rows)
	} else {
		rows := make([]table.Row, 0, len(d.resources))
		for _, r := range d.resources {
			rows = append(rows, table.Row{
				r.ResourceID,
				format.Trunc(r.Name, 24),
				shortType(r.ResourceType),
				styles.StatusPill(r.Status),
				format.RelTime(r.CreatedAt.Time, now),
				optRel(r.ActivatedAt, now),
				optRel(r.TerminatedAt, now),
				fmt.Sprintf("%.0fh", r.TerminateAfter),
			})
		}
		d.tbl.SetRows(rows)
	}
	if d.tbl.Cursor() >= len(d.tbl.Rows()) {
		if len(d.tbl.Rows()) > 0 {
			d.tbl.SetCursor(len(d.tbl.Rows()) - 1)
		}
	}
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
}

func activitiesColumns() []table.Column {
	return []table.Column{
		{Title: "ID", Width: 6},
		{Title: "NAME", Width: 24},
		{Title: "TYPE", Width: 22},
		{Title: "STATUS", Width: 12},
		{Title: "RES", Width: 6},
		{Title: "CREATED", Width: 12},
		{Title: "ACTIVATED", Width: 12},
		{Title: "TERMINATED", Width: 12},
	}
}

func resourcesColumns() []table.Column {
	return []table.Column{
		{Title: "ID", Width: 6},
		{Title: "NAME", Width: 24},
		{Title: "TYPE", Width: 28},
		{Title: "STATUS", Width: 12},
		{Title: "CREATED", Width: 12},
		{Title: "ACTIVATED", Width: 12},
		{Title: "TERMINATED", Width: 12},
		{Title: "TTL", Width: 6},
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

package screens

import (
	"context"
	"fmt"
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

// ResourceDetail shows a resource header and its instances.
type ResourceDetail struct {
	id         string
	client     *api.Client
	workflowID string
	resourceID string
	pollFast   time.Duration

	resource  *api.Resource
	instances []api.ResourceInstance

	tbl      table.Model
	spin     spinner.Model
	loading  bool
	err      error
	fetchSeq int // monotonic per-fetch seq; older loaded msgs are dropped
	w, h     int
}

type resourceLoadedMsg struct {
	id       string
	seq      int
	response *api.ResourceInstancesResponse
	err      error
}

// NewResourceDetail constructs a resource detail screen. Only the fast
// poll interval is needed: polling stops once the resource terminates.
func NewResourceDetail(client *api.Client, workflowID, resourceID string, fast time.Duration) *ResourceDetail {
	d := &ResourceDetail{
		id:         fmt.Sprintf("rscdetail-%d", time.Now().UnixNano()),
		client:     client,
		workflowID: workflowID,
		resourceID: resourceID,
		pollFast:   fast,
	}
	d.tbl = table.New(table.WithColumns(instancesColumns()), table.WithFocused(true), table.WithHeight(10))
	d.tbl.SetStyles(workflowsTableStyles())

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	d.spin = sp
	return d
}

func (d *ResourceDetail) ID() string { return d.id }
func (d *ResourceDetail) Title() string {
	return "wf " + format.Sanitize(d.workflowID) + " · resource " + format.Sanitize(d.resourceID)
}
func (d *ResourceDetail) PollInterval() time.Duration { return d.pickPoll() }

// pickPoll returns the auto-refresh interval. Terminal resources stop
// auto-polling entirely; the user can still press `r` to refresh.
func (d *ResourceDetail) pickPoll() time.Duration {
	if d.resource == nil || !d.resource.Status.IsTerminal() {
		return d.pollFast
	}
	return 0
}

func (d *ResourceDetail) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view spec")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

func (d *ResourceDetail) SetSize(width, height int) {
	d.w, d.h = width, height
	d.layout()
}

func (d *ResourceDetail) Init() tea.Cmd {
	d.loading = true
	return tea.Batch(d.spin.Tick, d.fetchCmd())
}

func (d *ResourceDetail) Refresh() tea.Cmd { return d.fetchCmd() }

func (d *ResourceDetail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case resourceLoadedMsg:
		if m.id != d.id || m.seq < d.fetchSeq {
			return d, nil // stale
		}
		d.loading = false
		if m.err != nil {
			d.err = m.err
		} else {
			d.err = nil
			d.resource = &m.response.Resource
			d.instances = m.response.Instances
			d.refreshTable()
		}
		return d, nil

	case uitypes.PollTickMsg:
		if m.ScreenID != d.id {
			return d, nil
		}
		return d, tea.Batch(d.fetchCmd(), d.tickCmd())

	case uitypes.RequestRefreshMsg:
		d.loading = true
		return d, tea.Batch(d.spin.Tick, d.fetchCmd())

	case spinner.TickMsg:
		if !d.loading {
			return d, nil
		}
		var cmd tea.Cmd
		d.spin, cmd = d.spin.Update(m)
		return d, cmd

	case tea.KeyMsg:
		if m.String() == "enter" {
			return d, d.openSpec()
		}
	}

	var cmd tea.Cmd
	d.tbl, cmd = d.tbl.Update(msg)
	return d, cmd
}

func (d *ResourceDetail) View() string {
	if d.w == 0 {
		return ""
	}
	header := d.headerCard()

	var body string
	switch {
	case d.err != nil:
		body = lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + format.Sanitize(d.err.Error()))
	case d.loading && d.resource == nil:
		body = d.spin.View() + " loading…"
	default:
		body = styles.Faint.Render(fmt.Sprintf("%d instance%s", len(d.instances), plural(len(d.instances)))) + "\n" + d.tbl.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body)
}

func (d *ResourceDetail) headerCard() string {
	if d.resource == nil {
		return components.Card{
			Title: "Resource " + format.Sanitize(d.resourceID),
			Lines: []components.CardLine{{Label: "status", Value: styles.Faint.Render("loading…")}},
			Width: d.w,
		}.View()
	}
	r := d.resource
	return components.Card{
		Title: format.Sanitize(r.Name),
		Lines: []components.CardLine{
			{Label: "id", Value: format.Sanitize(r.ResourceID)},
			{Label: "type", Value: format.Sanitize(r.ResourceType)},
			{Label: "status", Value: styles.StatusPill(r.Status)},
			{Label: "max attempts", Value: fmt.Sprintf("%d", r.MaxAttempt)},
			{Label: "terminate after", Value: fmt.Sprintf("%.1fh", r.TerminateAfter)},
			{Label: "created", Value: r.CreatedAt.Time.UTC().Format(time.RFC3339)},
		},
		Width: d.w,
	}.View()
}

func (d *ResourceDetail) tickCmd() tea.Cmd {
	id := d.id
	return poll.Tick(d.pickPoll(), func(t time.Time) tea.Msg {
		return uitypes.PollTickMsg{ScreenID: id, Time: t}
	})
}

func (d *ResourceDetail) fetchCmd() tea.Cmd {
	d.fetchSeq++
	seq := d.fetchSeq
	id := d.id
	client := d.client
	wfID := d.workflowID
	rscID := d.resourceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		resp, err := client.GetResource(ctx, wfID, rscID)
		return resourceLoadedMsg{id: id, seq: seq, response: resp, err: err}
	}
}

func (d *ResourceDetail) openSpec() tea.Cmd {
	idx := d.tbl.Cursor()
	if idx < 0 || idx >= len(d.instances) {
		return nil
	}
	inst := d.instances[idx]
	return uitypes.Push(NewJSONView(
		fmt.Sprintf("instance %d spec — wf %s · resource %s",
			inst.InstanceAttempt,
			format.Sanitize(d.workflowID),
			format.Sanitize(d.resourceID)),
		inst.InstanceSpec,
		awsURLForResource(d.resource, inst),
	))
}

func (d *ResourceDetail) refreshTable() {
	now := time.Now().UTC()
	rows := make([]table.Row, 0, len(d.instances))
	for _, inst := range d.instances {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", inst.InstanceAttempt),
			styles.StatusPill(inst.Status),
			format.Trunc(format.Sanitize(format.FirstLine(inst.ErrorMessage)), 40),
			format.RelTime(inst.CreatedAt.Time, now),
			optRel(inst.ActivatedAt, now),
			optRel(inst.TerminatedAt, now),
		})
	}
	d.tbl.SetRows(rows)
	if d.tbl.Cursor() >= len(rows) {
		if len(rows) > 0 {
			d.tbl.SetCursor(len(rows) - 1)
		}
	}
}

func (d *ResourceDetail) layout() {
	if d.h <= 0 || d.w <= 0 {
		return
	}
	cardH := 9
	tableH := d.h - cardH - 2
	if tableH < 4 {
		tableH = 4
	}
	d.tbl.SetHeight(tableH)
	d.tbl.SetWidth(d.w)
}

func instancesColumns() []table.Column {
	return []table.Column{
		{Title: "#", Width: 4},
		{Title: "STATUS", Width: 12},
		{Title: "ERROR", Width: 40},
		{Title: "CREATED", Width: 12},
		{Title: "ACTIVATED", Width: 12},
		{Title: "TERMINATED", Width: 12},
	}
}

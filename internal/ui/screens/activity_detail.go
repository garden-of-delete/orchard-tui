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

// ActivityDetail shows an activity header and its attempts.
type ActivityDetail struct {
	id         string
	client     *api.Client
	workflowID string
	activityID string
	pollFast   time.Duration

	activity *api.Activity
	attempts []api.ActivityAttempt
	fetchSeq int // monotonic per-fetch seq; older loaded msgs are dropped

	tbl     table.Model
	spin    spinner.Model
	loading bool
	err     error
	w, h    int
}

type activityLoadedMsg struct {
	id       string
	seq      int
	response *api.ActivityAttemptsResponse
	err      error
}

// NewActivityDetail constructs an activity detail screen. Only the fast
// poll interval is needed: polling stops once the activity terminates.
func NewActivityDetail(client *api.Client, workflowID, activityID string, fast time.Duration) *ActivityDetail {
	d := &ActivityDetail{
		id:         fmt.Sprintf("actdetail-%d", time.Now().UnixNano()),
		client:     client,
		workflowID: workflowID,
		activityID: activityID,
		pollFast:   fast,
	}
	d.tbl = table.New(table.WithColumns(attemptsColumns(0)), table.WithFocused(true), table.WithHeight(10))
	d.tbl.SetStyles(workflowsTableStyles())

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	d.spin = sp
	return d
}

func (d *ActivityDetail) ID() string { return d.id }
func (d *ActivityDetail) Title() string {
	return "wf " + format.Sanitize(d.workflowID) + " · activity " + format.Sanitize(d.activityID)
}
func (d *ActivityDetail) PollInterval() time.Duration { return d.pickPoll() }

// pickPoll returns the auto-refresh interval. Terminal activities stop
// auto-polling entirely; the user can still press `r` to refresh.
func (d *ActivityDetail) pickPoll() time.Duration {
	if d.activity == nil || !d.activity.Status.IsTerminal() {
		return d.pollFast
	}
	return 0
}

func (d *ActivityDetail) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view spec")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

func (d *ActivityDetail) SetSize(width, height int) {
	d.w, d.h = width, height
	d.layout()
}

func (d *ActivityDetail) Init() tea.Cmd {
	d.loading = true
	return tea.Batch(d.spin.Tick, d.fetchCmd())
}

func (d *ActivityDetail) Refresh() tea.Cmd { return d.fetchCmd() }

func (d *ActivityDetail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case activityLoadedMsg:
		if m.id != d.id || m.seq < d.fetchSeq {
			return d, nil // stale
		}
		d.loading = false
		if m.err != nil {
			d.err = m.err
		} else {
			d.err = nil
			d.activity = &m.response.Activity
			d.attempts = m.response.Attempts
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

func (d *ActivityDetail) View() string {
	if d.w == 0 {
		return ""
	}

	header := d.headerCard()

	var body string
	switch {
	case d.err != nil:
		body = lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + format.Sanitize(d.err.Error()))
	case d.loading && d.activity == nil:
		body = d.spin.View() + " loading…"
	default:
		body = styles.Faint.Render(fmt.Sprintf("%d attempt%s", len(d.attempts), plural(len(d.attempts)))) + "\n" + d.tbl.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body)
}

func (d *ActivityDetail) headerCard() string {
	if d.activity == nil {
		return components.Card{
			Title: "Activity " + format.Sanitize(d.activityID),
			Lines: []components.CardLine{{Label: "status", Value: styles.Faint.Render("loading…")}},
			Width: d.w,
		}.View()
	}
	a := d.activity
	return components.Card{
		Title: format.Sanitize(a.Name),
		Lines: []components.CardLine{
			{Label: "id", Value: format.Sanitize(a.ActivityID)},
			{Label: "type", Value: format.Sanitize(a.ActivityType)},
			{Label: "resource", Value: format.Sanitize(a.ResourceID)},
			{Label: "status", Value: styles.StatusPill(a.Status)},
			{Label: "max attempts", Value: fmt.Sprintf("%d", a.MaxAttempt)},
			{Label: "created", Value: a.CreatedAt.Time.UTC().Format(time.RFC3339)},
		},
		Width: d.w,
	}.View()
}

func (d *ActivityDetail) tickCmd() tea.Cmd {
	id := d.id
	return poll.Tick(d.pickPoll(), func(t time.Time) tea.Msg {
		return uitypes.PollTickMsg{ScreenID: id, Time: t}
	})
}

func (d *ActivityDetail) fetchCmd() tea.Cmd {
	d.fetchSeq++
	seq := d.fetchSeq
	id := d.id
	client := d.client
	wfID := d.workflowID
	actID := d.activityID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		resp, err := client.GetActivity(ctx, wfID, actID)
		return activityLoadedMsg{id: id, seq: seq, response: resp, err: err}
	}
}

func (d *ActivityDetail) openSpec() tea.Cmd {
	idx := d.tbl.Cursor()
	if idx < 0 || idx >= len(d.attempts) {
		return nil
	}
	att := d.attempts[idx]
	return uitypes.Push(NewJSONView(
		fmt.Sprintf("attempt %d spec — wf %s · activity %s",
			att.Attempt,
			format.Sanitize(d.workflowID),
			format.Sanitize(d.activityID)),
		att.AttemptSpec,
		awsURLForActivity(d.activity, att),
	))
}

func (d *ActivityDetail) refreshTable() {
	now := time.Now().UTC()
	cols := d.tbl.Columns()
	errW := cols[2].Width
	rows := make([]table.Row, 0, len(d.attempts))
	for _, a := range d.attempts {
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", a.Attempt),
			styles.StatusPill(a.Status),
			format.Trunc(format.Sanitize(format.FirstLine(a.ErrorMessage)), errW),
			format.Trunc(format.Sanitize(a.ResourceID), 6),
			fmt.Sprintf("%d", a.ResourceInstanceAttempt),
			format.RelTime(a.CreatedAt.Time, now),
			optRel(a.ActivatedAt, now),
			optRel(a.TerminatedAt, now),
		})
	}
	d.tbl.SetRows(rows)
	clampCursor(&d.tbl, len(rows))
}

func (d *ActivityDetail) layout() {
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
	d.tbl.SetColumns(attemptsColumns(d.w))
	d.refreshTable()
}

func attemptsColumns(width int) []table.Column {
	const (
		numW, statusW, resW, instW, timeW = 4, 14, 6, 6, 10
		minErr                            = 30
		nCols                             = 8
	)
	if width <= 0 {
		width = 80
	}
	fixed := numW + statusW + resW + instW + 3*timeW
	errW := width - fixed - 2 - cellPad*nCols
	if errW < minErr {
		errW = minErr
	}
	return []table.Column{
		{Title: "#", Width: numW},
		{Title: "STATUS", Width: statusW},
		{Title: "ERROR", Width: errW},
		{Title: "RES", Width: resW},
		{Title: "INST#", Width: instW},
		{Title: "CREATED", Width: timeW},
		{Title: "ACTIVATED", Width: timeW},
		{Title: "TERMINATED", Width: timeW},
	}
}

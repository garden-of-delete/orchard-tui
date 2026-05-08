package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/format"
	"github.com/garden-of-delete/orchard-tui/internal/ui/poll"
	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

// Stats is the dashboard view: daily stacked bars + day×hour heatmap.
type Stats struct {
	id      string
	client  *api.Client
	pollGap time.Duration

	daily   []api.DailyCount
	pattern []api.PatternCount

	bar      barchart.Model
	spin     spinner.Model
	loading  bool
	err      error
	fetchSeq int // monotonic per-fetch seq; older loaded msgs are dropped
	w, h     int
}

type statsLoadedMsg struct {
	id      string
	seq     int
	daily   []api.DailyCount
	pattern []api.PatternCount
	err     error
}

func NewStats(client *api.Client, slow time.Duration) *Stats {
	s := &Stats{
		id:      fmt.Sprintf("stats-%d", time.Now().UnixNano()),
		client:  client,
		pollGap: slow,
		bar:     barchart.New(40, 10),
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	s.spin = sp
	return s
}

func (s *Stats) ID() string                  { return s.id }
func (s *Stats) Title() string               { return "stats" }
func (s *Stats) PollInterval() time.Duration { return s.pollGap }

func (s *Stats) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (s *Stats) SetSize(width, height int) {
	s.w, s.h = width, height
	s.layout()
}

func (s *Stats) Init() tea.Cmd {
	s.loading = true
	return tea.Batch(s.spin.Tick, s.fetchCmd())
}

func (s *Stats) Refresh() tea.Cmd { return s.fetchCmd() }

func (s *Stats) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case statsLoadedMsg:
		if m.id != s.id || m.seq < s.fetchSeq {
			return s, nil // stale
		}
		s.loading = false
		if m.err != nil {
			s.err = m.err
		} else {
			s.err = nil
			s.daily = m.daily
			s.pattern = m.pattern
			s.rebuildBar()
		}
		return s, nil
	case uitypes.PollTickMsg:
		if m.ScreenID != s.id {
			return s, nil
		}
		return s, tea.Batch(s.fetchCmd(), s.tickCmd())
	case uitypes.RequestRefreshMsg:
		s.loading = true
		return s, tea.Batch(s.spin.Tick, s.fetchCmd())
	case spinner.TickMsg:
		if !s.loading {
			return s, nil
		}
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(m)
		return s, cmd
	}
	return s, nil
}

func (s *Stats) View() string {
	if s.w == 0 {
		return ""
	}
	switch {
	case s.err != nil:
		return lipgloss.NewStyle().Foreground(styles.Error).Render("error: " + format.Sanitize(s.err.Error()))
	case s.loading && len(s.daily) == 0 && len(s.pattern) == 0:
		return s.spin.View() + " loading stats…"
	}

	dailyTitle := styles.Title.Render("Daily workflow counts")
	dailySub := styles.Faint.Render("status colors match the workflow list  •  most recent on the right")
	dailyChart := s.bar.View()

	patternTitle := styles.Title.Render("Activity by hour and day")
	patternSub := styles.Faint.Render("count of activations per hour-of-day × day-of-week, last 30 days")
	patternGrid := s.renderPattern()

	return lipgloss.JoinVertical(lipgloss.Left,
		dailyTitle, dailySub, dailyChart, "",
		patternTitle, patternSub, patternGrid,
	)
}

func (s *Stats) tickCmd() tea.Cmd {
	id := s.id
	return poll.Tick(s.pollGap, func(t time.Time) tea.Msg {
		return uitypes.PollTickMsg{ScreenID: id, Time: t}
	})
}

func (s *Stats) fetchCmd() tea.Cmd {
	s.fetchSeq++
	seq := s.fetchSeq
	id := s.id
	client := s.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		daily, derr := client.GetDaily(ctx, 30)
		if derr != nil {
			return statsLoadedMsg{id: id, seq: seq, err: derr}
		}
		pattern, perr := client.GetPattern(ctx, 30)
		if perr != nil {
			return statsLoadedMsg{id: id, seq: seq, err: perr}
		}
		return statsLoadedMsg{id: id, seq: seq, daily: daily, pattern: pattern}
	}
}

// --- daily bar chart ---

func (s *Stats) rebuildBar() {
	if len(s.daily) == 0 {
		return
	}

	// Recreate the bar model so PushAll doesn't accumulate bars on top
	// of stale data from prior fetches. ntcharts barchart's PushAll
	// appends; without this reset the chart would show duplicate dates
	// after each poll tick.
	chartW, chartH := s.chartDims()
	s.bar = barchart.New(chartW, chartH,
		barchart.WithBarGap(0),
		barchart.WithBarWidth(2),
	)

	// Group by date, summing per status into stacked BarValues.
	byDate := map[string]map[api.Status]int{}
	dates := []string{}
	for _, d := range s.daily {
		key := d.Date.Time.Format("01-02")
		if _, ok := byDate[key]; !ok {
			byDate[key] = map[api.Status]int{}
			dates = append(dates, key)
		}
		byDate[key][d.Status] += d.Count
	}
	sort.Strings(dates)

	bars := make([]barchart.BarData, 0, len(dates))
	statusOrder := []api.Status{api.StatusFinished, api.StatusRunning, api.StatusFailed, api.StatusCascadeFailed, api.StatusCanceled, api.StatusTimeout}
	for _, d := range dates {
		vals := []barchart.BarValue{}
		for _, st := range statusOrder {
			if c := byDate[d][st]; c > 0 {
				vals = append(vals, barchart.BarValue{
					Name:  string(st),
					Value: float64(c),
					Style: barColor(st),
				})
			}
		}
		bars = append(bars, barchart.BarData{Label: d, Values: vals})
	}

	s.bar.PushAll(bars)
	s.bar.Draw()
}

func barColor(st api.Status) lipgloss.Style {
	c := styles.StatusColor(st)
	return lipgloss.NewStyle().Foreground(c).Background(c)
}

// --- heatmap ---

func (s *Stats) renderPattern() string {
	// Build a 7×24 matrix.
	max := 0
	grid := [7][24]int{}
	for _, p := range s.pattern {
		if p.DayOfWeek < 0 || p.DayOfWeek > 6 || p.Hour < 0 || p.Hour > 23 {
			continue
		}
		grid[p.DayOfWeek][p.Hour] = p.Count
		if p.Count > max {
			max = p.Count
		}
	}

	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	var b strings.Builder
	// Hour header (every 3 hours).
	b.WriteString("    ")
	for h := 0; h < 24; h++ {
		if h%3 == 0 {
			b.WriteString(fmt.Sprintf("%-3d", h))
		}
	}
	b.WriteString("\n")

	for d := 0; d < 7; d++ {
		b.WriteString(styles.Faint.Render(dayNames[d] + " "))
		for h := 0; h < 24; h++ {
			b.WriteString(heatCell(grid[d][h], max))
		}
		b.WriteString("\n")
	}

	if max == 0 {
		b.WriteString(styles.Faint.Render("  (no activity in the past 30 days)\n"))
	}
	return b.String()
}

func heatCell(count, max int) string {
	if max <= 0 {
		return styles.Faint.Render(" ·")
	}
	intensity := float64(count) / float64(max)
	switch {
	case count == 0:
		return styles.Faint.Render(" ·")
	case intensity < 0.25:
		return lipgloss.NewStyle().Foreground(styles.Info).Render(" ░")
	case intensity < 0.5:
		return lipgloss.NewStyle().Foreground(styles.Info).Render(" ▒")
	case intensity < 0.75:
		return lipgloss.NewStyle().Foreground(styles.Info).Render(" ▓")
	default:
		return lipgloss.NewStyle().Foreground(styles.Info).Render(" █")
	}
}

func (s *Stats) layout() {
	if s.w <= 0 || s.h <= 0 {
		return
	}
	// rebuildBar recreates the bar model with the current dimensions, so
	// resize is just a matter of running it. (When called from layout
	// before any data has loaded, rebuildBar early-returns.)
	s.rebuildBar()
}

// chartDims returns the bar chart's width/height clamped to sane minima.
func (s *Stats) chartDims() (int, int) {
	chartH := s.h / 2
	if chartH < 8 {
		chartH = 8
	}
	chartW := s.w
	if chartW < 30 {
		chartW = 30
	}
	return chartW, chartH
}

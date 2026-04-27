package screens

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/garden-of-delete/orchard-tui/internal/ui/components"
	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

// JSONView is a full-screen JSON pager with chroma highlighting.
type JSONView struct {
	id       string
	title    string
	awsURL   string
	raw      json.RawMessage
	pretty   string
	rendered string

	vp        viewport.Model
	w, h      int

	mode   jsonMode
	search string
}

type jsonMode int

const (
	jsonModeNormal jsonMode = iota
	jsonModeSearch
)

// NewJSONView builds a viewer. awsURL may be empty — when set, it's printed
// as a top hint so the user can copy it.
func NewJSONView(title string, raw json.RawMessage, awsURL string) *JSONView {
	v := &JSONView{
		id:     fmt.Sprintf("json-%d", time.Now().UnixNano()),
		title:  title,
		raw:    raw,
		awsURL: awsURL,
	}
	v.pretty = components.PrettyJSON(raw)
	v.rendered = components.HighlightJSON(v.pretty)
	v.vp = viewport.New(0, 0)
	v.vp.SetContent(v.rendered)
	return v
}

func (v *JSONView) ID() string                  { return v.id }
func (v *JSONView) Title() string               { return v.title }
func (v *JSONView) PollInterval() time.Duration { return 0 }
func (v *JSONView) Refresh() tea.Cmd            { return nil }

func (v *JSONView) KeyMap() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (v *JSONView) SetSize(width, height int) {
	v.w, v.h = width, height
	headerLines := 2
	if v.awsURL != "" {
		headerLines = 4
	}
	bodyH := height - headerLines
	if bodyH < 4 {
		bodyH = 4
	}
	v.vp.Width = width
	v.vp.Height = bodyH
	v.applyContent()
}

func (v *JSONView) Init() tea.Cmd { return nil }

func (v *JSONView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {

	case uitypes.FilterChangedMsg:
		v.search = m.Query
		v.applyContent()
		return v, nil
	case uitypes.FilterCommittedMsg:
		v.search = m.Query
		v.applyContent()
		return v, nil
	case uitypes.FilterClearedMsg:
		v.search = ""
		v.applyContent()
		return v, nil

	case tea.KeyMsg:
		switch m.String() {
		case "y":
			if err := components.CopyToClipboard(v.pretty); err != nil {
				return v, uitypes.Toast(uitypes.ToastErr, "clipboard unavailable: "+err.Error())
			}
			return v, uitypes.Toast(uitypes.ToastOK, "copied "+humanBytes(len(v.pretty))+" to clipboard")
		}
	}

	var cmd tea.Cmd
	v.vp, cmd = v.vp.Update(msg)
	return v, cmd
}

func (v *JSONView) View() string {
	if v.w == 0 {
		return ""
	}
	header := styles.Title.Render(v.title)
	hint := styles.Faint.Render(fmt.Sprintf("%s · y·yank · /·search · esc·back", humanBytes(len(v.pretty))))

	parts := []string{header, hint}
	if v.awsURL != "" {
		parts = append(parts, styles.Faint.Render("aws: ")+v.awsURL)
		parts = append(parts, "")
	}
	parts = append(parts, v.vp.View())

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (v *JSONView) applyContent() {
	if v.search == "" {
		v.vp.SetContent(v.rendered)
		return
	}
	// Highlight matches against the *plain* pretty string (chroma's ANSI
	// escapes would wreck substring search), then re-render.
	v.vp.SetContent(components.HighlightMatches(v.pretty, v.search))
}

func humanBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
}


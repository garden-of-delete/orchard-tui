package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// CmdBar is the `:`-prefixed command palette input shown at the bottom.
type CmdBar struct {
	input  textinput.Model
	width  int
	prefix string
}

// NewCmdBar returns a configured cmd bar. prefix is shown before the input
// (e.g., ":" or "/").
func NewCmdBar(prefix string) CmdBar {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 256
	ti.Focus()
	return CmdBar{input: ti, prefix: prefix}
}

func (c *CmdBar) SetWidth(w int) {
	c.width = w
	c.input.Width = w - len(c.prefix) - 2
}

func (c *CmdBar) Reset() {
	c.input.Reset()
}

// Value is the text the user has typed (no prefix).
func (c CmdBar) Value() string { return c.input.Value() }

// SetValue replaces the typed text and moves the cursor to the end.
func (c *CmdBar) SetValue(s string) {
	c.input.SetValue(s)
	c.input.SetCursor(len(s))
}

func (c CmdBar) View() string {
	return styles.Bold.Render(c.prefix) + c.input.View()
}

func (c *CmdBar) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return cmd
}

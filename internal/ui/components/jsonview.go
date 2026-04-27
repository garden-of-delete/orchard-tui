package components

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2/quick"
	"golang.design/x/clipboard"

	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// PrettyJSON returns indented JSON. Falls back to raw on parse error.
func PrettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

// HighlightJSON syntax-highlights pretty JSON for terminal output. Falls
// back to plain text on chroma error.
func HighlightJSON(pretty string) string {
	var buf strings.Builder
	if err := quick.Highlight(&buf, pretty, "json", "terminal16m", "monokai"); err != nil {
		return pretty
	}
	return buf.String()
}

// HighlightMatches wraps every occurrence of needle (case-insensitive) in
// `s` with an inverted style. needle == "" returns s unchanged.
func HighlightMatches(s, needle string) string {
	if needle == "" {
		return s
	}
	hl := styles.ToastErr.Copy().Reverse(true).Bold(true)
	out := &strings.Builder{}
	low := strings.ToLower(s)
	lowN := strings.ToLower(needle)

	i := 0
	for {
		idx := strings.Index(low[i:], lowN)
		if idx < 0 {
			out.WriteString(s[i:])
			return out.String()
		}
		start := i + idx
		end := start + len(lowN)
		out.WriteString(s[i:start])
		out.WriteString(hl.Render(s[start:end]))
		i = end
	}
}

var (
	clipboardOnce sync.Once
	clipboardErr  error
)

// CopyToClipboard writes b as text. Returns nil on success or an error
// if clipboard support isn't available on this platform / build.
func CopyToClipboard(text string) error {
	clipboardOnce.Do(func() {
		clipboardErr = clipboard.Init()
	})
	if clipboardErr != nil {
		return clipboardErr
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}

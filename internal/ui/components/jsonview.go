package components

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2/quick"
	"golang.design/x/clipboard"

	"github.com/garden-of-delete/orchard-tui/internal/format"
	"github.com/garden-of-delete/orchard-tui/internal/ui/styles"
)

// PrettyJSON returns indented JSON. Round-trips the input through
// Unmarshal+Marshal so any literal control bytes that may have crept
// into string values (RFC 8259 forbids them, but a buggy or malicious
// producer could emit them) are re-emitted as \u00xx escapes by Go's
// encoder. UseNumber preserves integer precision for large numbers.
//
// Side effect: object keys are emitted in alphabetical order. For a
// pure read-only spec viewer this is a feature (predictable scanning,
// stable in-content search) more than a regression.
//
// Falls back to a sanitized version of the raw bytes if the input
// can't be parsed (e.g., the producer emitted invalid JSON with raw
// control bytes inside a string).
func PrettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return format.Sanitize(string(raw))
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // preserve <, >, & literals; control chars still escaped
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return format.Sanitize(string(raw))
	}
	return strings.TrimRight(buf.String(), "\n")
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

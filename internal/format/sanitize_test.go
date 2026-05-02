package format

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "hello world", "hello world"},
		{"empty", "", ""},
		{"unicode preserved", "héllo café", "héllo café"},
		{"multibyte preserved", "日本語", "日本語"},

		// The headline cases — control bytes that would otherwise
		// hijack the terminal.
		{"esc only", "\x1b", "·"},
		{"clear screen", "\x1b[2J", "·[2J"},
		{"cursor home", "\x1b[H", "·[H"},
		{"set title", "\x1b]0;evil\x07", "·]0;evil·"},
		{"OSC 52 clipboard hijack",
			"prefix\x1b]52;c;ZXZpbA==\x07suffix",
			"prefix·]52;c;ZXZpbA==·suffix"},
		{"BEL", "ring\x07", "ring·"},

		// Other control bytes.
		{"null", "a\x00b", "a·b"},
		{"DEL", "a\x7fb", "a·b"},
		{"tab", "a\tb", "a·b"},
		{"newline", "line1\nline2", "line1·line2"},
		{"crlf", "a\r\nb", "a··b"},

		// Combinations.
		{"interleaved", "a\x1b[31mred\x1b[0mb", "a·[31mred·[0mb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Sanitize(c.in)
			if got != c.want {
				t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
			}
			// Sanitized output must contain no control characters.
			if strings.ContainsAny(got, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f") {
				t.Errorf("Sanitize(%q) returned %q which still contains control bytes", c.in, got)
			}
		})
	}
}

package format

import (
	"strings"
	"unicode"
)

// Sanitize replaces every Unicode control character (incl. ESC, BEL,
// DEL, tabs, newlines) with U+00B7. Apply to externally-sourced
// strings before terminal rendering — and before lipgloss styling,
// since our own ANSI is intentional and must not be stripped.
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '·'
		}
		return r
	}, s)
}

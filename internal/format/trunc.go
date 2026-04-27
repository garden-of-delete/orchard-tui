package format

import (
	"strings"
	"unicode/utf8"
)

// Trunc shortens s to at most max runes, appending "…" when truncated.
// max <= 1 returns s unchanged for sanity.
func Trunc(s string, max int) string {
	if max <= 1 {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	out := make([]rune, 0, max)
	for i, r := range s {
		_ = i
		if len(out) == max-1 {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}

// FirstLine returns just the first line of s, with later lines indicated by "↩".
func FirstLine(s string) string {
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		return s
	}
	return s[:i] + " ↩"
}

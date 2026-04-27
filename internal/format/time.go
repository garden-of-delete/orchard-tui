// Package format provides display-only formatting helpers.
package format

import (
	"fmt"
	"time"
)

// RelTime renders a time as e.g. "12s ago", "5m ago", "2h ago", "3d ago".
// For zero or future times it returns "—".
func RelTime(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := now.Sub(t)
	if d < 0 {
		return "—"
	}
	switch {
	case d < time.Minute:
		s := int(d / time.Second)
		if s <= 0 {
			s = 1
		}
		return fmt.Sprintf("%ds ago", s)
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	default:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	}
}

// Duration formats a positive duration compactly: "23s", "4m12s", "2h13m", "3d4h".
func Duration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int(d/time.Second)%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int(d/time.Minute)%60)
	default:
		return fmt.Sprintf("%dd%dh", int(d/(24*time.Hour)), int(d/time.Hour)%24)
	}
}

// Between returns Duration(end - start) when both are set, else "—".
// If end is zero it uses now.
func Between(start, end, now time.Time) string {
	if start.IsZero() {
		return "—"
	}
	stop := end
	if stop.IsZero() {
		stop = now
	}
	return Duration(stop.Sub(start))
}

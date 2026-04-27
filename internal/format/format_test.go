package format

import (
	"testing"
	"time"
)

func TestRelTime(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "—"},
		{now, "1s ago"},
		{now.Add(-30 * time.Second), "30s ago"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-72 * time.Hour), "3d ago"},
		{now.Add(time.Hour), "—"},
	}
	for _, c := range cases {
		if got := RelTime(c.t, now); got != c.want {
			t.Errorf("RelTime(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{4*time.Minute + 12*time.Second, "4m12s"},
		{2*time.Hour + 13*time.Minute, "2h13m"},
		{3*24*time.Hour + 4*time.Hour, "3d4h"},
	}
	for _, c := range cases {
		if got := Duration(c.d); got != c.want {
			t.Errorf("Duration(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTrunc(t *testing.T) {
	if got := Trunc("hello", 5); got != "hello" {
		t.Errorf("Trunc no-op: %q", got)
	}
	if got := Trunc("hello world", 8); got != "hello w…" {
		t.Errorf("Trunc = %q", got)
	}
	if got := Trunc("héllo", 4); got != "hél…" {
		t.Errorf("unicode Trunc = %q", got)
	}
}

func TestFirstLine(t *testing.T) {
	if got := FirstLine("hello"); got != "hello" {
		t.Errorf("FirstLine no-op: %q", got)
	}
	if got := FirstLine("hello\nworld"); got != "hello ↩" {
		t.Errorf("FirstLine = %q", got)
	}
}

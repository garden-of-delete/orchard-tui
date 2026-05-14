package ui_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/garden-of-delete/orchard-tui/internal/config"
	"github.com/garden-of-delete/orchard-tui/internal/ui"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

// fakeOrchard returns an httptest.Server that serves canned fixtures.
func fakeOrchard(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/workflow", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "workflow_list.json"))
	})
	mux.HandleFunc("/v1/workflow/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/activities"):
			_, _ = w.Write(fixture(t, "activities.json"))
		case strings.HasSuffix(r.URL.Path, "/resources"):
			_, _ = w.Write(fixture(t, "resources.json"))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/v1/stats/counts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "stats_counts.json"))
	})
	mux.HandleFunc("/v1/stats/daily", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "stats_daily.json"))
	})
	mux.HandleFunc("/v1/stats/pattern", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "stats_pattern.json"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAppBootsAndShowsWorkflows(t *testing.T) {
	srv := fakeOrchard(t)

	cfg := config.Defaults
	cfg.Host = srv.URL
	cfg.PollFast = 50 * time.Millisecond
	cfg.PollMedium = 50 * time.Millisecond
	cfg.PollSlow = 50 * time.Millisecond

	app := ui.New(cfg)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 40))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "nightly-load")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppShowsHeaderCounts(t *testing.T) {
	srv := fakeOrchard(t)

	cfg := config.Defaults
	cfg.Host = srv.URL
	cfg.PollFast = 50 * time.Millisecond
	cfg.PollMedium = 50 * time.Millisecond
	cfg.PollSlow = 50 * time.Millisecond

	app := ui.New(cfg)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 40))

	// Header counts come from /v1/stats/counts; the running glyph + 12 should appear.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "▶") && strings.Contains(string(b), "12")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppHelpOverlay(t *testing.T) {
	srv := fakeOrchard(t)
	cfg := config.Defaults
	cfg.Host = srv.URL
	cfg.PollFast = 50 * time.Millisecond
	cfg.PollMedium = 50 * time.Millisecond
	cfg.PollSlow = 50 * time.Millisecond

	app := ui.New(cfg)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 40))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "nightly-load")
	}, teatest.WithDuration(3*time.Second))

	// Open help.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "Keybindings") && strings.Contains(s, "Global")
	}, teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppDrilldownToWorkflowDetail(t *testing.T) {
	srv := fakeOrchard(t)
	cfg := config.Defaults
	cfg.Host = srv.URL
	cfg.PollFast = 50 * time.Millisecond
	cfg.PollMedium = 50 * time.Millisecond
	cfg.PollSlow = 50 * time.Millisecond

	app := ui.New(cfg)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 40))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "nightly-load")
	}, teatest.WithDuration(3*time.Second))

	// Drill into the first workflow.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Workflow detail loads the activities response, which has "extract" and "transform".
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := string(b)
		return strings.Contains(s, "extract") && strings.Contains(s, "transform")
	}, teatest.WithDuration(3*time.Second))

	// Switch to resources tab via tab key.
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "emr-cluster")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppFilterCommand(t *testing.T) {
	srv := fakeOrchard(t)
	cfg := config.Defaults
	cfg.Host = srv.URL
	cfg.PollFast = 50 * time.Millisecond
	cfg.PollMedium = 50 * time.Millisecond
	cfg.PollSlow = 50 * time.Millisecond

	app := ui.New(cfg)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 40))

	// Wait for first load.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "nightly-load")
	}, teatest.WithDuration(3*time.Second))

	// Open filter, type "nightly", commit.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "nightly" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "1 workflow · /nightly")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

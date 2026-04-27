//go:build integration

// integration_test.go drives the App against a real orchard instance,
// captures the rendered terminal output, strips ANSI, and prints frames
// for analysis. Run with:
//
//	ORCHARD_HOST=http://localhost:9001 go test -tags integration -v ./internal/ui -run TestRealOrchardCapture
package ui_test

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/garden-of-delete/orchard-tui/internal/config"
	"github.com/garden-of-delete/orchard-tui/internal/ui"
)

// ansiRE matches CSI sequences (color/cursor/style), OSC sequences
// (titles/hyperlinks), and a few common single-char escapes.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[()][AB012]|\x1b[=>]|\x1b\(B`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// readUntilQuiet reads from r until no new bytes arrive for `quiet`.
func readUntilQuiet(r io.Reader, quiet time.Duration, max time.Duration) string {
	deadline := time.Now().Add(max)
	buf := make([]byte, 65536)
	var collected []byte
	lastRead := time.Now()
	for time.Now().Before(deadline) {
		// Use a tiny sleep to let the model render.
		time.Sleep(20 * time.Millisecond)
		n, _ := r.Read(buf)
		if n > 0 {
			collected = append(collected, buf[:n]...)
			lastRead = time.Now()
		} else if time.Since(lastRead) > quiet {
			break
		}
	}
	return string(collected)
}

func TestRealOrchardCapture(t *testing.T) {
	host := os.Getenv("ORCHARD_HOST")
	if host == "" {
		t.Skip("ORCHARD_HOST not set; skipping live capture")
	}
	cfg := config.Defaults
	cfg.Host = host
	cfg.PollFast = 1 * time.Second
	cfg.PollMedium = 1 * time.Second
	cfg.PollSlow = 5 * time.Second

	app := ui.New(cfg)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 36))
	out := tm.Output()

	// Frame 1: initial workflows list (after a fetch lands).
	frame := readUntilQuiet(out, 250*time.Millisecond, 3*time.Second)
	dumpFrame(t, "1. workflows list (initial)", frame)

	// Frame 2: drill into the first workflow → activities.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	frame = readUntilQuiet(out, 300*time.Millisecond, 3*time.Second)
	dumpFrame(t, "2. workflow detail — activities tab", frame)

	// Frame 3: switch tab → resources.
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	frame = readUntilQuiet(out, 300*time.Millisecond, 3*time.Second)
	dumpFrame(t, "3. workflow detail — resources tab", frame)

	// Frame 4: drill into the (only) resource → instances.
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	frame = readUntilQuiet(out, 300*time.Millisecond, 3*time.Second)
	dumpFrame(t, "4. resource detail — instances", frame)

	// Frame 5: back, back, then stats screen.
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	frame = readUntilQuiet(out, 400*time.Millisecond, 3*time.Second)
	dumpFrame(t, "5. stats screen", frame)

	// Frame 6: open help overlay.
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	frame = readUntilQuiet(out, 200*time.Millisecond, 2*time.Second)
	dumpFrame(t, "6. help overlay", frame)

	// Quit cleanly.
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func dumpFrame(t *testing.T, label, raw string) {
	t.Helper()
	clean := stripANSI(raw)
	// Take the LAST screenful of output: terminals issue a clear/repaint
	// per render, so the trailing chunk is the freshest view.
	lines := strings.Split(clean, "\n")
	const lookback = 60
	if len(lines) > lookback {
		lines = lines[len(lines)-lookback:]
	}
	fmt.Printf("\n========== FRAME: %s ==========\n", label)
	for _, ln := range lines {
		fmt.Println(ln)
	}
	fmt.Println("========================================")
}

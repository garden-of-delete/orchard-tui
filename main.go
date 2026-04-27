// orchard-tui is a terminal UI for the orchard data-pipeline orchestration
// service. It is intended to run inside the orchard pod against
// http://localhost:9000.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/garden-of-delete/orchard-tui/internal/config"
	"github.com/garden-of-delete/orchard-tui/internal/ui"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	var (
		hostFlag    = flag.String("host", "", "orchard base URL (overrides "+config.EnvHost+")")
		showVersion = flag.Bool("version", false, "print version and exit")
		printConfig = flag.Bool("print-config", false, "print resolved config and exit")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println("orchard-tui", Version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	if *hostFlag != "" {
		cfg.Host = *hostFlag
	}

	if *printConfig {
		fmt.Println(cfg.String())
		return
	}

	closeLog := setupLog(cfg.LogFile)
	defer closeLog()

	app := ui.New(cfg)
	prog := tea.NewProgram(app, tea.WithAltScreen(), tea.WithReportFocus())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupLog(path string) func() {
	if path == "" {
		log.SetOutput(io.Discard)
		return func() {}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "log:", err)
		log.SetOutput(io.Discard)
		return func() {}
	}
	log.SetOutput(f)
	log.Printf("orchard-tui %s starting", Version)
	return func() { _ = f.Close() }
}

func usage() {
	fmt.Fprintln(os.Stderr, `orchard-tui — terminal UI for orchard

USAGE
  orchard-tui [flags]

FLAGS`)
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, `
ENVIRONMENT
  ORCHARD_HOST           orchard base URL (default: http://localhost:9000)
  ORCHARD_API_KEY        optional x-api-key header value
  ORCHARD_POLL_FAST      poll interval for active screens (default: 2s)
  ORCHARD_POLL_MEDIUM    poll interval for header counts (default: 10s)
  ORCHARD_POLL_SLOW      poll interval for stats (default: 60s)
  ORCHARD_LOG            file path to write logs to (default: none)`)
}

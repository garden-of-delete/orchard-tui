package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/ui/screens"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

// commandKeywords is the closed set of single-token commands. Sorted
// for deterministic completion ordering.
var commandKeywords = func() []string {
	ks := []string{
		"home", "wf", "workflows",
		"pending", "running", "finished", "canceled", "failed",
		"stats", "help", "quit", "exit",
	}
	sort.Strings(ks)
	return ks
}()

// CompleteCommand returns the longest common prefix of the matches for the
// (possibly partial) first token of `input`, plus a list of all matches.
// Used for tab-completion in the cmd bar.
func CompleteCommand(input string) (string, []string) {
	parts := strings.Fields(input)
	if len(parts) > 1 {
		// Don't try to complete arguments yet.
		return input, nil
	}
	prefix := ""
	if len(parts) == 1 {
		prefix = parts[0]
	}
	matches := []string{}
	for _, k := range commandKeywords {
		if strings.HasPrefix(k, prefix) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		return input, nil
	}
	if len(matches) == 1 {
		return matches[0] + " ", matches
	}
	return longestCommonPrefix(matches), matches
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}

// parseCommand interprets a string from the `:` cmd bar and returns
// either a tea.Cmd to dispatch, or a Cmd that emits an error toast.
func (a *App) parseCommand(raw string) tea.Cmd {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return nil
	}
	cmd, args := parts[0], parts[1:]

	switch cmd {
	case "q", "quit", "exit":
		return tea.Quit

	case "home", "wf", "workflows":
		if len(args) == 0 {
			return uitypes.Replace(a.makeWorkflows(nil))
		}
		id := normalizeWorkflowID(args[0])
		return uitypes.Push(screens.NewWorkflowDetail(a.client, id, a.cfg.PollFast))

	case "pending":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusPending}))
	case "running":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusRunning}))
	case "finished":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusFinished}))
	case "canceled", "cancelled":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusCanceled}))
	case "failed":
		return uitypes.Replace(a.makeWorkflows([]api.Status{api.StatusFailed, api.StatusCascadeFailed, api.StatusTimeout}))

	case "stats":
		return uitypes.Push(screens.NewStats(a.client, a.cfg.PollSlow))

	case "help", "?":
		a.mode = uitypes.ModeHelp
		return nil

	default:
		return uitypes.Toast(uitypes.ToastErr, "unknown command: "+cmd)
	}
}

func normalizeWorkflowID(id string) string {
	if strings.HasPrefix(id, "wf-") {
		return id
	}
	return "wf-" + id
}

func (a *App) makeWorkflows(statuses []api.Status) uitypes.Screen {
	return screens.NewWorkflows(a.client, statuses, a.cfg.PollFast, a.cfg.PollMedium, a.cfg.PollSlow)
}

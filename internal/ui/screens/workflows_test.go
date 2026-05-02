package screens

import (
	"testing"
	"time"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/ui/uitypes"
)

// TestApplyFilterDoesNotCorruptRows is a regression test for the slice
// aliasing bug where applyFilter, after a no-filter pass that sets
// w.visible = w.rows, would later truncate the alias and append into
// w.rows's backing array — silently overwriting w.rows.
func TestApplyFilterDoesNotCorruptRows(t *testing.T) {
	w := NewWorkflows(nil, nil, time.Second, time.Second, time.Second)
	w.rows = []api.Workflow{
		{ID: "wf-1", Name: "alpha", Status: api.StatusRunning},
		{ID: "wf-2", Name: "beta", Status: api.StatusFinished},
		{ID: "wf-3", Name: "gamma", Status: api.StatusRunning},
	}
	original := append([]api.Workflow(nil), w.rows...)

	// 1. Empty filter — w.visible aliases w.rows.
	_, _ = w.Update(uitypes.FilterClearedMsg{})
	if !equalWorkflowSlices(w.rows, original) {
		t.Fatalf("rows changed after empty filter: %v", w.rows)
	}

	// 2. Now filter for something that excludes "beta". With the bug
	// present, the append loop would overwrite w.rows[1] with the next
	// matching element ("gamma"), corrupting w.rows.
	_, _ = w.Update(uitypes.FilterCommittedMsg{Query: "alpha"})
	if !equalWorkflowSlices(w.rows, original) {
		t.Errorf("rows corrupted by filter pass: got %v, want %v", w.rows, original)
	}

	// 3. Cycle through more filter changes; rows must remain untouched.
	for _, q := range []string{"gamma", "running", "finished", ""} {
		_, _ = w.Update(uitypes.FilterCommittedMsg{Query: q})
		if !equalWorkflowSlices(w.rows, original) {
			t.Errorf("rows corrupted after filter %q: got %v, want %v", q, w.rows, original)
		}
	}
}

func equalWorkflowSlices(a, b []api.Workflow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Name != b[i].Name || a[i].Status != b[i].Status {
			return false
		}
	}
	return true
}

// TestWorkflowsFilterCancelRestores exercises the vim-like cancel
// semantics for `/`-mode: opening the filter, typing into it, then
// canceling restores the previously-committed filter rather than
// clearing it.
func TestWorkflowsFilterCancelRestores(t *testing.T) {
	w := NewWorkflows(nil, nil, time.Second, time.Second, time.Second)

	// Seed a few rows so applyFilter has something to operate on.
	w.rows = []api.Workflow{
		{ID: "wf-1", Name: "nightly", Status: api.StatusRunning},
		{ID: "wf-2", Name: "demo", Status: api.StatusFinished},
	}

	// step delivers a tea.Msg to the screen and discards the returned cmd.
	step := func(_ string, msg any) {
		t.Helper()
		_, _ = w.Update(msg)
	}

	// 1. Commit a filter for "nightly".
	step("commit-nightly", uitypes.FilterCommittedMsg{Query: "nightly"})
	if w.filter != "nightly" {
		t.Fatalf("after commit: filter = %q, want %q", w.filter, "nightly")
	}
	if got := len(w.visible); got != 1 || w.visible[0].Name != "nightly" {
		t.Fatalf("after commit: visible=%v", w.visible)
	}

	// 2. Open filter input — should snapshot the current filter.
	step("enter-mode", uitypes.FilterEnterMsg{})
	if w.preFilter != "nightly" {
		t.Fatalf("after enter: preFilter = %q, want %q", w.preFilter, "nightly")
	}

	// 3. Type a no-match string — live filter shouldn't match anything.
	step("change-zzzz", uitypes.FilterChangedMsg{Query: "zzzz"})
	if w.filter != "zzzz" {
		t.Fatalf("after change: filter = %q, want %q", w.filter, "zzzz")
	}
	if got := len(w.visible); got != 0 {
		t.Fatalf("after change: visible len=%d, want 0", got)
	}

	// 4. Cancel — should restore the prior committed filter.
	step("cancel", uitypes.FilterCancelMsg{})
	if w.filter != "nightly" {
		t.Errorf("after cancel: filter = %q, want %q (restored from snapshot)", w.filter, "nightly")
	}
	if w.preFilter != "" {
		t.Errorf("after cancel: preFilter = %q, want empty (cleared)", w.preFilter)
	}
	if got := len(w.visible); got != 1 || w.visible[0].Name != "nightly" {
		t.Errorf("after cancel: visible=%v, want only nightly", w.visible)
	}
}

// TestWorkflowsStaleLoadedMsgDropped verifies the stale-fetch guard:
// a loaded message whose seq is older than the current fetchSeq is
// ignored, so a slow earlier fetch can't overwrite a faster later one.
func TestWorkflowsStaleLoadedMsgDropped(t *testing.T) {
	w := NewWorkflows(nil, nil, time.Second, time.Second, time.Second)

	// Pretend two fetches have been issued. Latest seq is 2.
	w.fetchSeq = 2
	w.rows = []api.Workflow{{ID: "current", Name: "current", Status: api.StatusRunning}}
	w.applyFilter()

	// Stale msg from the older (seq=1) fetch arrives — must be ignored.
	_, _ = w.Update(workflowsLoadedMsg{
		id:   w.id,
		seq:  1,
		rows: []api.Workflow{{ID: "stale", Name: "stale", Status: api.StatusRunning}},
	})

	if len(w.rows) != 1 || w.rows[0].ID != "current" {
		t.Errorf("stale msg overwrote rows: %+v", w.rows)
	}

	// Fresh msg from the latest (seq=2) fetch arrives — must be applied.
	_, _ = w.Update(workflowsLoadedMsg{
		id:   w.id,
		seq:  2,
		rows: []api.Workflow{{ID: "fresh", Name: "fresh", Status: api.StatusRunning}},
	})

	if len(w.rows) != 1 || w.rows[0].ID != "fresh" {
		t.Errorf("fresh msg not applied: %+v", w.rows)
	}
}

// TestWorkflowsFilterClearedDropsAll verifies the unconditional clear
// (used by esc at the root with no screen to pop) still works.
func TestWorkflowsFilterClearedDropsAll(t *testing.T) {
	w := NewWorkflows(nil, nil, time.Second, time.Second, time.Second)
	w.rows = []api.Workflow{{ID: "wf-1", Name: "x", Status: api.StatusRunning}}

	_, _ = w.Update(uitypes.FilterCommittedMsg{Query: "x"})
	_, _ = w.Update(uitypes.FilterClearedMsg{})

	if w.filter != "" || w.preFilter != "" {
		t.Errorf("after clear: filter=%q preFilter=%q, want empty", w.filter, w.preFilter)
	}
	if len(w.visible) != 1 {
		t.Errorf("after clear: visible len=%d, want 1", len(w.visible))
	}
}

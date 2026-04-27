package ui

import "testing"

func TestCompleteCommand(t *testing.T) {
	cases := []struct {
		in            string
		want          string
		wantMatchesGE int
	}{
		{"", "", 12},     // any prefix matches all keywords; LCP of all is ""
		{"sta", "stats ", 1},
		{"q", "quit ", 1},
		{"f", "f", 2},        // "failed" + "finished" → LCP is "f"
		{"r", "running ", 1}, // only "running" starts with "r"
		{"xyz", "xyz", 0},
		{"wf abc", "wf abc", 0}, // arguments not completed
	}
	for _, c := range cases {
		got, matches := CompleteCommand(c.in)
		if got != c.want {
			t.Errorf("CompleteCommand(%q) = %q, want %q (matches=%v)", c.in, got, c.want, matches)
		}
		if len(matches) < c.wantMatchesGE {
			t.Errorf("CompleteCommand(%q) matches=%v, want at least %d", c.in, matches, c.wantMatchesGE)
		}
	}
}

func TestNormalizeWorkflowID(t *testing.T) {
	cases := map[string]string{
		"wf-abc":          "wf-abc",
		"abc":             "wf-abc",
		"wf-abc-def-1234": "wf-abc-def-1234",
	}
	for in, want := range cases {
		if got := normalizeWorkflowID(in); got != want {
			t.Errorf("normalizeWorkflowID(%q) = %q, want %q", in, got, want)
		}
	}
}

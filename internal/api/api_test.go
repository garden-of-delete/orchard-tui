package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixturePath returns the absolute path to a fixture in ../../testdata.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", name)
}

// readFixture loads a fixture file as a byte slice.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// fakeServer registers handlers and returns a Client pointed at the server.
// `routes` maps "<METHOD> <path>" → handler.
func fakeServer(t *testing.T, routes map[string]http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	for k, h := range routes {
		// All test routes are GET; mux distinguishes by path only.
		_, path := splitMethodPath(t, k)
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := New(srv.URL, "")
	return c, srv
}

func splitMethodPath(t *testing.T, mp string) (string, string) {
	t.Helper()
	for i := 0; i < len(mp); i++ {
		if mp[i] == ' ' {
			return mp[:i], mp[i+1:]
		}
	}
	t.Fatalf("bad route key %q", mp)
	return "", ""
}

func writeFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(readFixture(t, name)); err != nil {
		t.Errorf("write fixture: %v", err)
	}
}

func TestHealthOK(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /__status": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		},
	})
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestHealthError(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /__status": func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "kaboom", http.StatusInternalServerError)
		},
	})
	err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.Status != 500 {
		t.Errorf("Status = %d", apiErr.Status)
	}
	if apiErr.Body != "kaboom" {
		t.Errorf("Body = %q", apiErr.Body)
	}
}

// TestPathParamsEscaped verifies that user-controlled path components
// (workflow / activity / resource IDs) are URL-escaped, so a stray '?',
// '/', '#', etc. can't tear the request URL into a different shape.
func TestPathParamsEscaped(t *testing.T) {
	var seenWire string
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow/": func(w http.ResponseWriter, r *http.Request) {
			// RequestURI is the raw wire form, so percent-escapes are
			// preserved (r.URL.Path would already be decoded).
			seenWire = r.RequestURI
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workflow":{"id":"x","name":"x","status":"running","createdAt":"2026-01-01T00:00:00","activatedAt":null,"terminatedAt":null},"activities":[]}`))
		},
	})

	if _, err := c.GetActivities(context.Background(), "wf-foo?break=true/bar"); err != nil {
		t.Fatalf("GetActivities: %v", err)
	}
	// '?' must be %3F and '/' must be %2F so neither tears the URL.
	if !strings.Contains(seenWire, "%3F") || !strings.Contains(seenWire, "%2F") {
		t.Errorf("wire = %q, want %%3F and %%2F", seenWire)
	}
	if !strings.HasSuffix(seenWire, "/activities") {
		t.Errorf("wire = %q, want ending in /activities", seenWire)
	}
}

// TestAPIErrorSanitizesBody verifies that control bytes in an orchard
// error response body are stripped from the formatted Error() string,
// preventing ANSI injection into the toast / inline error display.
func TestAPIErrorSanitizesBody(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /__status": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("evil\x1b[2Jpayload"))
		},
	})
	err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	if strings.ContainsRune(got, '\x1b') {
		t.Errorf("Error() leaked ESC byte: %q", got)
	}
	if !strings.Contains(got, "evil·[2Jpayload") {
		t.Errorf("Error() = %q, want sanitized body", got)
	}
}

func TestAPIKeyHeaderSent(t *testing.T) {
	var seenKey string
	srvHit := false
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /__status": func(w http.ResponseWriter, r *http.Request) {
			seenKey = r.Header.Get("x-api-key")
			srvHit = true
			w.WriteHeader(http.StatusOK)
		},
	})
	c.APIKey = "topsecret"
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !srvHit {
		t.Fatal("server not hit")
	}
	if seenKey != "topsecret" {
		t.Errorf("x-api-key = %q", seenKey)
	}
}

func TestListWorkflowsBuildsQueryAndDecodes(t *testing.T) {
	var seen url.Values
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow": func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Query()
			writeFixture(t, w, "workflow_list.json")
		},
	})

	got, err := c.ListWorkflows(context.Background(), ListWorkflowsOpts{
		Like:     "nightly",
		Statuses: []Status{StatusRunning, StatusFinished},
		OrderBy:  OrderByCreatedAt,
		Order:    OrderDesc,
		Page:     2,
		PerPage:  25,
	})
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}

	wantQ := url.Values{
		"like":     {"nightly"},
		"statuses": {"running,finished"},
		"order_by": {"created_at"},
		"order":    {"desc"},
		"page":     {"2"},
		"per_page": {"25"},
	}
	if seen.Encode() != wantQ.Encode() {
		t.Errorf("query = %q, want %q", seen.Encode(), wantQ.Encode())
	}

	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	wf := got[0]
	if wf.ID != "wf-f231a08f-60e4-480a-b845-e53e06918f77" {
		t.Errorf("ID = %q", wf.ID)
	}
	if wf.Name != "nightly-load" {
		t.Errorf("Name = %q", wf.Name)
	}
	if wf.Status != StatusRunning {
		t.Errorf("Status = %q", wf.Status)
	}
	if wf.CreatedAt.IsZero() {
		t.Error("CreatedAt zero")
	}
	if wf.ActivatedAt == nil {
		t.Error("ActivatedAt nil")
	}
	if wf.TerminatedAt != nil {
		t.Errorf("TerminatedAt = %v, want nil", wf.TerminatedAt)
	}

	// Pending row has nil activatedAt + terminatedAt.
	pending := got[2]
	if pending.Status != StatusPending {
		t.Errorf("got[2].Status = %q", pending.Status)
	}
	if pending.ActivatedAt != nil {
		t.Errorf("pending ActivatedAt = %v, want nil", pending.ActivatedAt)
	}
}

func TestListWorkflowsDefaultsLikeToWildcard(t *testing.T) {
	var seen url.Values
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow": func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Query()
			_, _ = w.Write([]byte("[]"))
		},
	})
	if _, err := c.ListWorkflows(context.Background(), ListWorkflowsOpts{}); err != nil {
		t.Fatal(err)
	}
	if seen.Get("like") != "%" {
		t.Errorf("like = %q, want %%", seen.Get("like"))
	}
	if seen.Has("statuses") {
		t.Errorf("statuses set when not requested: %q", seen.Get("statuses"))
	}
}

func TestGetActivities(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow/wf-x/activities": func(w http.ResponseWriter, r *http.Request) {
			writeFixture(t, w, "activities.json")
		},
	})
	got, err := c.GetActivities(context.Background(), "wf-x")
	if err != nil {
		t.Fatal(err)
	}
	if got.Workflow.Status != StatusRunning {
		t.Errorf("Workflow.Status = %q", got.Workflow.Status)
	}
	if len(got.Activities) != 2 {
		t.Fatalf("Activities len = %d", len(got.Activities))
	}
	first := got.Activities[0]
	if first.ActivityID != "1" || first.Name != "extract" {
		t.Errorf("Activities[0] = %+v", first)
	}
	// Spec is preserved as raw JSON.
	var spec map[string]any
	if err := json.Unmarshal(first.ActivitySpec, &spec); err != nil {
		t.Fatalf("activitySpec not valid json: %v", err)
	}
}

func TestGetActivityWithAttempts(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow/wf-x/activity/2": func(w http.ResponseWriter, r *http.Request) {
			writeFixture(t, w, "activity.json")
		},
	})
	got, err := c.GetActivity(context.Background(), "wf-x", "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("Attempts len = %d", len(got.Attempts))
	}
	if got.Attempts[0].Status != StatusFailed {
		t.Errorf("Attempt[0].Status = %q", got.Attempts[0].Status)
	}
	if got.Attempts[0].ErrorMessage == "" {
		t.Error("expected non-empty ErrorMessage on failed attempt")
	}
	if got.Attempts[1].TerminatedAt != nil {
		t.Errorf("running attempt has non-nil TerminatedAt: %v", got.Attempts[1].TerminatedAt)
	}
}

func TestGetResources(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow/wf-x/resources": func(w http.ResponseWriter, r *http.Request) {
			writeFixture(t, w, "resources.json")
		},
	})
	got, err := c.GetResources(context.Background(), "wf-x")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Resources) != 1 {
		t.Fatalf("Resources len = %d", len(got.Resources))
	}
	if got.Resources[0].TerminateAfter != 8 {
		t.Errorf("TerminateAfter = %v", got.Resources[0].TerminateAfter)
	}
}

func TestGetResource(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow/wf-x/resource/1": func(w http.ResponseWriter, r *http.Request) {
			writeFixture(t, w, "resource.json")
		},
	})
	got, err := c.GetResource(context.Background(), "wf-x", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Resource.Status != StatusRunning {
		t.Errorf("Resource.Status = %q", got.Resource.Status)
	}
	if len(got.Instances) != 1 || got.Instances[0].InstanceAttempt != 1 {
		t.Fatalf("Instances = %+v", got.Instances)
	}
}

func TestGetCounts(t *testing.T) {
	var seen url.Values
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/stats/counts": func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Query()
			writeFixture(t, w, "stats_counts.json")
		},
	})
	got, err := c.GetCounts(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if seen.Get("days") != "30" {
		t.Errorf("days = %q", seen.Get("days"))
	}
	if got[StatusRunning] != 12 || got[StatusFinished] != 85 {
		t.Errorf("counts = %+v", got)
	}
}

func TestGetCountsZeroDaysOmitsParam(t *testing.T) {
	var seen url.Values
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/stats/counts": func(w http.ResponseWriter, r *http.Request) {
			seen = r.URL.Query()
			_, _ = w.Write([]byte("{}"))
		},
	})
	if _, err := c.GetCounts(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if seen.Has("days") {
		t.Errorf("days unexpectedly set: %q", seen.Get("days"))
	}
}

func TestGetDaily(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/stats/daily": func(w http.ResponseWriter, r *http.Request) {
			writeFixture(t, w, "stats_daily.json")
		},
	})
	got, err := c.GetDaily(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Date.Time.Year() != 2026 {
		t.Errorf("first date.Year = %d", got[0].Date.Time.Year())
	}
	if got[0].Status != StatusFinished {
		t.Errorf("first status = %q", got[0].Status)
	}
}

func TestGetPattern(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/stats/pattern": func(w http.ResponseWriter, r *http.Request) {
			writeFixture(t, w, "stats_pattern.json")
		},
	})
	got, err := c.GetPattern(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("len = %d", len(got))
	}
	if got[2].DayOfWeek != 1 || got[2].Hour != 9 || got[2].Count != 11 {
		t.Errorf("pattern[2] = %+v", got[2])
	}
}

func TestContextCancelStopsRequest(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /__status": func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(2 * time.Second):
				w.WriteHeader(http.StatusOK)
			}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Health(ctx)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

func TestStatusHelpers(t *testing.T) {
	cases := []struct {
		s        Status
		terminal bool
		active   bool
	}{
		{StatusPending, false, false},
		{StatusActivating, false, true},
		{StatusRunning, false, true},
		{StatusFinished, true, false},
		{StatusFailed, true, false},
		{StatusCanceling, false, true},
		{StatusCanceled, true, false},
		{StatusTimeout, true, false},
	}
	for _, c := range cases {
		if got := c.s.IsTerminal(); got != c.terminal {
			t.Errorf("%s.IsTerminal() = %v, want %v", c.s, got, c.terminal)
		}
		if got := c.s.IsActive(); got != c.active {
			t.Errorf("%s.IsActive() = %v, want %v", c.s, got, c.active)
		}
	}
}

func TestOrchardTimeUnmarshalAcceptsBothLayouts(t *testing.T) {
	cases := []string{
		`"2024-02-05T10:30:00"`,
		`"2024-02-05T10:30:00.123"`,
	}
	for _, in := range cases {
		var ot OrchardTime
		if err := ot.UnmarshalJSON([]byte(in)); err != nil {
			t.Errorf("UnmarshalJSON(%s): %v", in, err)
		}
		if ot.Time.Location() != time.UTC {
			t.Errorf("location = %v", ot.Time.Location())
		}
	}
}

func TestOrchardTimeUnmarshalNull(t *testing.T) {
	var ot OrchardTime
	if err := ot.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Errorf("UnmarshalJSON(null): %v", err)
	}
	if !ot.IsZero() {
		t.Error("expected zero")
	}
}

// TestGetJSONCapsResponseSize verifies the LimitReader cap on the
// success path: a response larger than maxResponseBytes errors instead
// of OOMing the process.
func TestGetJSONCapsResponseSize(t *testing.T) {
	c, _ := fakeServer(t, map[string]http.HandlerFunc{
		"GET /v1/workflow": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Stream an open-ended JSON array padded with garbage past
			// the cap; the decoder will EOF before it sees the closing
			// bracket.
			_, _ = w.Write([]byte(`[`))
			big := make([]byte, 4096)
			for i := range big {
				big[i] = 'x'
			}
			written := 1 // for the "["
			for written < maxResponseBytes+8192 {
				_, _ = w.Write([]byte(`{"id":"wf-`))
				_, _ = w.Write(big)
				_, _ = w.Write([]byte(`","name":"x","status":"running","createdAt":"2026-01-01T00:00:00","activatedAt":null,"terminatedAt":null},`))
				written += 4096 + 110
			}
		},
	})

	_, err := c.ListWorkflows(context.Background(), ListWorkflowsOpts{})
	if err == nil {
		t.Fatal("expected error from oversize response, got nil")
	}
	// Don't pin the error string — Go's encoding/json may surface this
	// as ErrUnexpectedEOF or as a decode error wrapped in our prefix.
	if !strings.Contains(err.Error(), "decode /v1/workflow") {
		t.Errorf("error = %q, want containing %q", err.Error(), "decode /v1/workflow")
	}
}

func TestOrchardDateRoundTrip(t *testing.T) {
	in := `"2024-02-05"`
	var d OrchardDate
	if err := d.UnmarshalJSON([]byte(in)); err != nil {
		t.Fatal(err)
	}
	if d.Time.Year() != 2024 || d.Time.Month() != time.February || d.Time.Day() != 5 {
		t.Errorf("date = %v", d.Time)
	}
	out, err := d.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Errorf("Marshal = %s, want %s", out, in)
	}
}

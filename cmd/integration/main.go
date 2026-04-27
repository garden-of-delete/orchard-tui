// integration is a manual probe that exercises the api package against a
// live orchard. Run it explicitly:
//
//	ORCHARD_HOST=http://localhost:9001 go run ./cmd/integration
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/garden-of-delete/orchard-tui/internal/api"
	"github.com/garden-of-delete/orchard-tui/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail("config: %v", err)
	}
	fmt.Printf("== orchard-tui API integration probe ==\n")
	fmt.Printf("host: %s\n", cfg.Host)

	client := api.New(cfg.Host, cfg.APIKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	section("HEALTH")
	if err := client.Health(ctx); err != nil {
		fmt.Printf("  health: ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  health: OK")

	section("LIST WORKFLOWS (all)")
	wfs, err := client.ListWorkflows(ctx, api.ListWorkflowsOpts{})
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}
	fmt.Printf("  count: %d\n", len(wfs))
	for i, w := range wfs {
		if i >= 5 {
			fmt.Printf("  … and %d more\n", len(wfs)-5)
			break
		}
		fmt.Printf("  - %s  %-20s  status=%-10s  created=%s\n",
			w.ID, trunc(w.Name, 20), w.Status, w.CreatedAt.Time.Format(time.RFC3339))
	}

	section("STATS COUNTS")
	counts, err := client.GetCounts(ctx, 0)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		dump(counts)
	}

	section("STATS DAILY (30d)")
	daily, err := client.GetDaily(ctx, 30)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  rows: %d\n", len(daily))
		for i, d := range daily {
			if i >= 5 {
				fmt.Printf("  … and %d more\n", len(daily)-5)
				break
			}
			fmt.Printf("  - %s  %-10s  %d\n", d.Date.Time.Format("2006-01-02"), d.Status, d.Count)
		}
	}

	section("STATS PATTERN (30d)")
	pattern, err := client.GetPattern(ctx, 30)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  rows: %d\n", len(pattern))
		for i, p := range pattern {
			if i >= 5 {
				fmt.Printf("  … and %d more\n", len(pattern)-5)
				break
			}
			fmt.Printf("  - dow=%d hour=%-2d count=%d\n", p.DayOfWeek, p.Hour, p.Count)
		}
	}

	if len(wfs) == 0 {
		section("DRILLDOWN (skipped)")
		fmt.Println("  no workflows present — create one to exercise activities/resources/attempts/instances")
		return
	}

	wf := wfs[0]
	section("ACTIVITIES (" + wf.ID + ")")
	acts, err := client.GetActivities(ctx, wf.ID)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		acts = nil
	} else {
		fmt.Printf("  workflow: %s (%s)\n", acts.Workflow.Name, acts.Workflow.Status)
		fmt.Printf("  activities: %d\n", len(acts.Activities))
		for _, a := range acts.Activities {
			fmt.Printf("    - %s  %-20s  %-30s  status=%-10s  res=%s\n",
				a.ActivityID, trunc(a.Name, 20), a.ActivityType, a.Status, a.ResourceID)
		}
	}

	section("RESOURCES (" + wf.ID + ")")
	resp, err := client.GetResources(ctx, wf.ID)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  workflow: %s (%s)\n", resp.Workflow.Name, resp.Workflow.Status)
		fmt.Printf("  resources: %d\n", len(resp.Resources))
		for _, r := range resp.Resources {
			fmt.Printf("    - %s  %-20s  %-30s  status=%-10s  ttl=%.1fh\n",
				r.ResourceID, trunc(r.Name, 20), r.ResourceType, r.Status, r.TerminateAfter)
		}
		if len(resp.Resources) > 0 {
			rsc := resp.Resources[0]
			section("RESOURCE INSTANCES (" + wf.ID + " / " + rsc.ResourceID + ")")
			r2, err := client.GetResource(ctx, wf.ID, rsc.ResourceID)
			if err != nil {
				fmt.Printf("  ERROR: %v\n", err)
			} else {
				fmt.Printf("  instances: %d\n", len(r2.Instances))
				for _, inst := range r2.Instances {
					fmt.Printf("    - inst#%d  status=%-10s  err=%q\n",
						inst.InstanceAttempt, inst.Status, trunc(inst.ErrorMessage, 40))
				}
			}
		}
	}

	if acts != nil && len(acts.Activities) > 0 {
		first := acts.Activities[0]
		section("ACTIVITY ATTEMPTS (" + wf.ID + " / " + first.ActivityID + ")")
		a2, err := client.GetActivity(ctx, wf.ID, first.ActivityID)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			fmt.Printf("  attempts: %d\n", len(a2.Attempts))
			for _, at := range a2.Attempts {
				fmt.Printf("    - #%d  status=%-10s  err=%q\n",
					at.Attempt, at.Status, trunc(at.ErrorMessage, 40))
			}
		}
	}

	fmt.Println("\n== probe complete ==")
}

func section(s string) {
	fmt.Printf("\n--- %s ---\n", s)
}

func dump(v any) {
	b, _ := json.MarshalIndent(v, "  ", "  ")
	fmt.Printf("  %s\n", string(b))
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

package screens

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/garden-of-delete/orchard-tui/internal/api"
)

// awsRegion picks a sensible default region from the env so AWS console
// URLs aren't blank when the user hasn't set NEXT_PUBLIC_AWS_REGION
// (mirroring orchard-ui's convention) or AWS_REGION.
func awsRegion() string {
	for _, k := range []string{"NEXT_PUBLIC_AWS_REGION", "AWS_REGION", "AWS_DEFAULT_REGION"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "us-east-1"
}

// awsURLForActivity returns a console URL for the activity attempt's external
// resource, when the activity type is recognized. Empty string means no link.
func awsURLForActivity(a *api.Activity, att api.ActivityAttempt) string {
	if a == nil {
		return ""
	}
	switch a.ActivityType {
	case "aws.activity.ShellScriptActivity":
		// Mirror orchard-ui: link to SSM Run Command for the command id
		// embedded in attemptSpec.commandId, when present.
		var spec map[string]any
		if err := json.Unmarshal(att.AttemptSpec, &spec); err == nil {
			if cmd, ok := spec["commandId"].(string); ok && cmd != "" {
				region := awsRegion()
				return fmt.Sprintf(
					"https://%s.console.aws.amazon.com/systems-manager/run-command/%s?region=%s",
					region, url.PathEscape(cmd), region,
				)
			}
		}
	}
	return ""
}

// awsURLForResource returns a console URL for the resource instance, when the
// resource type is recognized.
func awsURLForResource(r *api.Resource, inst api.ResourceInstance) string {
	if r == nil {
		return ""
	}
	region := awsRegion()
	switch r.ResourceType {
	case "aws.resource.EmrResource":
		var spec map[string]any
		if err := json.Unmarshal(inst.InstanceSpec, &spec); err == nil {
			if id, ok := spec["clusterId"].(string); ok && id != "" {
				return fmt.Sprintf(
					"https://%s.console.aws.amazon.com/emr/home?region=%s#/clusterDetails/%s",
					region, region, url.PathEscape(id),
				)
			}
		}
	case "aws.resource.Ec2Resource":
		var spec map[string]any
		if err := json.Unmarshal(inst.InstanceSpec, &spec); err == nil {
			if id, ok := spec["instanceId"].(string); ok && id != "" {
				return fmt.Sprintf(
					"https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#InstanceDetails:instanceId=%s",
					region, region, url.PathEscape(id),
				)
			}
		}
	}
	return ""
}

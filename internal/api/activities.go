package api

import (
	"context"
	"net/url"
)

// GetActivities fetches a workflow plus its activities.
func (c *Client) GetActivities(ctx context.Context, workflowID string) (*ActivitiesResponse, error) {
	var out ActivitiesResponse
	path := "/v1/workflow/" + url.PathEscape(workflowID) + "/activities"
	if err := c.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetActivity fetches an activity plus its attempts.
func (c *Client) GetActivity(ctx context.Context, workflowID, activityID string) (*ActivityAttemptsResponse, error) {
	var out ActivityAttemptsResponse
	path := "/v1/workflow/" + url.PathEscape(workflowID) + "/activity/" + url.PathEscape(activityID)
	if err := c.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

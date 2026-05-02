package api

import (
	"context"
	"net/url"
)

// GetResources fetches a workflow plus its resources.
func (c *Client) GetResources(ctx context.Context, workflowID string) (*ResourcesResponse, error) {
	var out ResourcesResponse
	path := "/v1/workflow/" + url.PathEscape(workflowID) + "/resources"
	if err := c.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetResource fetches a resource plus its instances.
func (c *Client) GetResource(ctx context.Context, workflowID, resourceID string) (*ResourceInstancesResponse, error) {
	var out ResourceInstancesResponse
	path := "/v1/workflow/" + url.PathEscape(workflowID) + "/resource/" + url.PathEscape(resourceID)
	if err := c.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

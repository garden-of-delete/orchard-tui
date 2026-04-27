package api

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

// OrderBy is the column workflow listings can be sorted by.
type OrderBy string

const (
	OrderByCreatedAt    OrderBy = "created_at"
	OrderByActivatedAt  OrderBy = "activated_at"
	OrderByTerminatedAt OrderBy = "terminated_at"
)

// Order is the sort direction.
type Order string

const (
	OrderDesc Order = "desc"
	OrderAsc  Order = "asc"
)

// ListWorkflowsOpts configures GET /v1/workflow.
//
// Like is a SQL LIKE pattern. Empty string is treated as "%" (match all)
// because the orchard endpoint requires the parameter to be present.
type ListWorkflowsOpts struct {
	Like     string
	Statuses []Status
	OrderBy  OrderBy
	Order    Order
	Page     int // 1-based; <=0 means use server default (1)
	PerPage  int // <=0 means use server default (50)
}

// ListWorkflows fetches a paginated, filterable list of workflows.
func (c *Client) ListWorkflows(ctx context.Context, opts ListWorkflowsOpts) ([]Workflow, error) {
	q := url.Values{}

	like := opts.Like
	if like == "" {
		like = "%"
	}
	q.Set("like", like)

	if len(opts.Statuses) > 0 {
		ss := make([]string, len(opts.Statuses))
		for i, s := range opts.Statuses {
			ss[i] = string(s)
		}
		q.Set("statuses", strings.Join(ss, ","))
	}
	if opts.OrderBy != "" {
		q.Set("order_by", string(opts.OrderBy))
	}
	if opts.Order != "" {
		q.Set("order", string(opts.Order))
	}
	if opts.Page > 0 {
		q.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(opts.PerPage))
	}

	var out []Workflow
	if err := c.getJSON(ctx, "/v1/workflow", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

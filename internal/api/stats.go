package api

import (
	"context"
	"net/url"
	"strconv"
)

// GetCounts returns workflow counts by status over the past `days`.
// Pass days<=0 to use the server default (365).
func (c *Client) GetCounts(ctx context.Context, days int) (StatusCounts, error) {
	q := daysQuery(days)
	raw := map[string]int{}
	if err := c.getJSON(ctx, "/v1/stats/counts", q, &raw); err != nil {
		return nil, err
	}
	out := make(StatusCounts, len(raw))
	for k, v := range raw {
		out[Status(k)] = v
	}
	return out, nil
}

// GetDaily returns daily workflow counts by status over the past `days`.
// Pass days<=0 to use the server default (30).
func (c *Client) GetDaily(ctx context.Context, days int) ([]DailyCount, error) {
	var out []DailyCount
	if err := c.getJSON(ctx, "/v1/stats/daily", daysQuery(days), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPattern returns hour-of-day × day-of-week activity counts.
// Pass days<=0 to use the server default (30).
func (c *Client) GetPattern(ctx context.Context, days int) ([]PatternCount, error) {
	var out []PatternCount
	if err := c.getJSON(ctx, "/v1/stats/pattern", daysQuery(days), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func daysQuery(days int) url.Values {
	if days <= 0 {
		return nil
	}
	q := url.Values{}
	q.Set("days", strconv.Itoa(days))
	return q
}

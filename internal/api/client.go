// Package api is a thin Go client for orchard's HTTP API.
//
// The client is read-only: it only implements the GET endpoints needed by
// orchard-tui's v0.1 viewer. Mutation endpoints (POST/PUT/DELETE) live on
// orchard but are intentionally not surfaced here.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	apiKeyHeader   = "x-api-key"
)

// Client talks to a single orchard host.
type Client struct {
	BaseURL string
	APIKey  string // optional — sent as x-api-key when non-empty
	HTTP    *http.Client
}

// New returns a Client with sensible defaults.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: defaultTimeout},
	}
}

// Health pings GET /__status. Returns nil on 2xx, an APIError otherwise.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, "/__status", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// getJSON issues GET <path>?<query> and decodes the body into out.
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.do(ctx, http.MethodGet, c.urlFor(path, query), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("api: decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) urlFor(path string, query url.Values) string {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// do issues a request, returning an open response on 2xx or an APIError
// otherwise. The caller is responsible for closing the body on success.
func (c *Client) do(ctx context.Context, method, target string, body io.Reader) (*http.Response, error) {
	target = c.absolute(target)

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("api: build request %s %s: %w", method, target, err)
	}
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set(apiKeyHeader, c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api: %s %s: %w", method, target, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
	return nil, &APIError{
		Method: method,
		URL:    target,
		Status: resp.StatusCode,
		Body:   strings.TrimSpace(string(bodyBytes)),
	}
}

func (c *Client) absolute(target string) string {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	return c.BaseURL + target
}

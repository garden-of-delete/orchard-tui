// Package api is a thin Go client for orchard's HTTP API.
//
// The client is read-only by design: it only implements GET endpoints, and
// the only verb wired into the request helper is http.MethodGet. orchard's
// mutation endpoints (POST/PUT/DELETE) are intentionally not surfaced here
// and adding them is out of scope for this project.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	apiKeyHeader   = "x-api-key"

	// maxResponseBytes caps the success-path response body. Orders of
	// magnitude above any realistic orchard response — its largest
	// payload (a workflow's full activities + resources tree with all
	// attemptSpecs) is typically <100 KB. This bound prevents a runaway
	// or malformed response from OOMing the process.
	maxResponseBytes = 16 * 1024 * 1024
)

// Client talks to a single orchard host.
type Client struct {
	BaseURL string
	APIKey  string // optional — sent as x-api-key when non-empty
	HTTP    *http.Client
}

// New returns a Client with sensible defaults. baseURL is expected to
// be already free of any trailing slash (config.Load trims it); we
// don't re-trim here to keep this function trivially predictable.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
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
// The response body is read through an io.LimitReader to prevent a
// runaway response from OOMing the process; see maxResponseBytes.
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.do(ctx, http.MethodGet, c.urlFor(path, query), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := io.LimitReader(resp.Body, maxResponseBytes)
	if err := json.NewDecoder(body).Decode(out); err != nil {
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
//
// Errors are also logged via the stdlib `log` package so they end up in
// ORCHARD_LOG when set (main.go's setupLog wires that). This gives a
// chronological trail of API failures even for errors that the UI
// suppresses (e.g., the latched counts-error path).
func (c *Client) do(ctx context.Context, method, target string, body io.Reader) (*http.Response, error) {
	target = c.absolute(target)

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		log.Printf("api: build request %s %s: %v", method, target, err)
		return nil, fmt.Errorf("api: build request %s %s: %w", method, target, err)
	}
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set(apiKeyHeader, c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		log.Printf("api: %s %s: %v", method, target, err)
		return nil, fmt.Errorf("api: %s %s: %w", method, target, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
	apiErr := &APIError{
		Method: method,
		URL:    target,
		Status: resp.StatusCode,
		Body:   strings.TrimSpace(string(bodyBytes)),
	}
	log.Printf("%v", apiErr)
	return nil, apiErr
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

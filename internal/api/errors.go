package api

import (
	"fmt"

	"github.com/garden-of-delete/orchard-tui/internal/format"
)

// APIError is returned for non-2xx orchard responses.
type APIError struct {
	Method string
	URL    string
	Status int
	Body   string
}

// Error formats the APIError. The response Body is sanitized so that
// orchard error responses with literal control bytes (ESC, OSC, etc.)
// can't inject ANSI sequences into anything that calls err.Error() —
// the inline error display on screens and the footer toast both do.
// Method/URL/Status come from us, not the wire, so they're trusted.
func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.URL, e.Status)
	}
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.URL, e.Status, format.Sanitize(e.Body))
}

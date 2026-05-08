// Package config loads orchard-tui runtime configuration from environment
// variables. Defaults assume the binary runs inside the orchard pod.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	EnvHost       = "ORCHARD_HOST"
	EnvAPIKey     = "ORCHARD_TUI_API_KEY"
	EnvPollFast   = "ORCHARD_POLL_FAST"
	EnvPollMedium = "ORCHARD_POLL_MEDIUM"
	EnvPollSlow   = "ORCHARD_POLL_SLOW"
	EnvLogFile    = "ORCHARD_LOG"
)

type Config struct {
	Host       string
	APIKey     string
	PollFast   time.Duration
	PollMedium time.Duration
	PollSlow   time.Duration
	LogFile    string
}

var Defaults = Config{
	Host:       "http://localhost:9000",
	PollFast:   2 * time.Second,
	PollMedium: 10 * time.Second,
	PollSlow:   60 * time.Second,
}

// Load reads env vars and returns a Config with defaults applied.
// CLI flags take precedence and are applied by main.go after this returns.
func Load() (Config, error) {
	c := Defaults

	if v := os.Getenv(EnvHost); v != "" {
		c.Host = v
	}
	c.Host = strings.TrimSpace(c.Host)
	c.Host = strings.TrimRight(c.Host, "/")
	if !strings.HasPrefix(c.Host, "http://") && !strings.HasPrefix(c.Host, "https://") {
		return c, fmt.Errorf("%s must include scheme (http:// or https://): got %q", EnvHost, c.Host)
	}
	if strings.ContainsAny(c.Host, "\r\n\x00") {
		return c, fmt.Errorf("%s must not contain control characters", EnvHost)
	}

	c.APIKey = os.Getenv(EnvAPIKey)
	c.LogFile = os.Getenv(EnvLogFile)

	for env, dst := range map[string]*time.Duration{
		EnvPollFast:   &c.PollFast,
		EnvPollMedium: &c.PollMedium,
		EnvPollSlow:   &c.PollSlow,
	} {
		if v := os.Getenv(env); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return c, fmt.Errorf("%s: %w", env, err)
			}
			if d <= 0 {
				return c, fmt.Errorf("%s must be positive: got %s", env, d)
			}
			*dst = d
		}
	}

	return c, nil
}

func (c Config) String() string {
	api := "(none)"
	if c.APIKey != "" {
		api = "(set)"
	}
	return fmt.Sprintf(
		"host=%s api_key=%s poll_fast=%s poll_medium=%s poll_slow=%s log=%q",
		c.Host, api, c.PollFast, c.PollMedium, c.PollSlow, c.LogFile,
	)
}

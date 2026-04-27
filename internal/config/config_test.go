package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Host != Defaults.Host {
		t.Errorf("Host = %q, want %q", c.Host, Defaults.Host)
	}
	if c.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", c.APIKey)
	}
	if c.PollFast != Defaults.PollFast {
		t.Errorf("PollFast = %s, want %s", c.PollFast, Defaults.PollFast)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvHost, "http://orchard.internal:9000/")
	t.Setenv(EnvAPIKey, "abc123")
	t.Setenv(EnvPollFast, "500ms")
	t.Setenv(EnvPollMedium, "5s")
	t.Setenv(EnvPollSlow, "30s")
	t.Setenv(EnvLogFile, "/tmp/orchard-tui.log")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Host != "http://orchard.internal:9000" {
		t.Errorf("Host = %q (trailing slash should be trimmed)", c.Host)
	}
	if c.APIKey != "abc123" {
		t.Errorf("APIKey = %q", c.APIKey)
	}
	if c.PollFast != 500*time.Millisecond {
		t.Errorf("PollFast = %s", c.PollFast)
	}
	if c.PollMedium != 5*time.Second {
		t.Errorf("PollMedium = %s", c.PollMedium)
	}
	if c.PollSlow != 30*time.Second {
		t.Errorf("PollSlow = %s", c.PollSlow)
	}
	if c.LogFile != "/tmp/orchard-tui.log" {
		t.Errorf("LogFile = %q", c.LogFile)
	}
}

func TestLoadRejectsHostWithoutScheme(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvHost, "orchard.internal:9000")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Fatalf("expected scheme error, got %v", err)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvPollFast, "not-a-duration")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for bad duration")
	}
}

func TestLoadRejectsNonPositiveDuration(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvPollFast, "0s")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
}

func TestStringHidesAPIKey(t *testing.T) {
	c := Config{Host: "http://x", APIKey: "supersecret"}
	s := c.String()
	if strings.Contains(s, "supersecret") {
		t.Errorf("String() leaked api key: %q", s)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{EnvHost, EnvAPIKey, EnvPollFast, EnvPollMedium, EnvPollSlow, EnvLogFile} {
		t.Setenv(k, "")
	}
}

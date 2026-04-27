package api

import (
	"fmt"
	"strings"
	"time"
)

// OrchardTime wraps time.Time to deserialize Scala/Play
// LocalDateTime values, which are emitted as ISO-8601 strings without
// a timezone (e.g. "2024-02-05T10:30:00.123" or "2024-02-05T10:30:00").
// We treat them as UTC.
type OrchardTime struct{ time.Time }

var orchardTimeLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
}

func (t *OrchardTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		t.Time = time.Time{}
		return nil
	}
	for _, layout := range orchardTimeLayouts {
		if parsed, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("api: cannot parse OrchardTime %q", s)
}

func (t OrchardTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.UTC().Format(orchardTimeLayouts[1]) + `"`), nil
}

func (t OrchardTime) IsZero() bool { return t.Time.IsZero() }

// OrchardDate wraps time.Time for Scala LocalDate values ("YYYY-MM-DD").
type OrchardDate struct{ time.Time }

const orchardDateLayout = "2006-01-02"

func (d *OrchardDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		d.Time = time.Time{}
		return nil
	}
	parsed, err := time.ParseInLocation(orchardDateLayout, s, time.UTC)
	if err != nil {
		return fmt.Errorf("api: cannot parse OrchardDate %q: %w", s, err)
	}
	d.Time = parsed
	return nil
}

func (d OrchardDate) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.UTC().Format(orchardDateLayout) + `"`), nil
}

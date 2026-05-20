package util

import (
	"strings"
	"testing"
)

// TestParseRelativeDate_IntegerOverflow verifies that absurdly large numeric
// inputs produce an error rather than silently producing the wrong date by
// ignoring strconv.Atoi's overflow.
func TestParseRelativeDate_IntegerOverflow(t *testing.T) {
	overflowInputs := []string{
		"+99999999999999999999d",
		"+99999999999999999999w",
		"+99999999999999999999m",
		"in 99999999999999999999 days",
		"in 99999999999999999999 weeks",
		"in 99999999999999999999 months",
		"in 99999999999999999999 years",
	}
	for _, in := range overflowInputs {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseRelativeDate(in); err == nil {
				t.Errorf("expected error for overflowing input %q, got nil", in)
			}
		})
	}
}

// TestParseFlexibleDate_IntegerOverflow covers the bare-shorthand path
// "Nd/Nw/Nm" which uses its own strconv.Atoi call.
func TestParseFlexibleDate_IntegerOverflow(t *testing.T) {
	overflowInputs := []string{
		"99999999999999999999d",
		"99999999999999999999w",
		"99999999999999999999m",
	}
	for _, in := range overflowInputs {
		t.Run(in, func(t *testing.T) {
			_, err := ParseFlexibleDate(in)
			if err == nil {
				t.Errorf("expected error for overflowing input %q, got nil", in)
				return
			}
			// Either the bare-shorthand fallback rejects it, or the underlying
			// ParseRelativeDate rejects it; both surface a non-nil error.
			if !strings.Contains(strings.ToLower(err.Error()), "invalid") &&
				!strings.Contains(strings.ToLower(err.Error()), "unrecognized") {
				t.Errorf("error %q does not mention invalid/unrecognized", err.Error())
			}
		})
	}
}

package config

import "fmt"

// GetWeekEndDay returns the configured week-end day abbreviation ("fri", "sat", or "sun").
// Defaults to "sun" if not set or invalid.
func (c *Config) GetWeekEndDay() string {
	switch c.WeekEndDay {
	case "fri", "sat", "sun":
		return c.WeekEndDay
	default:
		return "sun"
	}
}

// ValidateWeekEndDay returns an error if s is not a valid week-end day value.
// Valid values: "" (use default), "fri", "sat", "sun".
func ValidateWeekEndDay(s string) error {
	switch s {
	case "", "fri", "sat", "sun":
		return nil
	default:
		return fmt.Errorf("invalid week_end_day %q: must be fri, sat, or sun", s)
	}
}

// GetTimeFormat returns the configured time format string for Go's time.Format.
// "12h" → "3:04pm", "24h" (default) → "15:04". Safe to call on nil receiver.
func (c *Config) GetTimeFormat() string {
	if c != nil && c.TimeFormat == "12h" {
		return "3:04pm"
	}
	return "15:04"
}

// ValidateTimeFormat returns an error if s is not a valid time format value.
// Valid values: "" (use default), "12h", "24h".
func ValidateTimeFormat(s string) error {
	switch s {
	case "", "12h", "24h":
		return nil
	default:
		return fmt.Errorf("invalid time_format %q: must be 12h or 24h", s)
	}
}

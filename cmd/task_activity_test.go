package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func TestParseDuration_ValidInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1h", 1 * time.Hour},
		{"24h", 24 * time.Hour},
		{"1d", 1 * util.Day},
		{"7d", 7 * util.Day},
		{"1w", 1 * util.Week},
		{"2w", 2 * util.Week},
		{" 3d ", 3 * util.Day},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseDuration(tc.input)
			if err != nil {
				t.Fatalf("parseDuration(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseDuration_InvalidInputs(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{"", "empty string"},
		{"d", "missing number"},
		{"0d", "zero duration"},
		{"-3d", "negative duration"},
		{"-1w", "negative weeks"},
		{"3x", "unknown unit"},
		{"abc", "non-numeric"},
		{"1xd", "trailing junk in number"},
		{"2--w", "malformed number"},
		{"3 h", "embedded whitespace"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := parseDuration(tc.input)
			if err == nil {
				t.Errorf("parseDuration(%q) expected error for %s, got nil", tc.input, tc.desc)
			}
		})
	}
}

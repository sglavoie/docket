package tui

import (
	"math"
	"testing"
)

func TestValidateIntInRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		min, max  int
		wantVal   int
		wantErr   bool
		errMsg    string
	}{
		{"valid in range", "5", "count", 0, 10, 5, false, ""},
		{"at min", "0", "count", 0, 10, 0, false, ""},
		{"at max", "10", "count", 0, 10, 10, false, ""},
		{"empty required", "", "count", 0, 10, 0, true, "count is required"},
		{"whitespace only", "  ", "count", 0, 10, 0, true, "count is required"},
		{"not a number", "abc", "count", 0, 10, 0, true, "count must be a number"},
		{"below min", "-1", "count", 0, 10, 0, true, "count must be 0-10"},
		{"above max", "11", "count", 0, 10, 0, true, "count must be 0-10"},
		{"unbounded max", "999999", "count", 0, math.MaxInt, 999999, false, ""},
		{"below min unbounded", "-1", "count", 0, math.MaxInt, 0, true, "count must be 0 or greater"},
		{"with whitespace", "  7  ", "count", 0, 10, 7, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateIntInRange(tt.input, tt.fieldName, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIntInRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("validateIntInRange() error = %q, want %q", err.Error(), tt.errMsg)
			}
			if got != tt.wantVal {
				t.Errorf("validateIntInRange() = %d, want %d", got, tt.wantVal)
			}
		})
	}
}

func TestValidateIntOptionalInRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		min, max  int
		wantVal   int
		wantSet   bool
		wantErr   bool
	}{
		{"empty is ok", "", "hours", 0, 23, 0, false, false},
		{"whitespace empty", "  ", "hours", 0, 23, 0, false, false},
		{"valid value", "12", "hours", 0, 23, 12, true, false},
		{"not a number", "abc", "hours", 0, 23, 0, false, true},
		{"out of range", "25", "hours", 0, 23, 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, set, err := validateIntOptionalInRange(tt.input, tt.fieldName, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if set != tt.wantSet {
				t.Errorf("set = %v, want %v", set, tt.wantSet)
			}
			if got != tt.wantVal {
				t.Errorf("value = %d, want %d", got, tt.wantVal)
			}
		})
	}
}

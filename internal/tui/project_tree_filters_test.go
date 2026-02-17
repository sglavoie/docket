package tui

import (
	"testing"
)

func TestCycleFilter(t *testing.T) {
	t.Run("string filters", func(t *testing.T) {
		values := []string{"a", "b", "c"}

		tests := []struct {
			name    string
			current string
			want    string
		}{
			{"empty to first", "", "a"},
			{"first to second", "a", "b"},
			{"second to third", "b", "c"},
			{"last wraps to empty", "c", ""},
			{"unknown wraps to empty", "x", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cycleFilter(tt.current, values)
				if got != tt.want {
					t.Errorf("cycleFilter(%q, values) = %q, want %q", tt.current, got, tt.want)
				}
			})
		}
	})

	t.Run("int filters", func(t *testing.T) {
		values := []int{1, 2, 3}

		tests := []struct {
			name    string
			current int
			want    int
		}{
			{"zero to first", 0, 1},
			{"first to second", 1, 2},
			{"last wraps to zero", 3, 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := cycleFilter(tt.current, values)
				if got != tt.want {
					t.Errorf("cycleFilter(%d, values) = %d, want %d", tt.current, got, tt.want)
				}
			})
		}
	})

	t.Run("empty values", func(t *testing.T) {
		got := cycleFilter("a", []string{})
		if got != "" {
			t.Errorf("cycleFilter with empty values = %q, want empty", got)
		}
	})
}

package util

import (
	"reflect"
	"testing"
)

func TestSliceContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		value    string
		expected bool
	}{
		{"found at start", []string{"a", "b", "c"}, "a", true},
		{"found in middle", []string{"a", "b", "c"}, "b", true},
		{"found at end", []string{"a", "b", "c"}, "c", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"empty string in slice", []string{"", "a"}, "", true},
		{"single element found", []string{"only"}, "only", true},
		{"single element not found", []string{"only"}, "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SliceContains(tt.slice, tt.value)
			if result != tt.expected {
				t.Errorf("SliceContains(%v, %q) = %v, want %v", tt.slice, tt.value, result, tt.expected)
			}
		})
	}
}

func TestSliceRemove(t *testing.T) {
	tests := []struct {
		name           string
		slice          []string
		value          string
		expectedSlice  []string
		expectedRemove bool
	}{
		{"remove from start", []string{"a", "b", "c"}, "a", []string{"b", "c"}, true},
		{"remove from middle", []string{"a", "b", "c"}, "b", []string{"a", "c"}, true},
		{"remove from end", []string{"a", "b", "c"}, "c", []string{"a", "b"}, true},
		{"not found", []string{"a", "b", "c"}, "d", []string{"a", "b", "c"}, false},
		{"empty slice", []string{}, "a", []string{}, false},
		{"single element removed", []string{"only"}, "only", []string{}, true},
		{"single element not found", []string{"only"}, "other", []string{"only"}, false},
		{"removes first occurrence only", []string{"a", "b", "a"}, "a", []string{"b", "a"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, removed := SliceRemove(tt.slice, tt.value)
			if removed != tt.expectedRemove {
				t.Errorf("SliceRemove(%v, %q) removed = %v, want %v", tt.slice, tt.value, removed, tt.expectedRemove)
			}
			if !reflect.DeepEqual(result, tt.expectedSlice) {
				t.Errorf("SliceRemove(%v, %q) = %v, want %v", tt.slice, tt.value, result, tt.expectedSlice)
			}
		})
	}
}

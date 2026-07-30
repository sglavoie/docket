package format

import (
	"testing"
)

func TestLooksLikeMetadata_RecognizesRegistryKeys(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    bool
	}{
		{name: "core key", metaStr: "id: task-1", want: true},
		{name: "recurrence subfield", metaStr: "recur.original_day: 31", want: true},
		{name: "retired type-specific key", metaStr: "epic.scope_baselined: 2025-01-01", want: false},
		{name: "retired score key", metaStr: "score.impact: 5", want: false},
		{name: "unknown key", metaStr: "unknown: value", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LooksLikeMetadata(tc.metaStr); got != tc.want {
				t.Errorf("LooksLikeMetadata(%q) = %v, want %v", tc.metaStr, got, tc.want)
			}
		})
	}
}

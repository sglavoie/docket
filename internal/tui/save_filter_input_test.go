package tui

import (
	"strings"
	"testing"
)

func TestDescribeSnapshotFiltersAttrsDeterministic(t *testing.T) {
	attrs := map[string]string{
		"area":   "backend",
		"owner":  "alice",
		"sprint": "42",
		"team":   "core",
	}
	first := describeSnapshotFilters("", "", 0, nil, nil, 0, false, false, attrs, 0, 0, 0)
	for range 50 {
		got := describeSnapshotFilters("", "", 0, nil, nil, 0, false, false, attrs, 0, 0, 0)
		if got != first {
			t.Fatalf("describeSnapshotFilters output not deterministic:\nfirst: %q\ngot:   %q", first, got)
		}
	}

	// Verify all attrs appear sorted by key.
	expectedKeysInOrder := []string{"area=backend", "owner=alice", "sprint=42", "team=core"}
	prev := -1
	for _, k := range expectedKeysInOrder {
		idx := strings.Index(first, k)
		if idx < 0 {
			t.Fatalf("expected %q in output: %s", k, first)
		}
		if idx <= prev {
			t.Errorf("attrs not in sorted order: %q (expected order: %v) in %s", k, expectedKeysInOrder, first)
		}
		prev = idx
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestDescribeSnapshotFiltersAttrsDeterministic(t *testing.T) {
	attrs := map[string]string{
		"area":   "backend",
		"owner":  "alice",
		"sprint": "42",
		"team":   "core",
	}
	first := describeSnapshotFilters("", models.WorkflowStateSet{}, false, 0, nil, 0, false, false, false, false, attrs, 0, 0, 0)
	for range 50 {
		got := describeSnapshotFilters("", models.WorkflowStateSet{}, false, 0, nil, 0, false, false, false, false, attrs, 0, 0, 0)
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

func TestDescribeSnapshotFiltersIncludesGitHubIssue(t *testing.T) {
	got := describeSnapshotFilters("", models.WorkflowStateSet{}, false, 0, nil, 0, false, false, true, false, nil, 0, 0, 0)
	if !strings.Contains(got, "issue") {
		t.Fatalf("summary = %q, want issue marker", got)
	}
}

func TestDescribeSnapshotFiltersIncludesParked(t *testing.T) {
	got := describeSnapshotFilters("", models.WorkflowStateSet{}, false, 0, nil, 0, false, false, false, true, nil, 0, 0, 0)
	if !strings.Contains(got, "parked") {
		t.Fatalf("summary = %q, want parked marker", got)
	}
}

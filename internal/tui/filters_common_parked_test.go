package tui

import (
	"strings"
	"testing"
)

func TestCommonFilterParkedMatching(t *testing.T) {
	parkedTask := mustCommonFilterTask(t, "parked-task")
	parkedTask.ParkedNote = "waiting on review"
	plainTask := mustCommonFilterTask(t, "plain-task")

	filter := newFilterState()
	filter.Parked = true

	if !passesCommonFilters(parkedTask, &filter, nil, nil) {
		t.Fatal("parked task should pass the parked filter")
	}
	if passesCommonFilters(plainTask, &filter, nil, nil) {
		t.Fatal("plain task should not pass the parked filter")
	}
}

func TestCommonFilterParkedComposesWithOtherFilters(t *testing.T) {
	parkedHigh := mustCommonFilterTask(t, "parked-high")
	parkedHigh.ParkedNote = "resume deploy"
	parkedHigh.Priority = 1

	parkedLow := mustCommonFilterTask(t, "parked-low")
	parkedLow.ParkedNote = "resume docs"
	parkedLow.Priority = 4

	filter := newFilterState()
	filter.Parked = true
	filter.Priority = 1

	if !passesCommonFilters(parkedHigh, &filter, nil, nil) {
		t.Fatal("parked high-priority task should pass parked+priority filters")
	}
	if passesCommonFilters(parkedLow, &filter, nil, nil) {
		t.Fatal("parked low-priority task should fail the additional priority filter")
	}
}

func TestCommonFilterParkedBadgeCountAndClear(t *testing.T) {
	filter := newFilterState()
	filter.Parked = true

	if !hasActiveCommonFilters(&filter) {
		t.Fatal("parked filter should count as an active common filter")
	}
	if got := commonFilterParts(&filter); len(got) != 1 || got[0] != "parked" {
		t.Fatalf("commonFilterParts() = %v, want [parked]", got)
	}
	if got := renderCommonFilterIndicator(&filter); !strings.Contains(got, "parked") {
		t.Fatalf("renderCommonFilterIndicator() = %q, want to contain parked", got)
	}

	mixin := FilterMixin{filter: filter}
	if got := mixin.ActiveFilterCount(); got != 1 {
		t.Fatalf("FilterMixin.ActiveFilterCount() = %d, want 1", got)
	}

	copied := copyCommonFilterState(&filter)
	if !copied.Parked {
		t.Fatal("copyCommonFilterState should preserve Parked")
	}

	clearCommonFilters(&filter)
	if filter.Parked {
		t.Fatal("clearCommonFilters should clear Parked")
	}
	if hasActiveCommonFilters(&filter) {
		t.Fatal("cleared parked filter should not remain active")
	}
}

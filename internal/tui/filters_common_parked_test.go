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

	if !passesCommonFilters(parkedTask, &filter, "", nil) {
		t.Fatal("parked task should pass the parked filter")
	}
	if passesCommonFilters(plainTask, &filter, "", nil) {
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

	if !passesCommonFilters(parkedHigh, &filter, "", nil) {
		t.Fatal("parked high-priority task should pass parked+priority filters")
	}
	if passesCommonFilters(parkedLow, &filter, "", nil) {
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

func TestHandleCommonFilterPrefixParked(t *testing.T) {
	filter := newFilterState()

	result := handleCommonFilterPrefix("k", &filter, nil)
	if !result.Handled || !result.Changed {
		t.Fatalf("handleCommonFilterPrefix(k) = %+v, want handled changed result", result)
	}
	if !filter.Parked {
		t.Fatal("Fk should enable the parked filter")
	}

	result = handleCommonFilterPrefix("k", &filter, nil)
	if !result.Handled || !result.Changed {
		t.Fatalf("second handleCommonFilterPrefix(k) = %+v, want handled changed result", result)
	}
	if filter.Parked {
		t.Fatal("Fk should toggle the parked filter off")
	}
}

func TestCommonFilterPrefixHintsParked(t *testing.T) {
	filter := newFilterState()

	hint, ok := findPrefixHint(buildCommonFilterPrefixHints(&filter, true, true), "k")
	if !ok {
		t.Fatal("expected common filter hints to include Fk")
	}
	if hint.Description != "parked" || hint.Detail != "off" || hint.Active {
		t.Fatalf("inactive Fk hint = %+v, want parked/off/inactive", hint)
	}

	filter.Parked = true
	hint, ok = findPrefixHint(buildCommonFilterPrefixHints(&filter, true, true), "k")
	if !ok {
		t.Fatal("expected common filter hints to include Fk when active")
	}
	if hint.Description != "parked" || hint.Detail != "on" || !hint.Active {
		t.Fatalf("active Fk hint = %+v, want parked/on/active", hint)
	}
}

func TestProjectTreeFilterPrefixHintsParked(t *testing.T) {
	tree := &ProjectTreeModel{filter: NewFilterState()}

	hint, ok := findPrefixHint(tree.buildFilterPrefixHints(), "k")
	if !ok {
		t.Fatal("expected project tree filter hints to include Fk")
	}
	if hint.Description != "parked" || hint.Detail != "off" || hint.Active {
		t.Fatalf("inactive project tree Fk hint = %+v, want parked/off/inactive", hint)
	}

	tree.filter.Parked = true
	hint, ok = findPrefixHint(tree.buildFilterPrefixHints(), "k")
	if !ok {
		t.Fatal("expected project tree filter hints to include Fk when active")
	}
	if hint.Description != "parked" || hint.Detail != "on" || !hint.Active {
		t.Fatalf("active project tree Fk hint = %+v, want parked/on/active", hint)
	}
}

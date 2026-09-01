package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCommonFilterParts_TagsDeterministicOrder guards against a regression where
// commonFilterParts() iterated a map and produced a nondeterministic display
// order in the filter badge for the Tags filter. The chips are now the clauses'
// canonical sigil text, so the order is the query's own and one chip is one tag.
func TestCommonFilterParts_TagsDeterministicOrder(t *testing.T) {
	f := newFilterState()
	f.Tags["zebra"] = true
	f.Tags["alpha"] = true
	f.Tags["mango"] = true
	f.Tags["banana"] = true

	first := tagPartsFromCommonFilterParts(commonFilterParts(&f))
	want := []string{"#alpha", "#banana", "#mango", "#zebra"}
	if !slices.Equal(first, want) {
		t.Errorf("tag parts not in deterministic sorted order: got %v, want %v", first, want)
	}

	// Repeat many times to detect map-iteration nondeterminism.
	for i := range 50 {
		got := tagPartsFromCommonFilterParts(commonFilterParts(&f))
		if !slices.Equal(got, first) {
			t.Fatalf("iteration %d: tag parts changed across calls: got %v, want %v", i, got, first)
		}
	}
}

// TestCommonFilterParts_StaleKeepsCanonicalFacetOrder holds the badge to the
// canonical query order rather than a chrome order of its own: the chips are
// exactly the clauses the same narrowing is typed and saved as.
func TestCommonFilterParts_StaleKeepsCanonicalFacetOrder(t *testing.T) {
	f := newFilterState()
	f.Priority = models.PriorityMedium
	f.setStateFilter(testWorkflowStateSet(models.StateBacklog))
	f.setCompletionAgeFilter(CompletionAge30Days)
	f.Tags["focus"] = true
	f.DueDate = DueDateToday
	f.Hierarchy = HierarchyLeaf
	f.Stale = true
	f.Deferred = true
	f.Pinned = true

	want := []string{
		"!2+",
		"#focus",
		"@today",
		"$backlog",
		"*pin",
		"?stale",
		"?deferred",
		"?completed:30d",
		"?leaf",
	}
	if got := commonFilterParts(&f); !slices.Equal(got, want) {
		t.Fatalf("commonFilterParts() = %v, want %v", got, want)
	}
}

func tagPartsFromCommonFilterParts(parts []string) []string {
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, "#") {
			tags = append(tags, p)
		}
	}
	return tags
}

// TestOutlinerActiveFilterParts_StatesCanonicalOrder guards the canonical
// state-set display order used by the active filter chip. One state is one
// clause, so the set is spelled as sorted $state chips.
func TestOutlinerActiveFilterParts_StatesCanonicalOrder(t *testing.T) {
	app, project := newTestAppModelWithProject()
	ol := NewOutlinerModel(project, app.configManager, app.runtime)
	ol.filter.setStateFilter(testWorkflowStateSet(
		models.StateCanceled,
		models.StateBacklog,
		models.StateInbox,
		models.StateWaiting,
	))
	ol.adoptFilterCarriers()

	first := statePartsFromParts(ol.activeFilterParts())
	want := []string{"$backlog", "$canceled", "$inbox", "$waiting"}
	if !slices.Equal(first, want) {
		t.Errorf("state parts not in canonical order: got %v, want %v", first, want)
	}

	for i := range 50 {
		got := statePartsFromParts(ol.activeFilterParts())
		if !slices.Equal(got, first) {
			t.Fatalf("iteration %d: state parts changed across calls: got %v, want %v", i, got, first)
		}
	}
}

func statePartsFromParts(parts []string) []string {
	states := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, "$") {
			states = append(states, p)
		}
	}
	return states
}

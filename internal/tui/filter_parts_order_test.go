package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
)

// TestCommonFilterParts_TagsDeterministicOrder guards against a regression where
// CommonFilterParts() iterated a map and produced a nondeterministic display
// order in the filter badge for the Tags filter. The chips are now the clauses'
// canonical sigil text, so the order is the query's own and one chip is one tag.
func TestCommonFilterParts_TagsDeterministicOrder(t *testing.T) {
	f := query.NewCommonFilterState()
	f.Tags["zebra"] = true
	f.Tags["alpha"] = true
	f.Tags["mango"] = true
	f.Tags["banana"] = true

	first := tagPartsFromCommonFilterParts(query.CommonFilterParts(&f))
	want := []string{"#alpha", "#banana", "#mango", "#zebra"}
	if !slices.Equal(first, want) {
		t.Errorf("tag parts not in deterministic sorted order: got %v, want %v", first, want)
	}

	// Repeat many times to detect map-iteration nondeterminism.
	for i := range 50 {
		got := tagPartsFromCommonFilterParts(query.CommonFilterParts(&f))
		if !slices.Equal(got, first) {
			t.Fatalf("iteration %d: tag parts changed across calls: got %v, want %v", i, got, first)
		}
	}
}

// TestCommonFilterParts_StaleKeepsCanonicalFacetOrder holds the badge to the
// canonical query order rather than a chrome order of its own: the chips are
// exactly the clauses the same narrowing is typed and saved as.
func TestCommonFilterParts_StaleKeepsCanonicalFacetOrder(t *testing.T) {
	f := query.NewCommonFilterState()
	f.Priority = models.PriorityMedium
	f.SetStateFilter(testWorkflowStateSet(models.StateBacklog))
	f.SetCompletionAgeFilter(query.CompletionAge30Days)
	f.Tags["focus"] = true
	f.DueDate = query.DueDateToday
	f.Hierarchy = query.HierarchyLeaf
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
	if got := query.CommonFilterParts(&f); !slices.Equal(got, want) {
		t.Fatalf("CommonFilterParts() = %v, want %v", got, want)
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
	ol := outliner.NewOutlinerModel(project, app.domain.manager, app.domain.runtime)
	ol.Filter.SetStateFilter(testWorkflowStateSet(
		models.StateCanceled,
		models.StateBacklog,
		models.StateInbox,
		models.StateWaiting,
	))
	adoptFilterCarriers(ol)

	first := statePartsFromParts(ol.ActiveFilterParts())
	want := []string{"$backlog", "$canceled", "$inbox", "$waiting"}
	if !slices.Equal(first, want) {
		t.Errorf("state parts not in canonical order: got %v, want %v", first, want)
	}

	for i := range 50 {
		got := statePartsFromParts(ol.ActiveFilterParts())
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

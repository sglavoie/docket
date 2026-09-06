package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// lensNarrowedIDs installs one lens query and returns the task order the mode
// projection kept. The rebuild is forced because a lens query changes no scope.
func lensNarrowedIDs(outlinerView *outliner.OutlinerModel, query string) []string {
	outlinerView.SetLensQuery(query)
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	return searchVisibleTaskIDs(outlinerView)
}

// lensSectionNames lists the section headers a projection emitted, which is how
// a test observes that an emptied section stopped being drawn.
func lensSectionNames(outlinerView *outliner.OutlinerModel) []string {
	names := make([]string, 0)
	for _, row := range outlinerView.Rows {
		if row.IsSectionHeader {
			names = append(names, row.SectionName)
		}
	}
	return names
}

// lensProjectHeaderIDs lists the project sub-headers a projection emitted.
func lensProjectHeaderIDs(outlinerView *outliner.OutlinerModel) []string {
	ids := make([]string, 0)
	for _, row := range outlinerView.Rows {
		if row.IsProjectHeader {
			ids = append(ids, row.ProjectID)
		}
	}
	return ids
}

// inBaselineOrder projects a wanted set onto the order the unnarrowed mode
// produced, so an ordering assertion states "the saved order, minus the rows
// the query dropped" rather than restating a hand-copied sequence.
func inBaselineOrder(baseline []string, want ...string) []string {
	wanted := make(map[string]bool, len(want))
	for _, id := range want {
		wanted[id] = true
	}
	result := make([]string, 0, len(want))
	for _, id := range baseline {
		if wanted[id] {
			result = append(result, id)
		}
	}
	return result
}

// lensProjectionOutliner builds one two-project scope whose hierarchy, sections,
// and field placement exercise every narrowing rule at once.
func lensProjectionOutliner() *outliner.OutlinerModel {
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Title = "Roadmap groundwork"
	child := primaryRowsTask("child", models.StateBacklog)
	child.Title = "Needle in the tree"
	child.ParentID = &parent.ID
	described := primaryRowsTask("described", models.StateBacklog)
	described.Title = "Nothing obvious"
	described.Description = "buried needle in the body"
	tagged := primaryRowsTask("tagged", models.StateBacklog)
	tagged.Title = "Tagged only"
	tagged.Tags = []string{"backend"}
	tagged.Priority = models.PriorityHigh
	doing := primaryRowsTask("doing", models.StateDoing)
	doing.Title = "Unrelated in flight"
	alpha := primaryRowsProject("alpha", parent, child, described, tagged, doing)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta needle"
	beta := primaryRowsProject("beta", betaTask)

	return scopeTestOutliner(alpha, beta)
}

func TestLensQueryNarrowsTheModeProjectionAndRetainsAncestorContext(t *testing.T) {
	outlinerView := lensProjectionOutliner()
	baseline := searchVisibleTaskIDs(outlinerView)
	for _, id := range []string{"parent", "child", "described", "tagged", "doing", "beta-task"} {
		if !slices.Contains(baseline, id) {
			t.Fatalf("baseline projection %v is missing %s", baseline, id)
		}
	}
	baselineSections := lensSectionNames(outlinerView)

	got := lensNarrowedIDs(outlinerView, "needle")
	want := inBaselineOrder(baseline, "parent", "child", "described", "beta-task")
	if !slices.Equal(got, want) {
		t.Fatalf("lens narrowing = %v, want the matches plus retained ancestry in saved order %v", got, want)
	}

	sections := lensSectionNames(outlinerView)
	for _, name := range sections {
		if !slices.Contains(baselineSections, name) {
			t.Fatalf("narrowed sections %v invented %q", sections, name)
		}
	}
	if len(sections) >= len(baselineSections) {
		t.Fatalf("narrowed sections %v kept every baseline section %v", sections, baselineSections)
	}
	if headers := lensProjectHeaderIDs(outlinerView); slices.Contains(headers, "gamma") {
		t.Fatalf("project headers %v named a project with no retained row", headers)
	}
}

func TestLensQueryPreservesSavedOrderRatherThanRelevanceOrder(t *testing.T) {
	outlinerView := lensProjectionOutliner()
	baseline := searchVisibleTaskIDs(outlinerView)

	got := lensNarrowedIDs(outlinerView, "needle")
	if !slices.Equal(got, inBaselineOrder(baseline, got...)) {
		t.Fatalf("lens order %v is not the saved order it narrowed", got)
	}

	// The flat ranked list over the same words ranks a title hit above a body
	// hit, which is exactly the ordering a lens must not adopt.
	relevance := scopeTestOutliner(outlinerView.Population.Projects...)
	searchRankedList(relevance, "needle")
	ranked := searchVisibleTaskIDs(relevance)
	if slices.Equal(ranked, got) {
		t.Fatalf("lens order %v matched relevance order %v, so the ordering claim proves nothing", got, ranked)
	}
	if !slices.Contains(ranked, "described") || ranked[len(ranked)-1] != "described" {
		t.Fatalf("relevance order = %v, want the description hit ranked last", ranked)
	}
}

func TestLensQueryAttachesMatchCuesToMatchesAlone(t *testing.T) {
	outlinerView := lensProjectionOutliner()
	if got := lensNarrowedIDs(outlinerView, "needle"); len(got) == 0 {
		t.Fatal("lens narrowing dropped every row")
	}

	sources := make(map[string]outliner.MatchSource)
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil {
			sources[row.Task.ID] = row.MatchSource
		}
	}
	if sources["child"] != outliner.MatchSourceTitle {
		t.Fatalf("title match cue = %v, want matchSourceTitle", sources["child"])
	}
	if sources["beta-task"] != outliner.MatchSourceTitle {
		t.Fatalf("cross-project match cue = %v, want matchSourceTitle", sources["beta-task"])
	}
	if sources["described"] != outliner.MatchSourceDescription {
		t.Fatalf("body match cue = %v, want matchSourceDescription", sources["described"])
	}
	if sources["parent"] != outliner.MatchSourceNone {
		t.Fatalf("retained ancestor cue = %v, want no cue it did not earn", sources["parent"])
	}
}

func TestLensQueryGatesWithTheCaptureSigils(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"priority", "!1", []string{"tagged"}},
		{"tag prefix", "#back", []string{"tagged"}},
		{"project", "%bet", []string{"beta-task"}},
		{"destination", "$doing", []string{"doing"}},
		{"sigil and free text", "%alpha needle", []string{"parent", "child", "described"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outlinerView := lensProjectionOutliner()
			baseline := searchVisibleTaskIDs(outlinerView)
			got := lensNarrowedIDs(outlinerView, tt.query)
			want := inBaselineOrder(baseline, tt.want...)
			if !slices.Equal(got, want) {
				t.Fatalf("lens %q = %v, want %v", tt.query, got, want)
			}
		})
	}
}

func TestLensQueryAppliesSmartCaseAndTheRelaxedRetry(t *testing.T) {
	outlinerView := lensProjectionOutliner()

	if got := lensNarrowedIDs(outlinerView, "needle"); len(got) == 0 {
		t.Fatal("lowercase lens query matched nothing")
	}
	if outlinerView.Lens.NarrowRelaxed || outlinerView.LensCaseSensitive() {
		t.Fatalf("lowercase lens relaxed=%v sensitive=%v, want an insensitive first pass",
			outlinerView.Lens.NarrowRelaxed, outlinerView.LensCaseSensitive())
	}

	if got := lensNarrowedIDs(outlinerView, "Beta"); !slices.Equal(got, []string{"beta-task"}) {
		t.Fatalf("case-sensitive lens = %v, want the exact-case title", got)
	}
	if outlinerView.Lens.NarrowRelaxed || !outlinerView.LensCaseSensitive() {
		t.Fatalf("capitalized lens relaxed=%v sensitive=%v, want a case-sensitive pass",
			outlinerView.Lens.NarrowRelaxed, outlinerView.LensCaseSensitive())
	}

	if got := lensNarrowedIDs(outlinerView, "NEEDLE"); len(got) == 0 {
		t.Fatal("the case-relaxed retry admitted nothing")
	}
	if !outlinerView.Lens.NarrowRelaxed || outlinerView.LensCaseSensitive() {
		t.Fatalf("fallback lens relaxed=%v sensitive=%v, want a relaxed pass",
			outlinerView.Lens.NarrowRelaxed, outlinerView.LensCaseSensitive())
	}
	if outlinerView.LensTextQuery() != "NEEDLE" {
		t.Fatalf("lens remainder = %q, want the free text alone", outlinerView.LensTextQuery())
	}

	if got := lensNarrowedIDs(outlinerView, "#back needle"); len(got) != 0 {
		t.Fatalf("gated lens = %v, want the AND gate to admit nothing", got)
	}
	if outlinerView.LensTextQuery() != "needle" {
		t.Fatalf("gated remainder = %q, want the sigil lifted out", outlinerView.LensTextQuery())
	}
}

func TestLensQueryKeepsTheRegistryGuidanceForARefusedValue(t *testing.T) {
	outlinerView := lensProjectionOutliner()
	for _, query := range []string{"!9", "$ready", "##0"} {
		if got := lensNarrowedIDs(outlinerView, query); len(got) != 0 {
			t.Fatalf("refused lens %q kept rows %v", query, got)
		}
		if outlinerView.LensGuidanceText() == "" {
			t.Fatalf("refused lens %q produced no guidance", query)
		}
	}
	if got := lensNarrowedIDs(outlinerView, "!9"); len(got) != 0 {
		t.Fatalf("refused lens kept rows %v", got)
	}
	want := textgrammar.InvalidPriorityError("9").Error()
	if got := outlinerView.LensGuidanceText(); got != want {
		t.Fatalf("lens guidance = %q, want the registry sentence %q", got, want)
	}

	if got := lensNarrowedIDs(outlinerView, ""); len(got) == 0 {
		t.Fatal("clearing the lens query left the projection empty")
	}
	if outlinerView.LensGuidanceText() != "" {
		t.Fatal("a cleared lens query kept its refusal guidance")
	}
}

// TestSearchNarrowsTheModeAndTheRankedListLeavesIt is D7 stated over the same
// fixture the lens tests use: a query narrows the mode it was typed in, keeping
// that mode's lanes, its grouping, and its ancestor context, while the flat
// ranked list is what abandons them for relevance order.
func TestSearchNarrowsTheModeAndTheRankedListLeavesIt(t *testing.T) {
	outlinerView := lensProjectionOutliner()
	outlinerView.ApplyOpenOptions(outliner.OutlinerOpenOptions{Mode: chrome.ModeInbox, SetMode: true})
	outlinerView.Filter.Pinned = true
	adoptFilterCarriers(outlinerView)
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	if got := searchVisibleTaskIDs(outlinerView); len(got) != 0 {
		t.Fatalf("Inbox under a pinned facet = %v, want an empty mode projection", got)
	}

	// A query narrows what the mode already showed, so an empty mode stays empty:
	// the query is a clause, never a way out of the mode.
	outlinerView.SetSearchQuery("needle")
	if got := searchVisibleTaskIDs(outlinerView); len(got) != 0 {
		t.Fatalf("a query under an empty mode = %v, want the mode still deciding", got)
	}

	// The facet is an AND gate over the query too, so it is cleared before the
	// mode is widened to one that holds the matches.
	outlinerView.Filter.Pinned = false
	adoptFilterCarriers(outlinerView)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	got := searchVisibleTaskIDs(outlinerView)
	for _, id := range []string{"child", "described", "beta-task"} {
		if !slices.Contains(got, id) {
			t.Fatalf("narrowed All = %v, want %s admitted by the query", got, id)
		}
	}
	if !slices.Contains(got, "parent") {
		t.Fatalf("narrowed All = %v, want the matching child's ancestor retained as context", got)
	}

	// The same query, presented flat, drops the retained ancestor and adopts
	// relevance order with the body hit last.
	outlinerView.ToggleRankedList()
	ranked := searchVisibleTaskIDs(outlinerView)
	if slices.Contains(ranked, "parent") {
		t.Fatalf("ranked list = %v, want no hierarchy context in a flat list", ranked)
	}
	if len(ranked) == 0 || ranked[len(ranked)-1] != "described" {
		t.Fatalf("ranked list = %v, want relevance ordering with the body hit last", ranked)
	}

	// Re-arming the facet while the flat list stands gates the same result set:
	// no fixture task is pinned, so the AND leaves nothing.
	outlinerView.Filter.Pinned = true
	adoptFilterCarriers(outlinerView)
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	if got := searchVisibleTaskIDs(outlinerView); len(got) != 0 {
		t.Fatalf("ranked list under the pinned facet = %v, want the facet to gate it", got)
	}
	outlinerView.Filter.Pinned = false
	adoptFilterCarriers(outlinerView)

	// Leaving the frame drops the flat presentation with the query that fed it.
	outlinerView.ClearSearch()
	if outlinerView.ShowsRankedList() {
		t.Fatal("clearing the query left the flat ranked list standing")
	}
}

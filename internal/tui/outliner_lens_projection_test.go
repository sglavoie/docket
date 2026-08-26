package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// lensNarrowedIDs installs one lens query and returns the task order the mode
// projection kept. The rebuild is forced because a lens query changes no scope.
func lensNarrowedIDs(outliner *OutlinerModel, query string) []string {
	outliner.setLensQuery(query)
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	return searchVisibleTaskIDs(outliner)
}

// lensSectionNames lists the section headers a projection emitted, which is how
// a test observes that an emptied section stopped being drawn.
func lensSectionNames(outliner *OutlinerModel) []string {
	names := make([]string, 0)
	for _, row := range outliner.rows {
		if row.IsSectionHeader {
			names = append(names, row.SectionName)
		}
	}
	return names
}

// lensProjectHeaderIDs lists the project sub-headers a projection emitted.
func lensProjectHeaderIDs(outliner *OutlinerModel) []string {
	ids := make([]string, 0)
	for _, row := range outliner.rows {
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
func lensProjectionOutliner() *OutlinerModel {
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
	outliner := lensProjectionOutliner()
	baseline := searchVisibleTaskIDs(outliner)
	for _, id := range []string{"parent", "child", "described", "tagged", "doing", "beta-task"} {
		if !slices.Contains(baseline, id) {
			t.Fatalf("baseline projection %v is missing %s", baseline, id)
		}
	}
	baselineSections := lensSectionNames(outliner)

	got := lensNarrowedIDs(outliner, "needle")
	want := inBaselineOrder(baseline, "parent", "child", "described", "beta-task")
	if !slices.Equal(got, want) {
		t.Fatalf("lens narrowing = %v, want the matches plus retained ancestry in saved order %v", got, want)
	}

	sections := lensSectionNames(outliner)
	for _, name := range sections {
		if !slices.Contains(baselineSections, name) {
			t.Fatalf("narrowed sections %v invented %q", sections, name)
		}
	}
	if len(sections) >= len(baselineSections) {
		t.Fatalf("narrowed sections %v kept every baseline section %v", sections, baselineSections)
	}
	if headers := lensProjectHeaderIDs(outliner); slices.Contains(headers, "gamma") {
		t.Fatalf("project headers %v named a project with no retained row", headers)
	}
}

func TestLensQueryPreservesSavedOrderRatherThanRelevanceOrder(t *testing.T) {
	outliner := lensProjectionOutliner()
	baseline := searchVisibleTaskIDs(outliner)

	got := lensNarrowedIDs(outliner, "needle")
	if !slices.Equal(got, inBaselineOrder(baseline, got...)) {
		t.Fatalf("lens order %v is not the saved order it narrowed", got)
	}

	// Ordinary search over the same words ranks a title hit above a body hit,
	// which is exactly the ordering a lens must not adopt.
	relevance := scopeTestOutliner(outliner.loadedProjects...)
	relevance.SetSearchQuery("needle")
	ranked := searchVisibleTaskIDs(relevance)
	if slices.Equal(ranked, got) {
		t.Fatalf("lens order %v matched relevance order %v, so the ordering claim proves nothing", got, ranked)
	}
	if !slices.Contains(ranked, "described") || ranked[len(ranked)-1] != "described" {
		t.Fatalf("relevance order = %v, want the description hit ranked last", ranked)
	}
}

func TestLensQueryAttachesMatchCuesToMatchesAlone(t *testing.T) {
	outliner := lensProjectionOutliner()
	if got := lensNarrowedIDs(outliner, "needle"); len(got) == 0 {
		t.Fatal("lens narrowing dropped every row")
	}

	sources := make(map[string]outlinerMatchSource)
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil {
			sources[row.task.ID] = row.MatchSource
		}
	}
	if sources["child"] != matchSourceTitle {
		t.Fatalf("title match cue = %v, want matchSourceTitle", sources["child"])
	}
	if sources["beta-task"] != matchSourceTitle {
		t.Fatalf("cross-project match cue = %v, want matchSourceTitle", sources["beta-task"])
	}
	if sources["described"] != matchSourceDescription {
		t.Fatalf("body match cue = %v, want matchSourceDescription", sources["described"])
	}
	if sources["parent"] != matchSourceNone {
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
			outliner := lensProjectionOutliner()
			baseline := searchVisibleTaskIDs(outliner)
			got := lensNarrowedIDs(outliner, tt.query)
			want := inBaselineOrder(baseline, tt.want...)
			if !slices.Equal(got, want) {
				t.Fatalf("lens %q = %v, want %v", tt.query, got, want)
			}
		})
	}
}

func TestLensQueryAppliesSmartCaseAndTheRelaxedRetry(t *testing.T) {
	outliner := lensProjectionOutliner()

	if got := lensNarrowedIDs(outliner, "needle"); len(got) == 0 {
		t.Fatal("lowercase lens query matched nothing")
	}
	if outliner.lensNarrowRelaxed || outliner.lensCaseSensitive() {
		t.Fatalf("lowercase lens relaxed=%v sensitive=%v, want an insensitive first pass",
			outliner.lensNarrowRelaxed, outliner.lensCaseSensitive())
	}

	if got := lensNarrowedIDs(outliner, "Beta"); !slices.Equal(got, []string{"beta-task"}) {
		t.Fatalf("case-sensitive lens = %v, want the exact-case title", got)
	}
	if outliner.lensNarrowRelaxed || !outliner.lensCaseSensitive() {
		t.Fatalf("capitalized lens relaxed=%v sensitive=%v, want a case-sensitive pass",
			outliner.lensNarrowRelaxed, outliner.lensCaseSensitive())
	}

	if got := lensNarrowedIDs(outliner, "NEEDLE"); len(got) == 0 {
		t.Fatal("the case-relaxed retry admitted nothing")
	}
	if !outliner.lensNarrowRelaxed || outliner.lensCaseSensitive() {
		t.Fatalf("fallback lens relaxed=%v sensitive=%v, want a relaxed pass",
			outliner.lensNarrowRelaxed, outliner.lensCaseSensitive())
	}
	if outliner.lensTextQuery() != "NEEDLE" {
		t.Fatalf("lens remainder = %q, want the free text alone", outliner.lensTextQuery())
	}

	if got := lensNarrowedIDs(outliner, "#back needle"); len(got) != 0 {
		t.Fatalf("gated lens = %v, want the AND gate to admit nothing", got)
	}
	if outliner.lensTextQuery() != "needle" {
		t.Fatalf("gated remainder = %q, want the sigil lifted out", outliner.lensTextQuery())
	}
}

func TestLensQueryKeepsTheRegistryGuidanceForARefusedValue(t *testing.T) {
	outliner := lensProjectionOutliner()
	for _, query := range []string{"!9", "$ready", "##0"} {
		if got := lensNarrowedIDs(outliner, query); len(got) != 0 {
			t.Fatalf("refused lens %q kept rows %v", query, got)
		}
		if outliner.lensGuidanceText() == "" {
			t.Fatalf("refused lens %q produced no guidance", query)
		}
	}
	if got := lensNarrowedIDs(outliner, "!9"); len(got) != 0 {
		t.Fatalf("refused lens kept rows %v", got)
	}
	want := textgrammar.InvalidPriorityError("9").Error()
	if got := outliner.lensGuidanceText(); got != want {
		t.Fatalf("lens guidance = %q, want the registry sentence %q", got, want)
	}

	if got := lensNarrowedIDs(outliner, ""); len(got) == 0 {
		t.Fatal("clearing the lens query left the projection empty")
	}
	if outliner.lensGuidanceText() != "" {
		t.Fatal("a cleared lens query kept its refusal guidance")
	}
}

func TestOrdinarySearchIgnoresTheModeFacetsAndTheLensQuery(t *testing.T) {
	outliner := lensProjectionOutliner()
	outliner.ApplyOpenOptions(OutlinerOpenOptions{Mode: ModeInbox, SetMode: true})
	outliner.setLensQuery("%beta")
	outliner.filter.Pinned = true
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("Inbox under a lens and a priority facet = %v, want an empty mode projection", got)
	}

	outliner.SetSearchQuery("needle")
	got := searchVisibleTaskIDs(outliner)
	for _, id := range []string{"child", "described", "beta-task"} {
		if !slices.Contains(got, id) {
			t.Fatalf("scope-wide search = %v, want %s despite the mode, facet, and lens query", got, id)
		}
	}
	if slices.Contains(got, "parent") {
		t.Fatalf("scope-wide search = %v, want no lens-style ancestor retention", got)
	}
	if got[len(got)-1] != "described" {
		t.Fatalf("scope-wide search = %v, want relevance ordering with the body hit last", got)
	}

	outliner.ClearSearch()
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("after Esc = %v, want the lens-narrowed mode projection restored unchanged", got)
	}
}

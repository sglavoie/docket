package tui

import (
	"maps"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// projectGroupToggleFixture seeds alpha with two tasks and beta with one, in a
// mode that renders a project group header for each. It covers the modes
// newGroupingFixture declines, so between them every grouping surface has a
// fixture and the mode table below can name all seven.
func projectGroupToggleFixture(t *testing.T, mode OutlinerModeID) (*OutlinerModel, []string) {
	t.Helper()
	switch mode {
	case ModeAll, ModeToday, ModeNext:
		alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
		alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
		betaOne := primaryRowsTask("beta-one", models.StateBacklog)
		alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
		beta := primaryRowsProject("beta", betaOne)
		outliner, runtime := primaryRowsOutliner(alpha, beta)
		plan := []config.ProjectTaskLocator{
			{ProjectID: alpha.ID, TaskID: alphaOne.ID},
			{ProjectID: alpha.ID, TaskID: alphaTwo.ID},
			{ProjectID: beta.ID, TaskID: betaOne.ID},
		}
		switch mode {
		case ModeToday:
			runtime.Planning().SetToday(plan)
		case ModeNext:
			runtime.Planning().SetNext(plan)
		default:
			// All groups by state, so it needs no planning membership.
		}
		outliner.selectOutlinerMode(mode)
		return outliner, []string{alphaOne.ID, alphaTwo.ID, betaOne.ID}
	default:
		fixture := newGroupingFixture(t, mode)
		outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
		outliner.selectOutlinerMode(mode)
		return outliner, fixture.tasks
	}
}

// sectionFoldStates records the collapse flag of every section header, so a
// project-axis press can be shown to have written none of them.
func sectionFoldStates(m *OutlinerModel) map[string]bool {
	states := make(map[string]bool)
	for _, row := range m.visibleSectionHeaders() {
		states[row.effectiveCollapseKey()] = m.isSectionRowCollapsed(row)
	}
	return states
}

// One press folds every project group and the next puts them back. The headers
// themselves keep rendering throughout: folding a group hides its tasks, not the
// row naming the project the tasks belong to.
func TestToggleVisibleProjectGroupsFoldsEveryHeaderAndRestoresThem(t *testing.T) {
	outliner, tasks := projectGroupToggleFixture(t, ModeAll)

	headers, _ := projectGroupHeaders(outliner)
	if len(headers) != 2 {
		t.Fatalf("seeded project headers = %v, want one per project", headers)
	}

	if !outliner.toggleVisibleProjectGroups() {
		t.Fatal("toggleVisibleProjectGroups reported no change on all-expanded groups")
	}
	for _, row := range outliner.visibleProjectGroupHeaders() {
		if !outliner.isSectionRowCollapsed(row) {
			t.Errorf("project group %q did not fold", row.ProjectID)
		}
	}
	if got := visibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("folded rows = %v, want no task rows", got)
	}
	if folded, _ := projectGroupHeaders(outliner); !slices.Equal(folded, headers) {
		t.Fatalf("folded project headers = %v, want the same headers still rendered", folded)
	}

	if !outliner.toggleVisibleProjectGroups() {
		t.Fatal("the second press reported no change on folded groups")
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, tasks) {
		t.Fatalf("restored rows = %v, want %v", got, tasks)
	}
}

// The direction rule is any, not all: one folded group makes the next press
// unfold rather than fold the remaining groups to match it.
func TestToggleVisibleProjectGroupsUnfoldsWhenAnySingleGroupIsFolded(t *testing.T) {
	outliner, tasks := projectGroupToggleFixture(t, ModeAll)

	first := outliner.visibleProjectGroupHeaders()[0]
	outliner.setSectionRowCollapsed(first, true)
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); len(got) == 0 || len(got) == len(tasks) {
		t.Fatalf("hand-folded rows = %v, want a genuinely mixed state", got)
	}

	if !outliner.toggleVisibleProjectGroups() {
		t.Fatal("a mixed set of groups reported no change")
	}
	for _, row := range outliner.visibleProjectGroupHeaders() {
		if outliner.isSectionRowCollapsed(row) {
			t.Errorf("project group %q stayed folded", row.ProjectID)
		}
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, tasks) {
		t.Fatalf("rows after the press = %v, want %v", got, tasks)
	}
}

// p carries no zoom refusal, deliberately: a zoom overrides only the one section
// key it admits, and that admitted section still emits its project headers. So
// unlike s, p there folds rows that are genuinely on screen.
func TestToggleVisibleProjectGroupsFoldsInsideASectionZoom(t *testing.T) {
	outliner, _ := projectGroupToggleFixture(t, ModeAll)

	headers := outliner.visibleSectionHeaders()
	if len(headers) == 0 {
		t.Fatal("All mode emitted no section headers to zoom into")
	}
	if !outliner.enterSectionZoom(headers[0]) {
		t.Fatal("the section header refused a zoom")
	}
	zoomed := visibleTaskIDs(outliner)
	if len(zoomed) == 0 {
		t.Fatal("the zoomed section rendered no tasks")
	}
	if groups := outliner.visibleProjectGroupHeaders(); len(groups) == 0 {
		t.Fatal("the zoomed section rendered no project headers")
	}

	if !outliner.toggleVisibleProjectGroups() {
		t.Fatal("p refused inside a section zoom")
	}
	if got := visibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("zoomed folded rows = %v, want no task rows", got)
	}

	if !outliner.toggleVisibleProjectGroups() {
		t.Fatal("the second zoomed press reported no change")
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, zoomed) {
		t.Fatalf("zoomed restored rows = %v, want %v", got, zoomed)
	}
}

// Both header kinds live in collapsedSections and are told apart by scanned row
// kind, never by a key prefix. This is the test that would fail if that scan
// were replaced by string matching over a shared map.
func TestToggleVisibleProjectGroupsLeavesTheOtherFoldAxesAlone(t *testing.T) {
	// Two states, so All emits two sections: one gets folded by hand and has to
	// stay folded, the other stays expanded and has to stay expanded.
	doing := primaryRowsTask("alpha-doing", models.StateDoing)
	parent := primaryRowsTask("alpha-parent", models.StateBacklog)
	child := primaryRowsTask("alpha-child", models.StateBacklog)
	child.ParentID = &parent.ID
	betaOne := primaryRowsTask("beta-one", models.StateBacklog)
	alpha := primaryRowsProject("alpha", doing, parent, child)
	outliner, _ := primaryRowsOutliner(alpha, primaryRowsProject("beta", betaOne))
	outliner.selectOutlinerMode(ModeAll)

	sections := outliner.visibleSectionHeaders()
	if len(sections) < 2 {
		t.Fatalf("section header count = %d, want a folded one and an expanded one", len(sections))
	}
	outliner.setSectionRowCollapsed(sections[0], true)
	outliner.rebuildRows()

	outliner.expandedNodes[TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}] = false
	outliner.rebuildRows()
	sectionsBefore := sectionFoldStates(outliner)
	nodesBefore := maps.Clone(outliner.expandedNodes)

	outliner.toggleVisibleProjectGroups()
	outliner.toggleVisibleProjectGroups()

	if got := sectionFoldStates(outliner); !maps.Equal(got, sectionsBefore) {
		t.Errorf("the project toggle rewrote sections: got %v, want %v", got, sectionsBefore)
	}
	if !maps.Equal(outliner.expandedNodes, nodesBefore) {
		t.Errorf("the project toggle rewrote task folds: got %v, want %v", outliner.expandedNodes, nodesBefore)
	}
}

// Inside a project scope the chrome already names the project, so no group
// header is emitted and p has nothing to fold. Its refusal names P, which is
// exactly the key this scope was entered with.
func TestToggleVisibleProjectGroupsReportsInsideAProjectScope(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "task")
	app.projects[project.ID] = project
	outliner := app.crossOutliner
	outliner.loadedProjects = []*models.Project{project}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	outliner.pushProjectScope(project.ID)
	if groups := outliner.visibleProjectGroupHeaders(); len(groups) != 0 {
		t.Fatalf("project scope rendered %d redundant group headers", len(groups))
	}
	foldsBefore := maps.Clone(outliner.collapsedSections)

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('p'))
	if !handled || cmd == nil {
		t.Fatalf("scoped p = handled %v, cmd %v; want handled with a status", handled, cmd)
	}
	if status, ok := cmd().(StatusMsg); !ok || string(status) != "No project groups here; P switches project" {
		t.Fatalf("scoped p message = %#v, want the refusal that names P", cmd())
	}
	if !maps.Equal(outliner.collapsedSections, foldsBefore) {
		t.Fatalf("scoped p wrote a fold: got %v, want %v", outliner.collapsedSections, foldsBefore)
	}
}

// Every mode reaches its project headers from one press. Today, Next, Inbox, and
// Backlog carry the header at top level; All, Due, and Recent nest it under a
// section. The scan over row kind is what makes those five collapse-key
// namespaces one operation instead of five.
func TestToggleVisibleProjectGroupsReachesEveryGroupingNamespace(t *testing.T) {
	for _, mode := range []OutlinerModeID{
		ModeToday, ModeNext, ModeInbox, ModeBacklog, ModeAll, ModeDue, ModeRecent,
	} {
		t.Run(string(mode), func(t *testing.T) {
			outliner, tasks := projectGroupToggleFixture(t, mode)
			if groups := outliner.visibleProjectGroupHeaders(); len(groups) != 2 {
				t.Fatalf("%s project headers = %d, want one per project", mode, len(groups))
			}

			if !outliner.toggleVisibleProjectGroups() {
				t.Fatalf("%s reported no change on all-expanded groups", mode)
			}
			if got := visibleTaskIDs(outliner); len(got) != 0 {
				t.Fatalf("%s folded rows = %v, want no task rows", mode, got)
			}

			if !outliner.toggleVisibleProjectGroups() {
				t.Fatalf("%s reported no change on folded groups", mode)
			}
			if got := visibleTaskIDs(outliner); !slices.Equal(got, tasks) {
				t.Fatalf("%s restored rows = %v, want %v", mode, got, tasks)
			}
		})
	}
}

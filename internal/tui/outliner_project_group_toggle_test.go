package tui

import (
	"maps"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// projectGroupToggleFixture seeds alpha with two tasks and beta with one, in a
// mode that renders a project group header for each. It covers the modes
// newGroupingFixture declines, so between them every grouping surface has a
// fixture and the mode table below can name all seven.
func projectGroupToggleFixture(t *testing.T, mode chrome.OutlinerModeID) (*outliner.OutlinerModel, []string) {
	t.Helper()
	switch mode {
	case chrome.ModeAll, chrome.ModeToday, chrome.ModeNext:
		alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
		alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
		betaOne := primaryRowsTask("beta-one", models.StateBacklog)
		alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
		beta := primaryRowsProject("beta", betaOne)
		outlinerView, runtime := primaryRowsOutliner(alpha, beta)
		plan := []config.ProjectTaskLocator{
			{ProjectID: alpha.ID, TaskID: alphaOne.ID},
			{ProjectID: alpha.ID, TaskID: alphaTwo.ID},
			{ProjectID: beta.ID, TaskID: betaOne.ID},
		}
		switch mode {
		case chrome.ModeToday:
			runtime.Planning().SetToday(plan)
		case chrome.ModeNext:
			runtime.Planning().SetNext(plan)
		default:
			// All groups by state, so it needs no planning membership.
		}
		outlinerView.SelectOutlinerMode(mode)
		return outlinerView, []string{alphaOne.ID, alphaTwo.ID, betaOne.ID}
	default:
		fixture := newGroupingFixture(t, mode)
		outlinerView, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
		outlinerView.SelectOutlinerMode(mode)
		return outlinerView, fixture.tasks
	}
}

// sectionFoldStates records the collapse flag of every section header, so a
// project-axis press can be shown to have written none of them.
func sectionFoldStates(m *outliner.OutlinerModel) map[string]bool {
	states := make(map[string]bool)
	for _, row := range m.VisibleSectionHeaders() {
		states[row.EffectiveCollapseKey()] = m.IsSectionRowCollapsed(row)
	}
	return states
}

// One press folds every project group and the next puts them back. The headers
// themselves keep rendering throughout: folding a group hides its tasks, not the
// row naming the project the tasks belong to.
func TestToggleVisibleProjectGroupsFoldsEveryHeaderAndRestoresThem(t *testing.T) {
	outlinerView, tasks := projectGroupToggleFixture(t, chrome.ModeAll)

	headers, _ := projectGroupHeaders(outlinerView)
	if len(headers) != 2 {
		t.Fatalf("seeded project headers = %v, want one per project", headers)
	}

	if !outlinerView.ToggleVisibleProjectGroups() {
		t.Fatal("toggleVisibleProjectGroups reported no change on all-expanded groups")
	}
	for _, row := range outlinerView.VisibleProjectGroupHeaders() {
		if !outlinerView.IsSectionRowCollapsed(row) {
			t.Errorf("project group %q did not fold", row.ProjectID)
		}
	}
	if got := visibleTaskIDs(outlinerView); len(got) != 0 {
		t.Fatalf("folded rows = %v, want no task rows", got)
	}
	if folded, _ := projectGroupHeaders(outlinerView); !slices.Equal(folded, headers) {
		t.Fatalf("folded project headers = %v, want the same headers still rendered", folded)
	}

	if !outlinerView.ToggleVisibleProjectGroups() {
		t.Fatal("the second press reported no change on folded groups")
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, tasks) {
		t.Fatalf("restored rows = %v, want %v", got, tasks)
	}
}

// The direction rule is any, not all: one folded group makes the next press
// unfold rather than fold the remaining groups to match it.
func TestToggleVisibleProjectGroupsUnfoldsWhenAnySingleGroupIsFolded(t *testing.T) {
	outlinerView, tasks := projectGroupToggleFixture(t, chrome.ModeAll)

	first := outlinerView.VisibleProjectGroupHeaders()[0]
	outlinerView.SetSectionRowCollapsed(first, true)
	outlinerView.RebuildRows()
	if got := visibleTaskIDs(outlinerView); len(got) == 0 || len(got) == len(tasks) {
		t.Fatalf("hand-folded rows = %v, want a genuinely mixed state", got)
	}

	if !outlinerView.ToggleVisibleProjectGroups() {
		t.Fatal("a mixed set of groups reported no change")
	}
	for _, row := range outlinerView.VisibleProjectGroupHeaders() {
		if outlinerView.IsSectionRowCollapsed(row) {
			t.Errorf("project group %q stayed folded", row.ProjectID)
		}
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, tasks) {
		t.Fatalf("rows after the press = %v, want %v", got, tasks)
	}
}

// p carries no zoom refusal, deliberately: a zoom overrides only the one section
// key it admits, and that admitted section still emits its project headers. So
// unlike s, p there folds rows that are genuinely on screen.
func TestToggleVisibleProjectGroupsFoldsInsideASectionZoom(t *testing.T) {
	outlinerView, _ := projectGroupToggleFixture(t, chrome.ModeAll)

	headers := outlinerView.VisibleSectionHeaders()
	if len(headers) == 0 {
		t.Fatal("All mode emitted no section headers to zoom into")
	}
	if !outlinerView.EnterSectionZoom(headers[0]) {
		t.Fatal("the section header refused a zoom")
	}
	zoomed := visibleTaskIDs(outlinerView)
	if len(zoomed) == 0 {
		t.Fatal("the zoomed section rendered no tasks")
	}
	if groups := outlinerView.VisibleProjectGroupHeaders(); len(groups) == 0 {
		t.Fatal("the zoomed section rendered no project headers")
	}

	if !outlinerView.ToggleVisibleProjectGroups() {
		t.Fatal("p refused inside a section zoom")
	}
	if got := visibleTaskIDs(outlinerView); len(got) != 0 {
		t.Fatalf("zoomed folded rows = %v, want no task rows", got)
	}

	if !outlinerView.ToggleVisibleProjectGroups() {
		t.Fatal("the second zoomed press reported no change")
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, zoomed) {
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
	outlinerView, _ := primaryRowsOutliner(alpha, primaryRowsProject("beta", betaOne))
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	sections := outlinerView.VisibleSectionHeaders()
	if len(sections) < 2 {
		t.Fatalf("section header count = %d, want a folded one and an expanded one", len(sections))
	}
	outlinerView.SetSectionRowCollapsed(sections[0], true)
	outlinerView.RebuildRows()

	outlinerView.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}] = false
	outlinerView.RebuildRows()
	sectionsBefore := sectionFoldStates(outlinerView)
	nodesBefore := maps.Clone(outlinerView.Folds.ExpandedNodes)

	outlinerView.ToggleVisibleProjectGroups()
	outlinerView.ToggleVisibleProjectGroups()

	if got := sectionFoldStates(outlinerView); !maps.Equal(got, sectionsBefore) {
		t.Errorf("the project toggle rewrote sections: got %v, want %v", got, sectionsBefore)
	}
	if !maps.Equal(outlinerView.Folds.ExpandedNodes, nodesBefore) {
		t.Errorf("the project toggle rewrote task folds: got %v, want %v", outlinerView.Folds.ExpandedNodes, nodesBefore)
	}
}

// Inside a project scope the chrome already names the project, so no group
// header is emitted and p has nothing to fold. Its refusal names P, which is
// exactly the key this scope was entered with.
func TestToggleVisibleProjectGroupsReportsInsideAProjectScope(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "task")
	app.projects[project.ID] = project
	outlinerView := app.crossOutliner
	outlinerView.Population.Projects = []*models.Project{project}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	outlinerView.PushProjectScope(project.ID)
	if groups := outlinerView.VisibleProjectGroupHeaders(); len(groups) != 0 {
		t.Fatalf("project scope rendered %d redundant group headers", len(groups))
	}
	foldsBefore := maps.Clone(outlinerView.Folds.CollapsedSections)

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('p'))
	if !handled || cmd == nil {
		t.Fatalf("scoped p = handled %v, cmd %v; want handled with a status", handled, cmd)
	}
	if status, ok := cmd().(status.StatusMsg); !ok || string(status) != "No project groups here; P switches project" {
		t.Fatalf("scoped p message = %#v, want the refusal that names P", cmd())
	}
	if !maps.Equal(outlinerView.Folds.CollapsedSections, foldsBefore) {
		t.Fatalf("scoped p wrote a fold: got %v, want %v", outlinerView.Folds.CollapsedSections, foldsBefore)
	}
}

// Every mode reaches its project headers from one press. Today, Next, Inbox, and
// Backlog carry the header at top level; All, Due, and Recent nest it under a
// section. The scan over row kind is what makes those five collapse-key
// namespaces one operation instead of five.
func TestToggleVisibleProjectGroupsReachesEveryGroupingNamespace(t *testing.T) {
	for _, mode := range []chrome.OutlinerModeID{
		chrome.ModeToday, chrome.ModeNext, chrome.ModeInbox, chrome.ModeBacklog, chrome.ModeAll, chrome.ModeDue, chrome.ModeRecent,
	} {
		t.Run(string(mode), func(t *testing.T) {
			outlinerView, tasks := projectGroupToggleFixture(t, mode)
			if groups := outlinerView.VisibleProjectGroupHeaders(); len(groups) != 2 {
				t.Fatalf("%s project headers = %d, want one per project", mode, len(groups))
			}

			if !outlinerView.ToggleVisibleProjectGroups() {
				t.Fatalf("%s reported no change on all-expanded groups", mode)
			}
			if got := visibleTaskIDs(outlinerView); len(got) != 0 {
				t.Fatalf("%s folded rows = %v, want no task rows", mode, got)
			}

			if !outlinerView.ToggleVisibleProjectGroups() {
				t.Fatalf("%s reported no change on folded groups", mode)
			}
			if got := visibleTaskIDs(outlinerView); !slices.Equal(got, tasks) {
				t.Fatalf("%s restored rows = %v, want %v", mode, got, tasks)
			}
		})
	}
}

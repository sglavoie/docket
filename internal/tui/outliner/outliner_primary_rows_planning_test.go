package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestPlanningSectionsNestAtEarliestMemberRankWithoutMutatingDurableOrder(t *testing.T) {
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeToday, chrome.ModeNext} {
		t.Run(string(mode), func(t *testing.T) {
			parent := primaryRowsTask("parent", models.StateBacklog)
			child := primaryRowsTask("child", models.StateBacklog)
			child.ParentID = &parent.ID
			alpha := primaryRowsProject("alpha", parent, child)
			betaTask := primaryRowsTask("beta-task", models.StateBacklog)
			beta := primaryRowsProject("beta", betaTask)
			outliner, runtime := primaryRowsOutliner(alpha, beta)
			order := []config.ProjectTaskLocator{
				{ProjectID: alpha.ID, TaskID: child.ID},
				{ProjectID: beta.ID, TaskID: betaTask.ID},
				{ProjectID: alpha.ID, TaskID: parent.ID},
			}
			if mode == chrome.ModeToday {
				runtime.Planning().SetToday(order)
			} else {
				runtime.Planning().SetNext(order)
			}

			for _, renderedMode := range []chrome.OutlinerModeID{mode, chrome.ModeAll} {
				outliner.SelectOutlinerMode(renderedMode)
				if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{parent.ID, child.ID, betaTask.ID}) {
					t.Fatalf("%s rows = %v, want earliest-ranked Alpha subtree before Beta", renderedMode, got)
				}
				parentRow, parentFound := findPlanningTaskRow(outliner, parent.ID)
				childRow, childFound := findPlanningTaskRow(outliner, child.ID)
				if !parentFound || !childFound || !parentRow.HasVisibleChildren || childRow.Breadcrumb != "" {
					t.Fatalf("%s nested rows = parent %+v found=%t child %+v found=%t",
						renderedMode, parentRow, parentFound, childRow, childFound)
				}
			}

			var gotOrder []config.ProjectTaskLocator
			if mode == chrome.ModeToday {
				gotOrder = runtime.Planning().GetToday()
			} else {
				gotOrder = runtime.Planning().GetNext()
			}
			if !slices.Equal(gotOrder, order) {
				t.Fatalf("durable %s order = %v, want unchanged %v", mode, gotOrder, order)
			}
		})
	}
}

// TestPrimaryAllFocusedProjectLeadsWithOneHeader is the focused-scope round
// trip. Entering a project drops its redundant per-group headers and gains the
// one focused header above the content it still shows; leaving restores the
// cross-project grouping unchanged, which is the half this test has always
// locked.
func TestPrimaryAllFocusedProjectLeadsWithOneHeader(t *testing.T) {
	alpha := primaryRowsProject(
		"alpha",
		primaryRowsTask("alpha-progress", models.StateDoing),
		primaryRowsTask("alpha-ready", models.StateBacklog),
	)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	baseProjectHeaders := 0
	for _, row := range outliner.Rows {
		if row.IsProjectHeader {
			baseProjectHeaders++
		}
	}
	if baseProjectHeaders == 0 {
		t.Fatal("single-project base scope omitted cross-project sub-headers")
	}

	outliner.PushProjectScope(alpha.ID)
	assertSoleFocusedProjectHeader(t, outliner, alpha.ID, "all")
	// The push seats the cursor on the first row of content rather than on the
	// header it just inserted: the header is reachable above, not where a focus
	// lands when there is work to look at.
	if outliner.Cursor == 0 {
		t.Fatal("focusing a project with content parked the cursor on its header")
	}

	sectionCount := 0
	taskCount := 0
	for _, row := range outliner.Rows {
		switch {
		case row.IsProjectHeader && !row.IsFocusedProjectHeader:
			t.Fatalf("focused project rendered redundant project header: %+v", row)
		case row.IsProjectHeader:
		case row.IsSectionHeader:
			sectionCount++
		case row.IsTaskRow():
			taskCount++
			if row.ProjectID != alpha.ID {
				t.Errorf("focused task %q project ID = %q, want %q", row.Task.ID, row.ProjectID, alpha.ID)
			}
		}
	}
	if sectionCount != 2 || taskCount != 2 {
		t.Fatalf("focused rows contain %d sections and %d tasks, want 2 and 2", sectionCount, taskCount)
	}

	if !outliner.PopProjectScope() {
		t.Fatal("focused project scope did not pop")
	}
	projectHeaders := 0
	for _, row := range outliner.Rows {
		if row.IsProjectHeader {
			projectHeaders++
		}
	}
	if projectHeaders == 0 {
		t.Fatal("base scope did not restore cross-project sub-headers")
	}
}

// projectionDependencySource builds a two-level tree whose every task stores
// soft references, so a projection that shallow-copies a task without
// detaching Dependencies is observable as a shared backing array.
func projectionDependencySource() (roots []*models.Task, stored map[string][]models.TaskReference) {
	child := primaryRowsTask("child", models.StateBacklog)
	child.Dependencies = []models.TaskReference{
		{ProjectID: "proj", TaskID: "dep-a"},
		{ProjectID: "proj", TaskID: "dep-b"},
	}
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Dependencies = []models.TaskReference{
		{ProjectID: "proj", TaskID: "dep-c"},
		{ProjectID: "proj", TaskID: "dep-d"},
	}
	parent.Children = []*models.Task{child}
	stored = map[string][]models.TaskReference{
		"parent": models.CloneTaskReferences(parent.Dependencies),
		"child":  models.CloneTaskReferences(child.Dependencies),
	}
	return []*models.Task{parent}, stored
}

func flattenProjectedTasks(tasks []*models.Task) []*models.Task {
	var flat []*models.Task
	for _, task := range tasks {
		if task == nil {
			continue
		}
		flat = append(flat, task)
		flat = append(flat, flattenProjectedTasks(task.Children)...)
	}
	return flat
}

// The tree projections hand their clones to renderers and to further
// projections, so a clone that keeps the source's dependency backing array
// lets a later element write or in-capacity append reach repository-owned
// state. Every projection must detach the list.
func TestTaskProjectionCopiesDetachDependencies(t *testing.T) {
	suppressed := map[taskref.TaskIdentity]bool{{ProjectID: "proj", TaskID: "absent"}: true}

	cases := []struct {
		name    string
		project func([]*models.Task) []*models.Task
	}{
		{"filterTaskTreeKeepingAncestors", func(roots []*models.Task) []*models.Task {
			return filterTaskTreeKeepingAncestors(roots, func(*models.Task) bool { return true })
		}},
		{"removePlanningRowsFromTree", func(roots []*models.Task) []*models.Task {
			return removePlanningRowsFromTree(roots, "proj", suppressed)
		}},
		{"excludeTaskTreePromotingChildren", func(roots []*models.Task) []*models.Task {
			return excludeTaskTreePromotingChildren(roots, func(*models.Task) bool { return false })
		}},
		{"removeProjectedRowsFromTree", func(roots []*models.Task) []*models.Task {
			return removeProjectedRowsFromTree(roots, "proj", suppressed)
		}},
		{"sortPrimaryTaskTree", func(roots []*models.Task) []*models.Task {
			outliner, _ := primaryRowsOutliner()
			outliner.Sort = query.SortState{Field: query.SortByTitle, Ascending: true}
			return outliner.sortPrimaryTaskTree(roots)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roots, stored := projectionDependencySource()
			sources := flattenProjectedTasks(roots)

			isSource := make(map[*models.Task]bool, len(sources))
			for _, source := range sources {
				isSource[source] = true
			}

			projected := flattenProjectedTasks(tc.project(roots))
			if len(projected) == 0 {
				t.Fatal("projection dropped every task")
			}
			mutated := 0
			for _, clone := range projected {
				if isSource[clone] {
					// A projection may pass a task through untouched
					// (sortPrimaryTaskTree keeps leaves); only copies
					// carry the detachment obligation.
					continue
				}
				want := stored[clone.ID]
				if !models.TaskReferencesEqual(clone.Dependencies, want) {
					t.Fatalf("%s dependencies = %v, want %v", clone.ID, clone.Dependencies, want)
				}
				if len(clone.Dependencies) == 0 {
					continue
				}
				clone.Dependencies[0] = models.TaskReference{ProjectID: "proj", TaskID: "overwritten"}
				clone.Dependencies = append(clone.Dependencies[:1],
					models.TaskReference{ProjectID: "proj", TaskID: "appended"})
				mutated++
			}
			if mutated == 0 {
				t.Fatal("no projected clone carried dependencies to mutate")
			}
			for _, source := range sources {
				if !models.TaskReferencesEqual(source.Dependencies, stored[source.ID]) {
					t.Fatalf("source %s mutated through its projection: %v, want %v",
						source.ID, source.Dependencies, stored[source.ID])
				}
			}
		})
	}
}

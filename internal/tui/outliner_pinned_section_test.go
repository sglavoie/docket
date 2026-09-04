package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestPinnedSectionAppearsAbovePromotingModesAndDeduplicatesQualifiedTasks(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	older := now.Add(-time.Hour)
	newer := now.Add(-time.Minute)

	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("shared", models.StateBacklog)
	child.ParentID = &parent.ID
	child.Pinned, child.PinnedAt = true, &newer
	alphaOlder := primaryRowsTask("alpha-older", models.StateBacklog)
	alphaOlder.Pinned, alphaOlder.PinnedAt = true, &older
	due := primaryRowsTask("due", models.StateBacklog)
	due.DueDate = &now
	alpha := primaryRowsProject("alpha", parent, child, alphaOlder, due)
	alpha.Name, alpha.Slug = "Alpha", "alpha"

	sameID := primaryRowsTask("shared", models.StateInbox)
	sameID.Pinned, sameID.PinnedAt = true, &older
	next := primaryRowsTask("next", models.StateBacklog)
	beta := primaryRowsProject("beta", sameID, next)
	beta.Name, beta.Slug = "Beta", "beta"

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.population.projects = []*models.Project{beta, alpha}

	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeToday} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.selectOutlinerMode(mode)
			if len(outliner.rows) < 3 || !outliner.rows[0].IsSectionHeader ||
				!outliner.rows[0].IsPinnedSection || outliner.rows[0].SectionName != pinnedSectionName {
				t.Fatalf("first row = %+v, want Pinned section header", outliner.rows[0])
			}
			if outliner.rows[0].SectionCount != 3 {
				t.Fatalf("Pinned count = %d, want 3", outliner.rows[0].SectionCount)
			}

			var projectHeaders []outlinerRow
			var pinnedTasks []outlinerRow
			for _, row := range outliner.rows[1:] {
				if !row.IsPinnedSection {
					break
				}
				switch {
				case row.IsProjectHeader:
					projectHeaders = append(projectHeaders, row)
				case row.isTaskRow() && row.task != nil:
					pinnedTasks = append(pinnedTasks, row)
				}
			}
			if len(projectHeaders) != 2 || len(pinnedTasks) != 3 {
				t.Fatalf("Pinned groups/tasks = %d/%d, want 2/3", len(projectHeaders), len(pinnedTasks))
			}
			if got := []string{projectHeaders[0].ProjectID, projectHeaders[1].ProjectID}; !slices.Equal(got, []string{"alpha", "beta"}) {
				t.Fatalf("Pinned project groups = %v, want alphabetical order", got)
			}
			if got := []int{projectHeaders[0].SectionCount, projectHeaders[1].SectionCount}; !slices.Equal(got, []int{2, 1}) {
				t.Fatalf("Pinned project counts = %v, want [2 1]", got)
			}
			if got := []string{
				pinnedTasks[0].ProjectID + "/" + pinnedTasks[0].task.ID,
				pinnedTasks[1].ProjectID + "/" + pinnedTasks[1].task.ID,
				pinnedTasks[2].ProjectID + "/" + pinnedTasks[2].task.ID,
			}; !slices.Equal(got, []string{"alpha/shared", "alpha/alpha-older", "beta/shared"}) {
				t.Fatalf("Pinned task rows = %v, want grouped newest-first qualified rows", got)
			}
			if pinnedTasks[0].Breadcrumb != "parent ›" || pinnedTasks[0].ProjectID != alpha.ID {
				t.Fatalf("promoted child context = project %q breadcrumb %q", pinnedTasks[0].ProjectID, pinnedTasks[0].Breadcrumb)
			}
			if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: child.ID}) != 1 ||
				countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: sameID.ID}) != 1 {
				t.Fatalf("mode %s duplicated a promoted pinned identity", mode)
			}
		})
	}
}

// TestPinnedSectionIsAbsentFromNarrowModes proves Inbox, Next, Backlog, Due, and
// Recent keep their own answer whole: no Pinned header, and a pinned task still
// appears exactly once wherever the mode itself places it.
func TestPinnedSectionIsAbsentFromNarrowModes(t *testing.T) {
	now := time.Now()
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Pinned, pinned.PinnedAt, pinned.DueDate = true, &now, &now
	pinnedInbox := primaryRowsTask("pinned-inbox", models.StateInbox)
	pinnedInbox.Pinned, pinnedInbox.PinnedAt = true, &now
	plain := primaryRowsTask("plain", models.StateBacklog)
	project := primaryRowsProject("alpha", pinned, pinnedInbox, plain)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: pinned.ID}})

	for _, testCase := range []struct {
		mode    chrome.OutlinerModeID
		counted *models.Task
	}{
		{chrome.ModeInbox, pinnedInbox},
		{chrome.ModeNext, pinned},
		{chrome.ModeBacklog, pinned},
		{chrome.ModeDue, pinned},
		{chrome.ModeRecent, pinned},
	} {
		t.Run(string(testCase.mode), func(t *testing.T) {
			outliner.selectOutlinerMode(testCase.mode)
			for _, row := range outliner.rows {
				if row.IsPinnedSection {
					t.Fatalf("%s rendered a Pinned row: %+v", testCase.mode, row)
				}
			}
			if got := countTaskRowsForIdentity(outliner,
				taskref.TaskIdentity{ProjectID: project.ID, TaskID: testCase.counted.ID}); got != 1 {
				t.Fatalf("%s rendered the pinned task %d times, want 1", testCase.mode, got)
			}
		})
	}
}

func TestPinnedProjectHeadersCollapseIndependently(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("alpha-task", models.StateBacklog)
	alphaTask.Pinned, alphaTask.PinnedAt = true, &now
	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Pinned, betaTask.PinnedAt = true, &now
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(chrome.ModeAll)

	alphaHeader := -1
	for index, row := range outliner.rows {
		if row.IsProjectHeader && row.IsPinnedSection && row.ProjectID == alpha.ID {
			alphaHeader = index
			break
		}
	}
	if alphaHeader < 0 {
		t.Fatalf("Pinned project header is missing from rows: %+v", outliner.rows)
	}
	outliner.Cursor = alphaHeader
	collapseKey := outliner.rows[alphaHeader].effectiveCollapseKey()
	if !outliner.collapseOrMoveToParent() {
		t.Fatal("left on Pinned project header did not collapse the group")
	}
	if !outliner.folds.collapsedSections[collapseKey] {
		t.Fatalf("Pinned group collapse key %q was not recorded", collapseKey)
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 0 {
		t.Fatal("collapsing Alpha retained its pinned task")
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 1 {
		t.Fatal("collapsing Alpha hid Beta's pinned task")
	}

	if !outliner.expandAtCursor() {
		t.Fatal("right on Pinned project header did not expand the group")
	}
	if outliner.folds.collapsedSections[collapseKey] {
		t.Fatalf("Pinned group collapse key %q remained set after expansion", collapseKey)
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 1 {
		t.Fatal("expanding Alpha did not restore its pinned task")
	}
}

func TestPinnedSectionRanksSubtreesByRootPinOrder(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	parentTime := now.Add(-time.Hour)
	otherTime := now.Add(-30 * time.Minute)
	childTime := now.Add(-time.Minute)
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateInbox)
	child.ParentID = &parent.ID
	other := primaryRowsTask("other", models.StateBacklog)
	parent.Pinned, parent.PinnedAt = true, &parentTime
	child.Pinned, child.PinnedAt = true, &childTime
	other.Pinned, other.PinnedAt = true, &otherTime
	project := primaryRowsProject("alpha", parent, child, other)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(chrome.ModeAll)

	// The child holds the newest pin, but a subtree ranks by its own root so the
	// order J/K manipulates is the order the rows display in.
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{other.ID, parent.ID, child.ID}) {
		t.Fatalf("Pinned rows = %v, want subtrees ranked by their root's pin", got)
	}
	parentRow, parentFound := findPlanningTaskRow(outliner, parent.ID)
	childRow, childFound := findPlanningTaskRow(outliner, child.ID)
	if !parentFound || !childFound || !parentRow.IsPinnedSection || !childRow.IsPinnedSection ||
		!parentRow.hasVisibleChildren || childRow.Breadcrumb != "" || len(childRow.prefix) <= len(parentRow.prefix) {
		t.Fatalf("Pinned hierarchy = parent %+v found=%t child %+v found=%t",
			parentRow, parentFound, childRow, childFound)
	}

	outliner.folds.expandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: parent.ID}] = false
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{other.ID, parent.ID}) {
		t.Fatalf("collapsed Pinned rows = %v, want nested child hidden", got)
	}
	if outliner.rows[0].SectionCount != 3 {
		t.Fatalf("collapsed Pinned section count = %d, want all 3 members", outliner.rows[0].SectionCount)
	}
}

func TestPinnedSectionOmitsProjectHeaderInFocusedProjectScope(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("alpha-task", models.StateBacklog)
	alphaTask.Pinned, alphaTask.PinnedAt = true, &now
	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Pinned, betaTask.PinnedAt = true, &now
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.pushProjectScope(alpha.ID)

	for _, row := range outliner.rows {
		if row.IsPinnedSection && row.IsProjectHeader {
			t.Fatalf("focused Pinned section rendered redundant project header: %+v", row)
		}
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 1 {
		t.Fatal("focused Pinned section omitted Alpha's task")
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 0 {
		t.Fatal("focused Pinned section retained an out-of-scope task")
	}
}

func TestPinnedAndDueTiesUsePinnedProjectThenAlphabeticalOrder(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaA := primaryRowsTask("alpha-a", models.StateBacklog)
	alphaB := primaryRowsTask("alpha-b", models.StateBacklog)
	betaA := primaryRowsTask("beta-a", models.StateBacklog)
	for _, task := range []*models.Task{alphaA, alphaB, betaA} {
		task.Pinned, task.PinnedAt, task.DueDate = true, &now, &now
	}
	alpha := primaryRowsProject("alpha", alphaA, alphaB)
	beta := primaryRowsProject("beta", betaA)
	beta.Pinned = true
	runtime := testConfigWithProjects(alpha, beta)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.population.projects = []*models.Project{alpha, beta}

	outliner.selectOutlinerMode(chrome.ModeAll)
	if got := visibleQualifiedTaskIDs(outliner); !slices.Equal(got, []string{
		"beta/beta-a",
		"alpha/alpha-a",
		"alpha/alpha-b",
	}) {
		t.Fatalf("Pinned tie order = %v, want pinned projects first then alphabetical", got)
	}

	for _, task := range []*models.Task{alphaA, alphaB, betaA} {
		task.Pinned, task.PinnedAt = false, nil
	}
	outliner.selectOutlinerMode(chrome.ModeDue)
	if got := visibleQualifiedTaskIDs(outliner); !slices.Equal(got, []string{
		"beta/beta-a",
		"alpha/alpha-a",
		"alpha/alpha-b",
	}) {
		t.Fatalf("Due tie order = %v, want pinned projects first then alphabetical", got)
	}
}

func TestPinnedPromotionRebuildKeepsCrossProjectCursorIdentity(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("shared", models.StateBacklog)
	betaTask := primaryRowsTask("shared", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(chrome.ModeAll)
	if !outliner.moveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	betaTask.Pinned, betaTask.PinnedAt = true, &now
	rebuildOutlinerPreservingCursor(outliner)

	row := outliner.rows[outliner.Cursor]
	if !row.IsPinnedSection || row.ProjectID != beta.ID || row.task == nil || row.task.ID != betaTask.ID {
		t.Fatalf("cursor after promotion = %+v, want beta/shared in Pinned", row)
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 1 {
		t.Fatal("promoted cursor task was not deduplicated")
	}
}

func TestFilterRebuildKeepsQualifiedCursorIdentity(t *testing.T) {
	alphaTask := primaryRowsTask("shared", models.StateBacklog)
	alphaTask.Priority = models.PriorityLow
	betaTask := primaryRowsTask("shared", models.StateBacklog)
	betaTask.Priority = 1
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(chrome.ModeBacklog)
	if !outliner.moveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	appModel := &AppModel{crossOutliner: outliner}
	result := appModel.executeFilterOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterPriority},
		Value:      1,
	})
	if result.Err != nil {
		t.Fatalf("apply priority filter: %v", result.Err)
	}
	task, project := outliner.GetTaskRef()
	if task == nil || project == nil || task.ID != betaTask.ID || project.ID != beta.ID {
		t.Fatalf("cursor after filter = %v/%v, want beta/shared", project, task)
	}
	if got := visibleQualifiedTaskIDs(outliner); !slices.Equal(got, []string{"beta/shared"}) {
		t.Fatalf("filtered rows = %v, want beta/shared", got)
	}
}

func TestPinnedSectionEndsWithSeparatorInEveryPromotingMode(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Pinned, pinned.PinnedAt = true, &now
	plain := primaryRowsTask("plain", models.StateBacklog)
	project := primaryRowsProject("alpha", pinned, plain)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: plain.ID}})

	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeToday} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.selectOutlinerMode(mode)
			boundary := slices.IndexFunc(outliner.rows, func(row outlinerRow) bool { return row.IsSeparator })
			if boundary <= 0 || boundary >= len(outliner.rows)-1 {
				t.Fatalf("rows = %+v, want a separator between Pinned and the mode rows", outliner.rows)
			}
			if !outliner.rows[boundary-1].IsPinnedSection || outliner.rows[boundary+1].IsPinnedSection {
				t.Fatalf("separator at %d does not close Pinned: %+v", boundary, outliner.rows)
			}
			if outliner.rows[outliner.Cursor].IsSeparator {
				t.Fatal("cursor settled on the separator")
			}
		})
	}
}

func countTaskRowsForIdentity(outliner *OutlinerModel, identity taskref.TaskIdentity) int {
	count := 0
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && outliner.taskIdentityForRow(row) == identity {
			count++
		}
	}
	return count
}

func visibleQualifiedTaskIDs(outliner *OutlinerModel) []string {
	var ids []string
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil {
			ids = append(ids, row.ProjectID+"/"+row.task.ID)
		}
	}
	return ids
}

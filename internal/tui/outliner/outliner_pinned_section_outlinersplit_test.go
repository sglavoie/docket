package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
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
	outliner.Population.Projects = []*models.Project{beta, alpha}

	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeToday} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.SelectOutlinerMode(mode)
			if len(outliner.Rows) < 3 || !outliner.Rows[0].IsSectionHeader ||
				!outliner.Rows[0].IsPinnedSection || outliner.Rows[0].SectionName != pinnedSectionName {
				t.Fatalf("first row = %+v, want Pinned section header", outliner.Rows[0])
			}
			if outliner.Rows[0].SectionCount != 3 {
				t.Fatalf("Pinned count = %d, want 3", outliner.Rows[0].SectionCount)
			}

			// The lane's groups are read off the rows: an expanded group draws no
			// heading, so the project each row carries is the whole of what says
			// where one group ends and the next begins.
			var projectGroups []OutlinerRowGroup
			var pinnedTasks []OutlinerRow
			counts := make(map[string]int)
			for _, row := range outliner.Rows[1:] {
				if !row.IsPinnedSection {
					break
				}
				if row.Group.ProjectGrouped() && !slices.Contains(projectGroups, row.Group) {
					projectGroups = append(projectGroups, row.Group)
				}
				if row.IsTaskRow() && row.Task != nil {
					pinnedTasks = append(pinnedTasks, row)
					counts[row.Group.ProjectID]++
				}
			}
			if len(projectGroups) != 2 || len(pinnedTasks) != 3 {
				t.Fatalf("Pinned groups/tasks = %d/%d, want 2/3", len(projectGroups), len(pinnedTasks))
			}
			if got := []string{projectGroups[0].ProjectID, projectGroups[1].ProjectID}; !slices.Equal(got, []string{"alpha", "beta"}) {
				t.Fatalf("Pinned project groups = %v, want alphabetical order", got)
			}
			if counts["alpha"] != 2 || counts["beta"] != 1 {
				t.Fatalf("Pinned project counts = %v, want alpha=2 and beta=1", counts)
			}
			if got := []string{
				pinnedTasks[0].ProjectID + "/" + pinnedTasks[0].Task.ID,
				pinnedTasks[1].ProjectID + "/" + pinnedTasks[1].Task.ID,
				pinnedTasks[2].ProjectID + "/" + pinnedTasks[2].Task.ID,
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
			outliner.SelectOutlinerMode(testCase.mode)
			for _, row := range outliner.Rows {
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

// TestPinnedProjectGroupsCollapseIndependently folds one project's Pinned group
// from a task row inside it, because that group draws no heading while it is
// open: the summary the fold produces is where l answers from.
func TestPinnedProjectGroupsCollapseIndependently(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("alpha-task", models.StateBacklog)
	alphaTask.Pinned, alphaTask.PinnedAt = true, &now
	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Pinned, betaTask.PinnedAt = true, &now
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	if !outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) {
		t.Fatalf("alpha's pinned task has no row to stand on: %+v", outliner.Rows)
	}
	collapseKey := outliner.Rows[outliner.Cursor].Group.ProjectKey
	if collapseKey == "" {
		t.Fatalf("alpha's pinned row carries no project fold: %+v", outliner.Rows[outliner.Cursor])
	}
	if !outliner.CollapseOrMoveToParent() {
		t.Fatal("left on a Pinned task row did not collapse its group")
	}
	if !outliner.Folds.CollapsedSections[collapseKey] {
		t.Fatalf("Pinned group collapse key %q was not recorded", collapseKey)
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 0 {
		t.Fatal("collapsing Alpha retained its pinned task")
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 1 {
		t.Fatal("collapsing Alpha hid Beta's pinned task")
	}

	if row := outliner.Rows[outliner.Cursor]; !row.IsProjectHeader || row.Group.ProjectKey != collapseKey {
		t.Fatalf("cursor after the fold = %+v, want alpha's Pinned summary", row)
	}
	if !outliner.ExpandAtCursor() {
		t.Fatal("right on the Pinned summary did not expand the group")
	}
	if outliner.Folds.CollapsedSections[collapseKey] {
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
	outliner.SelectOutlinerMode(chrome.ModeAll)

	// The child holds the newest pin, but a subtree ranks by its own root so the
	// order J/K manipulates is the order the rows display in.
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{other.ID, parent.ID, child.ID}) {
		t.Fatalf("Pinned rows = %v, want subtrees ranked by their root's pin", got)
	}
	parentRow, parentFound := findPlanningTaskRow(outliner, parent.ID)
	childRow, childFound := findPlanningTaskRow(outliner, child.ID)
	if !parentFound || !childFound || !parentRow.IsPinnedSection || !childRow.IsPinnedSection ||
		!parentRow.HasVisibleChildren || childRow.Breadcrumb != "" || len(childRow.Prefix) <= len(parentRow.Prefix) {
		t.Fatalf("Pinned hierarchy = parent %+v found=%t child %+v found=%t",
			parentRow, parentFound, childRow, childFound)
	}

	outliner.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: parent.ID}] = false
	outliner.RebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{other.ID, parent.ID}) {
		t.Fatalf("collapsed Pinned rows = %v, want nested child hidden", got)
	}
	if outliner.Rows[0].SectionCount != 3 {
		t.Fatalf("collapsed Pinned section count = %d, want all 3 members", outliner.Rows[0].SectionCount)
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
	outliner.PushProjectScope(alpha.ID)

	for _, row := range outliner.Rows {
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
	outliner.Population.Projects = []*models.Project{alpha, beta}

	outliner.SelectOutlinerMode(chrome.ModeAll)
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
	outliner.SelectOutlinerMode(chrome.ModeDue)
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
	outliner.SelectOutlinerMode(chrome.ModeAll)
	if !outliner.MoveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	betaTask.Pinned, betaTask.PinnedAt = true, &now
	RebuildOutlinerPreservingCursor(outliner)

	row := outliner.Rows[outliner.Cursor]
	if !row.IsPinnedSection || row.ProjectID != beta.ID || row.Task == nil || row.Task.ID != betaTask.ID {
		t.Fatalf("cursor after promotion = %+v, want beta/shared in Pinned", row)
	}
	if countTaskRowsForIdentity(outliner, taskref.TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 1 {
		t.Fatal("promoted cursor task was not deduplicated")
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
			outliner.SelectOutlinerMode(mode)
			boundary := slices.IndexFunc(outliner.Rows, func(row OutlinerRow) bool { return row.IsSeparator })
			if boundary <= 0 || boundary >= len(outliner.Rows)-1 {
				t.Fatalf("rows = %+v, want a separator between Pinned and the mode rows", outliner.Rows)
			}
			if !outliner.Rows[boundary-1].IsPinnedSection || outliner.Rows[boundary+1].IsPinnedSection {
				t.Fatalf("separator at %d does not close Pinned: %+v", boundary, outliner.Rows)
			}
			if outliner.Rows[outliner.Cursor].IsSeparator {
				t.Fatal("cursor settled on the separator")
			}
		})
	}
}

func countTaskRowsForIdentity(outliner *OutlinerModel, identity taskref.TaskIdentity) int {
	count := 0
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && outliner.TaskIdentityForRow(row) == identity {
			count++
		}
	}
	return count
}

package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestPinnedSectionAppearsAboveEveryModeAndDeduplicatesQualifiedTasks(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	older := now.Add(-time.Hour)
	newer := now.Add(-time.Minute)

	parent := primaryRowsTask("parent", models.StateReady)
	child := primaryRowsTask("shared", models.StateReady)
	child.ParentID = &parent.ID
	child.Pinned, child.PinnedAt = true, &newer
	alphaOlder := primaryRowsTask("alpha-older", models.StateReady)
	alphaOlder.Pinned, alphaOlder.PinnedAt = true, &older
	due := primaryRowsTask("due", models.StateReady)
	due.DueDate = &now
	alpha := primaryRowsProject("alpha", parent, child, alphaOlder, due)
	alpha.Name, alpha.Slug = "Alpha", "alpha"

	sameID := primaryRowsTask("shared", models.StateInbox)
	sameID.Pinned, sameID.PinnedAt = true, &older
	next := primaryRowsTask("next", models.StateNext)
	beta := primaryRowsProject("beta", sameID, next)
	beta.Name, beta.Slug = "Beta", "beta"

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.loadedProjects = []*models.Project{beta, alpha} // catalog order resolves equal pin times.

	for _, mode := range []OutlinerModeID{ModeAll, ModeInbox, ModeNext, ModeBacklog, ModeDue} {
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
				t.Fatalf("Pinned project groups = %v, want newest-pin-first group order", got)
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
			if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: alpha.ID, TaskID: child.ID}) != 1 ||
				countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: beta.ID, TaskID: sameID.ID}) != 1 {
				t.Fatalf("mode %s duplicated a promoted pinned identity", mode)
			}
		})
	}
}

func TestPinnedProjectHeadersCollapseIndependently(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("alpha-task", models.StateReady)
	alphaTask.Pinned, alphaTask.PinnedAt = true, &now
	betaTask := primaryRowsTask("beta-task", models.StateReady)
	betaTask.Pinned, betaTask.PinnedAt = true, &now
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeAll)

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
	outliner.cursor = alphaHeader
	collapseKey := outliner.rows[alphaHeader].effectiveCollapseKey()
	if !outliner.collapseOrMoveToParent() {
		t.Fatal("left on Pinned project header did not collapse the group")
	}
	if !outliner.collapsedSections[collapseKey] {
		t.Fatalf("Pinned group collapse key %q was not recorded", collapseKey)
	}
	if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 0 {
		t.Fatal("collapsing Alpha retained its pinned task")
	}
	if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 1 {
		t.Fatal("collapsing Alpha hid Beta's pinned task")
	}

	if !outliner.expandAtCursor() {
		t.Fatal("right on Pinned project header did not expand the group")
	}
	if outliner.collapsedSections[collapseKey] {
		t.Fatalf("Pinned group collapse key %q remained set after expansion", collapseKey)
	}
	if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 1 {
		t.Fatal("expanding Alpha did not restore its pinned task")
	}
}

func TestPinnedSectionOmitsProjectHeaderInFocusedProjectScope(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("alpha-task", models.StateReady)
	alphaTask.Pinned, alphaTask.PinnedAt = true, &now
	betaTask := primaryRowsTask("beta-task", models.StateReady)
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
	if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: alpha.ID, TaskID: alphaTask.ID}) != 1 {
		t.Fatal("focused Pinned section omitted Alpha's task")
	}
	if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 0 {
		t.Fatal("focused Pinned section retained an out-of-scope task")
	}
}

func TestPinnedAndDueTiesUseCatalogThenProjectDocumentOrder(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaA := primaryRowsTask("alpha-a", models.StateReady)
	alphaB := primaryRowsTask("alpha-b", models.StateReady)
	betaA := primaryRowsTask("beta-a", models.StateReady)
	for _, task := range []*models.Task{alphaA, alphaB, betaA} {
		task.Pinned, task.PinnedAt, task.DueDate = true, &now, &now
	}
	alpha := primaryRowsProject("alpha", alphaA, alphaB)
	beta := primaryRowsProject("beta", betaA)
	runtime := testConfigWithProjects(beta, alpha)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.loadedProjects = []*models.Project{alpha, beta}

	outliner.selectOutlinerMode(ModeDue)
	if got := visibleQualifiedTaskIDs(outliner); !slices.Equal(got, []string{
		"beta/beta-a",
		"alpha/alpha-a",
		"alpha/alpha-b",
	}) {
		t.Fatalf("Pinned tie order = %v, want catalog then document order", got)
	}

	for _, task := range []*models.Task{alphaA, alphaB, betaA} {
		task.Pinned, task.PinnedAt = false, nil
	}
	rebuildOutlinerPreservingCursor(outliner)
	if got := visibleQualifiedTaskIDs(outliner); !slices.Equal(got, []string{
		"beta/beta-a",
		"alpha/alpha-a",
		"alpha/alpha-b",
	}) {
		t.Fatalf("Due tie order = %v, want catalog then document order", got)
	}
}

func TestPinnedPromotionRebuildKeepsCrossProjectCursorIdentity(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	alphaTask := primaryRowsTask("shared", models.StateReady)
	betaTask := primaryRowsTask("shared", models.StateReady)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeBacklog)
	if !outliner.moveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	betaTask.Pinned, betaTask.PinnedAt = true, &now
	rebuildOutlinerPreservingCursor(outliner)

	row := outliner.rows[outliner.cursor]
	if !row.IsPinnedSection || row.ProjectID != beta.ID || row.task == nil || row.task.ID != betaTask.ID {
		t.Fatalf("cursor after promotion = %+v, want beta/shared in Pinned", row)
	}
	if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: beta.ID, TaskID: betaTask.ID}) != 1 {
		t.Fatal("promoted cursor task was not deduplicated")
	}
}

func TestFilterRebuildKeepsQualifiedCursorIdentity(t *testing.T) {
	alphaTask := primaryRowsTask("shared", models.StateReady)
	alphaTask.Priority = 5
	betaTask := primaryRowsTask("shared", models.StateReady)
	betaTask.Priority = 1
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeBacklog)
	if !outliner.moveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	appModel := &AppModel{crossOutliner: outliner}
	result := appModel.executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterPriority},
		Value:      1,
	})
	if result.Err != nil {
		t.Fatalf("apply priority filter: %v", result.Err)
	}
	task, project := outliner.getTaskRef()
	if task == nil || project == nil || task.ID != betaTask.ID || project.ID != beta.ID {
		t.Fatalf("cursor after filter = %v/%v, want beta/shared", project, task)
	}
	if got := visibleQualifiedTaskIDs(outliner); !slices.Equal(got, []string{"beta/shared"}) {
		t.Fatalf("filtered rows = %v, want beta/shared", got)
	}
}

func countTaskRowsForIdentity(outliner *OutlinerModel, identity TaskIdentity) int {
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

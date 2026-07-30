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
	due := primaryRowsTask("due", models.StateReady)
	due.DueDate = &now
	alpha := primaryRowsProject("alpha", parent, child, due)
	alpha.Name, alpha.Slug = "Alpha", "alpha"

	sameID := primaryRowsTask("shared", models.StateInbox)
	sameID.Pinned, sameID.PinnedAt = true, &older
	next := primaryRowsTask("next", models.StateNext)
	beta := primaryRowsProject("beta", sameID, next)
	beta.Name, beta.Slug = "Beta", "beta"

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.loadedProjects = []*models.Project{beta, alpha} // catalog order must win.

	for _, mode := range []OutlinerModeID{ModeAll, ModeInbox, ModeNext, ModeBacklog, ModeDue} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.selectOutlinerMode(mode)
			if len(outliner.rows) < 3 || !outliner.rows[0].IsSectionHeader ||
				!outliner.rows[0].IsPinnedSection || outliner.rows[0].SectionName != pinnedSectionName {
				t.Fatalf("first row = %+v, want Pinned section header", outliner.rows[0])
			}
			if outliner.rows[0].SectionCount != 2 {
				t.Fatalf("Pinned count = %d, want 2", outliner.rows[0].SectionCount)
			}

			first, second := outliner.rows[1], outliner.rows[2]
			if !first.IsPinnedSection || !second.IsPinnedSection {
				t.Fatalf("Pinned task flags = %v/%v, want true/true", first.IsPinnedSection, second.IsPinnedSection)
			}
			if got := []string{first.ProjectID + "/" + first.task.ID, second.ProjectID + "/" + second.task.ID}; !slices.Equal(got, []string{"alpha/shared", "beta/shared"}) {
				t.Fatalf("Pinned order = %v, want newest-pin-first qualified rows", got)
			}
			if first.Breadcrumb != "parent ›" || first.ProjectID != alpha.ID {
				t.Fatalf("promoted child context = project %q breadcrumb %q", first.ProjectID, first.Breadcrumb)
			}
			if countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: alpha.ID, TaskID: child.ID}) != 1 ||
				countTaskRowsForIdentity(outliner, TaskIdentity{ProjectID: beta.ID, TaskID: sameID.ID}) != 1 {
				t.Fatalf("mode %s duplicated a promoted pinned identity", mode)
			}
		})
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

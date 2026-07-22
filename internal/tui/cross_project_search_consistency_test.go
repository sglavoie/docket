package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSearchTagOnlyMatchConsistency(t *testing.T) {
	taggedTask := makeTask("tag-task-1", "Unrelated title", "")
	taggedTask.Tags = []string{"backend", "api"}

	untaggedTask := makeTask("tag-task-2", "Another unrelated title", "")

	proj := &models.Project{
		ID:    "proj-tag-test",
		Name:  "TagTest",
		Tasks: []*models.Task{taggedTask, untaggedTask},
	}
	proj.BuildTaskTree()

	const query = "backend"

	singleM := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll,
		project:           proj,
	}
	singleM.SetSearchQuery(query)
	singleM.rebuildRows()

	singleTaskTitles := make(map[string]bool)
	for _, row := range singleM.rows {
		if row.isTaskRow() && row.task != nil {
			singleTaskTitles[row.task.Title] = true
		}
	}

	if !singleTaskTitles[taggedTask.Title] {
		t.Error("single-project outliner: tag-matched task not found in rows (tag-only search returned no match)")
	}
	if singleTaskTitles[untaggedTask.Title] {
		t.Error("single-project outliner: untagged task should not appear when query matches neither its title nor tags")
	}

	crossM := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[TaskIdentity]bool),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
	}
	crossM.SetSearchQuery(query)
	crossM.crossProjectRebuildRows()

	crossTaskTitles := make(map[string]bool)
	for _, row := range crossM.rows {
		if row.isTaskRow() && row.task != nil {
			crossTaskTitles[row.task.Title] = true
		}
	}

	if !crossTaskTitles[taggedTask.Title] {
		t.Error("cross-project outliner: tag-matched task not found in rows (tag-only search returned no match)")
	}
	if crossTaskTitles[untaggedTask.Title] {
		t.Error("cross-project outliner: untagged task should not appear when query matches neither its title nor tags")
	}
}

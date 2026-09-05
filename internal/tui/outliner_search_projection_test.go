package tui

import (
	"maps"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestScopeWideSearchRestoresProjectionExactlyAndFallsBackAfterDeletion(t *testing.T) {
	first := primaryRowsTask("first", models.StateInbox)
	first.Title = "Needle first"
	second := primaryRowsTask("second", models.StateInbox)
	second.Title = "Needle second"
	third := primaryRowsTask("third", models.StateInbox)
	third.Title = "Needle third"
	first.Tags = []string{"saved"}
	second.Tags = []string{"saved"}
	third.Tags = []string{"saved"}
	project := primaryRowsProject("alpha", first, second, third)
	outlinerView := scopeTestOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	outlinerView.Filter.Tags = map[string]bool{"saved": true}
	outlinerView.AdoptFilterCarriers()
	outlinerView.Folds.CollapsedSections = map[string]bool{"saved-section": true}
	outlinerView.Folds.ExpandedNodes = map[taskref.TaskIdentity]bool{{ProjectID: project.ID, TaskID: first.ID}: false}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	if !outlinerView.MoveCursorToProjectTask(second.ID, project.ID) {
		t.Fatal("failed to seed cursor")
	}
	outlinerView.ViewScrollOffset = 4
	wantFilter := outliner.CopyFilterState(outlinerView.Filter)
	wantCollapsed := maps.Clone(outlinerView.Folds.CollapsedSections)
	wantExpanded := maps.Clone(outlinerView.Folds.ExpandedNodes)

	searchRankedList(outlinerView, "needle")
	filterResult := applyOutlinerFilterOperation(outlinerView, operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterPriority},
		Value:      models.PriorityHigh,
	})
	// The facet applies to the live projection rather than refusing, and Esc
	// still restores the snapshot taken at entry — including this edit.
	if filterResult.Status != "Filtering High priority" ||
		outlinerView.Filter.Priority != models.PriorityHigh {
		t.Fatalf("filter refused during search: result=%+v priority=%d",
			filterResult, outlinerView.Filter.Priority)
	}
	outlinerView.Filter.Tags = map[string]bool{"changed": true}
	outlinerView.AdoptFilterCarriers()
	outlinerView.Folds.CollapsedSections["changed-section"] = true
	outlinerView.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: third.ID}] = false
	// The mode is a live axis under a held query, and it is also part of the
	// frame the query was typed in, so moving it here is exactly what Esc has to
	// put back below.
	if _, _ = outlinerView.SelectOutlinerMode(chrome.ModeDue); outlinerView.PrimaryMode != chrome.ModeDue {
		t.Fatalf("mode refused to change during search, staying %s", outlinerView.PrimaryMode)
	}
	outlinerView.SetSearchActive(true)
	outlinerView.SetSearchInputValue("needle")
	_, _ = outlinerView.HandleKeyMsg(syntheticSpecialKey(tea.KeyEsc))

	if outlinerView.PrimaryMode != chrome.ModeInbox ||
		outlinerView.Filter.Priority != wantFilter.Priority ||
		!maps.Equal(outlinerView.Filter.Tags, wantFilter.Tags) ||
		!maps.Equal(outlinerView.Folds.CollapsedSections, wantCollapsed) ||
		!maps.Equal(outlinerView.Folds.ExpandedNodes, wantExpanded) ||
		outlinerView.ViewScrollOffset != 4 {
		t.Fatalf("restored state differs: mode=%s filter=%v collapsed=%v expanded=%v scroll=%d",
			outlinerView.PrimaryMode, outlinerView.Filter.Tags, outlinerView.Folds.CollapsedSections,
			outlinerView.Folds.ExpandedNodes, outlinerView.ViewScrollOffset)
	}
	if outlinerView.HasSearchProjection() || outlinerView.HasActiveSearch() || outlinerView.SearchQuery() != "" {
		t.Fatalf("Esc left search active: projection=%v input=%v query=%q",
			outlinerView.HasSearchProjection(), outlinerView.HasActiveSearch(), outlinerView.SearchQuery())
	}
	if task, selectedProject := outlinerView.GetTaskRef(); task == nil || task.ID != second.ID || selectedProject != project {
		t.Fatalf("restored cursor = %v/%v, want alpha/second", selectedProject, task)
	}

	searchRankedList(outlinerView, "needle")
	project.Tasks = []*models.Task{first, third}
	project.BuildTaskTree()
	outlinerView.ClearSearch()
	if task, _ := outlinerView.GetTaskRef(); task == nil || task.ID != third.ID {
		t.Fatalf("deleted cursor fallback = %v, want nearest following task %s", task, third.ID)
	}
}

func qualifiedVisibleTaskIDs(outlinerView *outliner.OutlinerModel) []taskref.TaskIdentity {
	result := make([]taskref.TaskIdentity, 0)
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil {
			result = append(result, taskref.TaskIdentity{ProjectID: row.ProjectID, TaskID: row.Task.ID})
		}
	}
	return result
}

package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func TestFilterRebuildKeepsQualifiedCursorIdentity(t *testing.T) {
	alphaTask := primaryRowsTask("shared", models.StateBacklog)
	alphaTask.Priority = models.PriorityLow
	betaTask := primaryRowsTask("shared", models.StateBacklog)
	betaTask.Priority = 1
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outlinerView, _ := primaryRowsOutliner(alpha, beta)
	outlinerView.SelectOutlinerMode(chrome.ModeBacklog)
	if !outlinerView.MoveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	appModel := &AppModel{crossOutliner: outlinerView}
	result := appModel.executeFilterOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterPriority},
		Value:      1,
	})
	if result.Err != nil {
		t.Fatalf("apply priority filter: %v", result.Err)
	}
	task, project := outlinerView.GetTaskRef()
	if task == nil || project == nil || task.ID != betaTask.ID || project.ID != beta.ID {
		t.Fatalf("cursor after filter = %v/%v, want beta/shared", project, task)
	}
	if got := visibleQualifiedTaskIDs(outlinerView); !slices.Equal(got, []string{"beta/shared"}) {
		t.Fatalf("filtered rows = %v, want beta/shared", got)
	}
}

func visibleQualifiedTaskIDs(outlinerView *outliner.OutlinerModel) []string {
	var ids []string
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids = append(ids, row.ProjectID+"/"+row.Task.ID)
		}
	}
	return ids
}

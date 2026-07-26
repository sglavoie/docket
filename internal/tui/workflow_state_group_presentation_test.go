package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestWorkflowStateGroupThemeSemanticsCoverEveryGroup(t *testing.T) {
	for _, group := range models.AllWorkflowStateGroups() {
		if accent := GetWorkflowStateGroupAccent(group); accent == nil {
			t.Errorf("group %q has no theme accent", group)
		}
		label := group.Label()
		if got := GetWorkflowStateGroupStyle(group).Render(label); got == "" {
			t.Errorf("group %q rendered an empty header", group)
		}
	}
}

func TestLegacyAllPresentationGroupingUsesStateGroups(t *testing.T) {
	ready, _ := models.NewTask("ready", 1, models.TaskTypeTask, "Ready")
	started, _ := models.NewTask("started", 2, models.TaskTypeTask, "Started")
	inbox, _ := models.NewTask("inbox", 3, models.TaskTypeTask, "Inbox")
	started.ApplyWorkflowState(models.StateInProgress)
	inbox.ApplyWorkflowState(models.StateInbox)

	forests := buildSectionTaskForests([]*models.Task{ready, started, inbox})
	if got := forests[models.StateGroupBacklog]; got == nil || got.count != 2 {
		t.Fatalf("legacy Active adapter did not combine canonical Backlog/In Progress: %#v", got)
	}
	if got := forests[models.StateGroupInbox]; got == nil || got.count != 1 {
		t.Fatalf("Inbox state group = %#v, want one task", got)
	}
	if _, exists := forests[models.StateGroupInProgress]; exists {
		t.Fatal("visible adapter leaked a second In Progress header before atomic cutover")
	}
}
